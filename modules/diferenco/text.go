package diferenco

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"unsafe"

	"github.com/antgroup/hugescm/modules/chardet"
	"github.com/antgroup/hugescm/modules/mime"
	"github.com/antgroup/hugescm/modules/streamio"
)

// MAX_DIFF_SIZE mirrors git's xdiff cutoff (1GB - 1MB). We pick a smaller,
// more conservative 100 MiB ceiling because diferenco runs in interactive
// tooling where loading a >100 MiB file into memory is rarely useful.
//
// Reference (git xdiff.h):
//
//	xdiff isn't equipped to handle content over a gigabyte; we make the
//	cutoff 1GB - 1MB to give some breathing room for constant-sized
//	additions (e.g., merge markers).
//	#define MAX_XDIFF_SIZE (1024UL * 1024 * 1023)
const (
	MAX_DIFF_SIZE = 100 << 20 // 100 MiB
	UTF8          = "UTF-8"
	sniffLen      = 8000
)

// ErrNonText is returned by ReadUnifiedText (and helpers) when the
// content is detected as non-text (binary). Callers typically branch on
// this with errors.Is.
var ErrNonText = errors.New("non-text content")

// checkCharset extracts the charset parameter from a MIME type string
// such as "text/plain; charset=GBK". When no charset parameter is
// present it falls back to UTF-8, matching the HTTP/text default.
func checkCharset(s string) string {
	if _, charset, ok := strings.Cut(s, ";"); ok {
		return strings.TrimPrefix(strings.TrimSpace(charset), "charset=")
	}
	return UTF8
}

// charsetFromMIME returns the charset declared on m and reports whether
// m descends from text/plain. The charset parameter is carried on the
// concrete leaf node (see mime.cloneHierarchy in modules/mime), so the
// charset is read from m itself; the text/plain ancestor only serves as
// the "is text" predicate.
func charsetFromMIME(m *mime.MIME) (string, bool) {
	for p := m; p != nil; p = p.Parent() {
		if p.Is("text/plain") {
			return checkCharset(m.String()), true
		}
	}
	return "", false
}

// readUnifiedText reads all of r, performs charset detection on the
// leading sniff window and, when necessary, transcodes the payload to
// UTF-8. It returns ErrNonText when the sniff buffer indicates the
// content is not text.
func readUnifiedText(r io.Reader) (string, string, error) {
	sniffBytes, err := streamio.ReadMax(r, sniffLen)
	if err != nil {
		return "", "", fmt.Errorf("sniff for charset detection: %w", err)
	}

	charset, isText := charsetFromMIME(mime.DetectAny(sniffBytes))
	if !isText {
		return "", "", ErrNonText
	}

	// Materialise sniff + rest into a single bytes.Buffer. bytes.Buffer
	// implements io.ReaderFrom, so ReadFrom below uses the zero-copy
	// path that grows the buffer's own backing array directly instead of
	// going through io.copyBuffer's 32 KiB shuttle.
	var b bytes.Buffer
	b.Grow(len(sniffBytes))
	b.Write(sniffBytes)
	if _, err := b.ReadFrom(r); err != nil {
		return "", "", fmt.Errorf("read content: %w", err)
	}

	// UTF-8 fast path: hand the buffer's bytes to the caller as a string
	// without an extra copy.
	if strings.EqualFold(charset, UTF8) {
		buf := b.Bytes()
		// SAFETY: b is local; the returned string takes over ownership
		// of the underlying array via its string header.
		return unsafe.String(unsafe.SliceData(buf), len(buf)), UTF8, nil
	}

	// Other charsets: transcode through chardet.
	decoded, err := chardet.DecodeFromCharset(b.Bytes(), charset)
	if err != nil {
		return "", "", fmt.Errorf("decode charset %q: %w", charset, err)
	}
	// SAFETY: decoded is the freshly-allocated slice returned by chardet
	// and is not retained or mutated after this call.
	return unsafe.String(unsafe.SliceData(decoded), len(decoded)), charset, nil
}

