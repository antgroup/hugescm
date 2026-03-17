// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"io"
	"math/bits"
)

const (
	// CDC default parameters optimized for AI model files
	// Following FastCDC paper recommendations: min = target/4, max = target*8
	//
	// Why 4MB for AI models?
	// - Typical tensor sizes: several MB to hundreds of MB
	// - Fine-tuning updates: entire tensors or large regions
	// - 4MB chunks: good balance between dedupe precision and metadata overhead
	//
	// Benefits vs 1MB chunks:
	// - 75% fewer fragments (less metadata, faster negotiation)
	// - Similar dedupe effectiveness for model files
	// - Lower CPU overhead for hash computation
	CDCDefaultTargetSize = 4 << 20  // Target chunk size 4MB (optimal for AI models)
	CDCDefaultMinSize    = 1 << 20  // Minimum chunk size 1MB (target/4)
	CDCDefaultMaxSize    = 32 << 20 // Maximum chunk size 32MB (target*8)
)

// CDCChunker implements FastCDC algorithm
// Reference: "FastCDC: A Fast and Efficient Content-Defined Chunking Approach"
// Paper: https://www.usenix.org/node/196197
//
// Key innovation: Normalized chunking with three masks
// - Skip fast in the beginning (maskS)
// - Normal cutting in the middle (maskN)
// - Allow larger chunks at the end (maskL)
type CDCChunker struct {
	targetSize int64
	minSize    int64
	maxSize    int64
	normalSize int64 // Normalization point
	windowSize int64 // Window size for normal phase

	// Three masks for FastCDC normalized cutting
	maskS uint64 // Small mask: higher cutting probability (skip fast phase)
	maskN uint64 // Normal mask: standard cutting probability (normal phase)
	maskL uint64 // Large mask: lower cutting probability (tail phase)
}

// NewCDCChunker creates a FastCDC chunker
func NewCDCChunker(targetSize int64) *CDCChunker {
	// FastCDC normalization strategy:
	// - min = target / 4
	// - normal = target
	// - max = target * 8
	//
	// This gives better chunk size distribution than basic CDC

	minSize := max(targetSize/4,
		// Minimum 64KB
		64<<10)

	maxSize := min(targetSize*8,
		// Maximum 64MB
		64<<20)

	// Calculate mask bits
	// FastCDC uses: maskBits = log2(target) - normalization_offset
	// Typical offset is 0-2, we use 1 for better average size
	// Ensure reasonable bounds
	// Minimum 1KB chunks
	// Maximum 16MB chunks
	maskBits := min(max(bits.Len64(uint64(targetSize))-1, 10), 24)

	// Three masks with different cutting probabilities
	// maskS: 1/(2^(bits-2)) - skip phase, highest cutting probability
	// maskN: 1/(2^bits) - normal phase, standard cutting probability
	// maskL: 1/(2^(bits+1)) - tail phase, lowest cutting probability
	maskS := uint64(1)<<(maskBits-2) - 1
	maskN := uint64(1)<<maskBits - 1
	maskL := uint64(1)<<(maskBits+1) - 1

	return &CDCChunker{
		targetSize: targetSize,
		minSize:    minSize,
		maxSize:    maxSize,
		normalSize: targetSize,
		windowSize: targetSize,
		maskS:      maskS,
		maskN:      maskN,
		maskL:      maskL,
	}
}

