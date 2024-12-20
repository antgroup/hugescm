package streamio

import (
	"io"
	"sync"

	"github.com/klauspost/compress/zstd"
)

var (
	zstdReader = sync.Pool{
		New: func() any {
			d, _ := zstd.NewReader(nil)
			return &ZstdDecoder{
				Decoder: d,
			}
		},
	}
	zstdWriter = sync.Pool{
		New: func() any {
			e, _ := zstd.NewWriter(nil)
			return &ZstdEncoder{
				Encoder: e,
			}
		},
	}
)

type ZstdDecoder struct {
	*zstd.Decoder
}

// GetZstdReader returns a ZstdDecoder that is managed by a sync.Pool.
// Returns a ZLibReader that is reset using a dictionary that is
// also managed by a sync.Pool.
//
// After use, the ZstdDecoder should be put back into the sync.Pool
// by calling PutZstdReader.
func GetZstdReader(r io.Reader) (*ZstdDecoder, error) {
	z := zstdReader.Get().(*ZstdDecoder)

	err := z.Reset(r)

	return z, err
}

// PutZstdReader puts z back into its sync.Pool, first closing the reader.
// The Byte slice dictionary is also put back into its sync.Pool.
func PutZstdReader(z *ZstdDecoder) {
	zstdReader.Put(z)
}

type ZstdEncoder struct {
	*zstd.Encoder
}

// GetZstdWriter returns a *ztsd.Encoder that is managed by a sync.Pool.
// Returns a writer that is reset with w and ready for use.
//
// After use, the *ztsd.Encoder  should be put back into the sync.Pool
// by calling PutZstdWriter.
func GetZstdWriter(w io.Writer) *ZstdEncoder {
	z := zstdWriter.Get().(*ZstdEncoder)
	z.Reset(w)
	return z
}

// PutZstdWriter puts w back into its sync.Pool.
func PutZstdWriter(w *ZstdEncoder) {
	w.Encoder.Close() // close flush writer
	zstdWriter.Put(w)
}