// readRawText reads r into memory without any charset transcoding. It
// rejects content whose sniff window contains a NUL byte (the same
// heuristic git uses to classify a blob as binary).
//
// size is a hint used for buffer pre-sizing only; the function still
// reads until r returns EOF.
func readRawText(r io.Reader, size int) (string, error) {
	if size < 0 {
		size = 0
	}

	var b bytes.Buffer
	if size > 0 {
		b.Grow(size)
	}

	// Single ReadFrom path: bytes.Buffer.ReadFrom uses the zero-copy
	// read-into-own-buffer loop, so we get the whole payload with one
	// allocation chain regardless of whether r implements WriterTo.
	if _, err := b.ReadFrom(r); err != nil {
		return "", fmt.Errorf("read content: %w", err)
	}

	// NUL-byte scan only on the sniff window to match git's behaviour.
	head := b.Bytes()
	if len(head) > sniffLen {
		head = head[:sniffLen]
	}
	if bytes.IndexByte(head, 0) != -1 {
		return "", ErrNonText
	}

	buf := b.Bytes()
	// SAFETY: b goes out of scope when this function returns; the
	// returned string takes over ownership of the underlying array.
	return unsafe.String(unsafe.SliceData(buf), len(buf)), nil
}

// ReadUnifiedText reads r into a string suitable for unified-diff input.
//
// When textconv is true the content is sniffed and, if needed, transcoded
// to UTF-8 via the chardet helpers. When textconv is false the bytes are
// returned verbatim and the only check performed is a NUL-byte scan of
// the sniff window (matching git's binary heuristic). Either path returns
// ErrNonText for non-text content.
//
// size is the caller's expected length in bytes. Anything exceeding
// MAX_DIFF_SIZE is rejected up front to avoid OOMing on giant blobs.
func ReadUnifiedText(r io.Reader, size int64, textconv bool) (content string, charset string, err error) {
	if size > MAX_DIFF_SIZE {
		return "", "", fmt.Errorf("file size %d bytes exceeds limit %d bytes", size, MAX_DIFF_SIZE)
	}

	if textconv {
		return readUnifiedText(r)
	}

	// Clamp negative or oversized size hints; readRawText still reads to
	// EOF, this only governs initial buffer sizing.
	hint := min(max(size, 0), MAX_DIFF_SIZE)

	content, err = readRawText(r, int(hint))
	if err != nil {
		return "", "", err
	}
	return content, UTF8, nil
}

// Metadata describes the content detected on the reader produced by
// NewUnifiedReader. MIME is always non-nil; Charset holds the detected
// text encoding (e.g. UTF-8 or "GBK") and is empty when the content is
// non-text. Use IsText to branch on the text/binary distinction.
type Metadata struct {
	MIME    *mime.MIME
	Charset string
}

// IsText reports whether the content was detected as text, i.e. the
// reader yields a meaningful (possibly transcoded) UTF-8 byte stream.
func (m *Metadata) IsText() bool {
	return m != nil && m.Charset != ""
}

// NewUnifiedReader sniffs r for charset / MIME, then returns a reader
// that yields UTF-8 bytes (transcoding via chardet when needed) plus a
// Metadata describing what was detected. Metadata.MIME is never nil (it
// falls back to the root application/octet-stream node when nothing else
// matches), so callers can safely chain Is/Parent calls without nil
// checks.
//
// When textconv is false the function uses the cheap NUL-byte heuristic
// to decide text vs binary (so existing text/binary boundaries are
// unchanged), but still runs MIME detection on the sniff buffer so
// callers can react to specific binary subtypes (e.g. images).
func NewUnifiedReader(r io.Reader, textconv bool) (io.Reader, *Metadata, error) {
	sniffBytes, err := streamio.ReadMax(r, sniffLen)
	if err != nil {
		return nil, nil, err
	}
	reader := io.MultiReader(bytes.NewReader(sniffBytes), r)
	m := mime.DetectAny(sniffBytes)
	if !textconv {
		charset := UTF8
		if bytes.IndexByte(sniffBytes, 0) != -1 {
			charset = ""
		}
		return reader, &Metadata{MIME: m, Charset: charset}, nil
	}
	charset, isText := charsetFromMIME(m)
	meta := &Metadata{MIME: m, Charset: charset}
	// non-text or UTF-8 content is forwarded as-is.
	if !isText || strings.EqualFold(charset, UTF8) {
		return reader, meta, nil
	}
	return chardet.NewReader(reader, charset), meta, nil
}