// Chunk performs FastCDC chunking and returns chunk boundaries
// This is a reference implementation for testing
func (c *CDCChunker) Chunk(reader io.Reader, size int64) ([]chunk, error) {
	chunks := make([]chunk, 0)

	buf := make([]byte, 32<<10) // 32KB read buffer
	hash := uint64(0)
	chunkStart := int64(0)
	bytesRead := int64(0)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			for i := range n {
				// Standard Gear hash: hash = (hash << 1) + gearTable[byte]
				hash = (hash << 1) + gearTable[buf[i]]
				bytesRead++

				chunkSize := bytesRead - chunkStart

				// FastCDC normalized cutting strategy (three-phase):
				// Phase 1 (0 ~ min): No cutting
				// Phase 2 (min ~ normal): Use maskS (highest cutting probability)
				// Phase 3 (normal ~ normal+window): Use maskN (standard cutting probability)
				// Phase 4 (normal+window ~ max): Use maskL (lowest cutting probability)
				// Phase 5 (max+): Force cut

				if chunkSize < c.minSize {
					// Too small, continue
					continue
				}

				shouldCut := false

				if chunkSize < c.normalSize {
					// Phase 2: Skip phase - use maskS to skip quickly
					shouldCut = (hash & c.maskS) == 0
				} else if chunkSize < c.normalSize+c.windowSize {
					// Phase 3: Normal phase - use maskN for standard cutting
					shouldCut = (hash & c.maskN) == 0
				} else if chunkSize < c.maxSize {
					// Phase 4: Tail phase - use maskL to allow larger chunks
					shouldCut = (hash & c.maskL) == 0
				} else {
					// Phase 5: Force cut at max size
					shouldCut = true
				}

				if shouldCut {
					chunks = append(chunks, chunk{
						offset: chunkStart,
						size:   chunkSize,
					})
					chunkStart = bytesRead
					hash = 0
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	// Last chunk
	if bytesRead > chunkStart {
		chunks = append(chunks, chunk{
			offset: chunkStart,
			size:   bytesRead - chunkStart,
		})
	}

	return chunks, nil
}

// ChunkStreaming performs FastCDC chunking with rolling buffer
//
// IMPORTANT: This is NOT pure streaming! It uses a rolling buffer to enable
// chunk boundary detection and hash computation in a single pass.
//
// Memory usage: O(maxChunkSize) - the rolling buffer holds up to maxSize bytes
// This is a standard trade-off in CDC implementations (restic, borg, etc.)
//
// The onChunk callback receives a reader over the chunk data. The callback
// should stream the data to hash writer, not materialize it entirely.
func (c *CDCChunker) ChunkStreaming(reader io.Reader, size int64, onChunk func(offset, size int64, chunkReader io.Reader) error) error {
	buf := make([]byte, 32<<10) // 32KB read buffer
	hash := uint64(0)
	chunkStart := int64(0)
	bytesRead := int64(0)

	// Rolling buffer to hold chunk data
	// This is necessary for CDC because we need to:
	// 1. Read bytes to compute rolling hash
	// 2. Detect boundary
	// 3. Then hash the chunk
	//
	// We cannot "unread" bytes from a stream, so we buffer them.
	// This is how restic, borg, and other CDC implementations work.
	//
	// Memory: O(maxChunkSize), typically 32MB for 4MB target size
	chunkBuf := make([]byte, 0, c.maxSize)

	for {
		n, err := reader.Read(buf)
		if n > 0 {
			for i := range n {
				// Standard Gear hash
				hash = (hash << 1) + gearTable[buf[i]]
				bytesRead++

				// Store byte in rolling buffer
				chunkBuf = append(chunkBuf, buf[i])

				chunkSize := bytesRead - chunkStart

				// FastCDC normalized cutting (three-phase)
				if chunkSize < c.minSize {
					continue
				}

				shouldCut := false

				if chunkSize < c.normalSize {
					// Phase 2: Skip phase
					shouldCut = (hash & c.maskS) == 0
				} else if chunkSize < c.normalSize+c.windowSize {
					// Phase 3: Normal phase
					shouldCut = (hash & c.maskN) == 0
				} else if chunkSize < c.maxSize {
					// Phase 4: Tail phase
					shouldCut = (hash & c.maskL) == 0
				} else {
					// Phase 5: Force cut
					shouldCut = true
				}

				if shouldCut {
					// Found boundary, emit chunk
					chunkReader := &sliceReader{data: chunkBuf}
					if err := onChunk(chunkStart, chunkSize, chunkReader); err != nil {
						return err
					}

					chunkStart = bytesRead
					hash = 0
					chunkBuf = chunkBuf[:0]
				}
			}
		}

		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// Last chunk
	if bytesRead > chunkStart {
		chunkReader := &sliceReader{data: chunkBuf}
		return onChunk(chunkStart, bytesRead-chunkStart, chunkReader)
	}

	return nil
}

// sliceReader implements io.Reader for a byte slice
type sliceReader struct {
	data []byte
	pos  int
}

func (r *sliceReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

// gearTable is a precomputed pseudo-random table for Gear rolling hash.
// Generated using LCG (Linear Congruential Generator) to avoid pattern bias.
// This is the same approach used by restic and other production CDC implementations.
var gearTable = [256]uint64{
	0x2CEAEE21BF46BC00, 0xAA80754D1A1A8D4F, 0xB3C4904A6D278932, 0xBC69CF4276846D19,
	0x377B2FD56A5B15B4, 0x64D815DEEAF29DF3, 0xF66E100DB2D7D206, 0x1069E6A57E06665D,
	0x7BE902917B70A2A8, 0x68901BAF16AD70D7, 0xF2CAA7C38002001A, 0x5A55DB213CF06BE1,
	0x4C88C014892416DC, 0xA0C1AC9A9822A9FB, 0x9E5C77E75AE9E76E, 0x37E83672575AC1A5,
	0x960FDE3F09756650, 0xD4076A092B5C2D5F, 0xF4A4CB6FED689C02, 0xF541557FFD59EBA9,
	0x4285A9AF717BC504, 0x8E4D95DD2C3A1F03, 0x7B96DAE083C071D6, 0xD9F321BE6242ADED,
	0x7DF64FCFE859A6F8, 0xF6453AA71756E2E7, 0x92191EFBD0B0FCEA, 0xD9895985202E0C71,
	0x68518D384170C02C, 0xAA7D7E42512B1D0B, 0xCF273511CACB113E, 0x138C146FCBBD4B35,
	0x57100D0A31D604A0, 0x2BB1B0131771B16F, 0xD6E649D7B704C2D2, 0xA7FDD0BB3C1DEE39,
	0xA95D2364F505A854, 0x3976BAD3C0CBC413, 0x5BC5A782B48D65A6, 0x4BCDEBB24B4DB97D,
	0xA6480378D4D71F48, 0x0653E779AEA4B8F7, 0xCBAD05BA5DE18DBA, 0xA433BDE1129EB101,
	0xD1C2886248B11D7C, 0x8F2179789556341B, 0x860B0F2A531F0F0E, 0x4F09BC2DE77B18C5,
	0x871ABA079FFD96F0, 0xB551A6CCFE8C197F, 0x661DC63EC198FDA2, 0x9A7F8D639C6974C9,
	0x6E7133FB6BDDBFA4, 0xDF97F5FD29E88D23, 0xCE07EA313CABAD76, 0xE31B5068CA50890D,
	0x9DA9BF211C1E0B98, 0x92B8B3E9AFE7F307, 0xE399AC387BD0B28A, 0xEF9FB69425FB5991,
	0x1B9ABEC0836A2ECC, 0x1CBB31158B04EF2B, 0x8D8433AED1F2E0DE, 0x613DB78625DD2A55,
	0x7F5A721836C11D40, 0xDEF1E43F6B1C658F, 0x2084FC9872024C72, 0xA69EA4EC7C157F59,
	0x74859B95FC290AF4, 0xB5B4ACFC77117A33, 0x1191B75BD4C84946, 0xFC692CD428B41C9D,
	0x8BEC71E7BCA36BE8, 0xE0F3DEBEE0B19117, 0x77F93FFBD3FB6B5A, 0x9F1254F3283D0621,
	0x48DE1C56AD60F41C, 0x0C9CC190EED84E3B, 0xEF1E9FF44E9386AE, 0xC7A1D8DB026C7FE5,
	0xB8AB9E91A4359790, 0x9BCFFC0F61D3959F, 0xC7BB053F6A5DAF42, 0xFA614C91BD3B0DE9,
	0x3C796F72CB4C8A44, 0xB594301456078B43, 0xF08EAC42C6D03916, 0x436B6CDEF821742D,
	0x8333E5A8281C4038, 0x05BFAB4E08D29327, 0x4457FEB1351EB82A, 0xEAF89AEF339CB6B1,
	0xFDCEC3B5A99A6D6C, 0xDB26BCEB23B1514B, 0x5D63880EC98E007E, 0xB1C1620788F21975,
	0xA68705CAB1B005E0, 0xBEC543D871A2A9AF, 0x13C2D6BA5A082612, 0x5666582056332079,
	0xAE5D669C4DED3D94, 0x5DACBC0222CBC053, 0xFF9D3F68BDF07CE6, 0x348FB59DA2818FBD,
	0x9DF31020937D8888, 0x01B936E5025BF937, 0x09E7A61B633798FA, 0x5238B1403E936B41,
	0xAE9880E6D25B9ABC, 0xD8A3AABA42B0F85B, 0x474FA40D0CAF4E4E, 0x62E55412E576F705,
	0x6018B21B93C56830, 0xAAD525EAC3BAA1BF, 0xBC736B14CD9EB0E2, 0x0393E9C2E196B709,
	0x4F2F0C4E97F024E4, 0x79C3287CF79F1963, 0x3225C5D8969614B6, 0xB74904C3F9FD6F4D,
	0xA87F3D0C46FC44D8, 0xF3327D1AC99EC347, 0x98E73E15E7830DCA, 0xB80ADF13ABDA23D1,
	0xCCADBC8B49297C0C, 0x1C34DD5B2B38436B, 0x800650887B04701E, 0xBF2C90A7F4441895,
	0xAF6EF4523A4ABE80, 0x561AD36D2B8C7DCF, 0x833245CBFEFE4FB2, 0xE1E687D22E3ED199,
	0x8435DD10AC7A4034, 0xAF342458BD029673, 0x1137BA3F2E6E0086, 0x25010E5EC8FE12DD,
	0x0ABD6631EE0D7528, 0xB86E7C038D2BF157, 0x823899ABE07E169A, 0x3E529C3EDA69E061,
	0x42A4942F46C9115C, 0xECFE9D4692E8327B, 0xEE56AF88E0DA65EE, 0x879BB650D1E27E25,
	0xABF0769AA05508D0, 0x0CE266E336C93DDF, 0xAB46574FA5440282, 0xE59911A9CF447029,
	0x4FF69381CDF08F84, 0x1E12204D39B73783, 0x73E112D934654056, 0x313D61D1622C7A6D,
	0x22584E65E7661978, 0x1AA2A948BDD48367, 0x7B7A2C42D1E5B36A, 0x069456F5B57BA0F1,
	0xE281FCD16B3F5AAC, 0xBFFFEB8A15A1C58B, 0x265936BC43BE2FBE, 0x6C98E0766B1B27B5,
	0x4CDB94DB1C394720, 0x445BC8173D61E1EF, 0xB6567B16C4CCC952, 0x19CA4C80AC0092B9,
	0x7EF829DACDF812D4, 0x4F89496122BDFC93, 0x590CB334F8A8D426, 0xDFB69F173071A5FD,
	0xCE29348094FB31C8, 0x994329251EA97977, 0x90290FD974B6E43A, 0x8E58C60544886581,
	0x77D0149E0DD157FC, 0xB6505C654585FC9B, 0xF7ED5802B27CCD8E, 0x9BBFB4240CF71545,
	0x56746F84AF8C7970, 0x0207E268718769FF, 0x58BC0D487F35A422, 0x54410145900C3949,
	0xB82D55235D75CA24, 0x065133F92B57E5A3, 0xC7DE46C83CA5BBF6, 0xC8E0454946F6958D,
	0x04083348AC01BE18, 0x92597844D4FBD387, 0x6B4C595A872EA90A, 0xDB12E0923B492E11,
	0xA99C08DE8D04094C, 0x3BDCCB0ABAF5D7AB, 0xB330314E15233F5E, 0x0902628EF4BF46D5,
	0x9CBA6DD757239FC0, 0xCAC2FE7CEFAAD60F, 0x823754F8E35B92F2, 0xB0FBE47FBB4063D9,
	0x89BC7F1B5C8EB574, 0x7B5A188B1505F2B3, 0xB83B115A0308F7C6, 0x20AFE367F124491D,
	0x5F14A35184EEBE68, 0x33EF7989785C9197, 0xEAF818039CCA01DA, 0x2171D45B89B6FAA1,
	0x7487351C9E9C6E9C, 0xA629E785249256BB, 0xFC2670D5FCFE852E, 0xAFA48A61DFFCBC65,
	0xC501249A5B13BA10, 0x96215057CE7D261F, 0x1F48BFF9BD5B95C2, 0x31D6C13371B61269,
	0x8B1783D82AA7D4C4, 0x4359C2F4BF8923C3, 0x4DB882405FBF8796, 0x0A52B46842A3C0AD,
	0x19B113CD6B7732B8, 0xBFF318B2229CB3A7, 0x5D939CDFEE45EEAA, 0x086C9380EC0ACB31,
	0x767660799F9F87EC, 0x2121DFC0573C79CB, 0x54C4DA9F749B9EFE, 0xA9C520CC9C7875F5,
	0x689BD6489EB1C860, 0xCCB50AD32EEF5A2F, 0xF1F72D3F6692AC92, 0x02BA9FCA8BC644F9,
	0x3CDBB815F6662814, 0xAD62957738E278D3, 0x7782977247F66B66, 0x81F0EF85A75DFC3D,
	0x4FE9AF53EE901B08, 0x9B60F2E77FCD39B7, 0xD43BE757299F6F7A, 0x1D48D2CD7ABD9FC1,
	0x40193E39E452553C, 0x7980A4B65E1540DB, 0xDC441C58CFC78CCE, 0x88E4CB57983B7385,
	0xB1EE01B0F092CAB0, 0xC5C7897E4C32723F, 0xA220C2D9959DD762, 0x698AE2010609FB89,
	0xA1C9FD3D0DAEAF64, 0x03454055CD52F1E3, 0xA3F55D6D621AA336, 0x4F9E55D7737BFBCD,
	0x28C7219C306E7758, 0xC6FEB62BDE3F23C7, 0x71D853D04213844A, 0x876391863A887851,
	0x188396840839D68C, 0x0D108CC08A7DABEB, 0xF83DDC897B8F4E9E, 0x131E537B718EB515,
}
