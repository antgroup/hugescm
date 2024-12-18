package diferenco

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"unsafe"

	"github.com/antgroup/hugescm/modules/chardet"
	"github.com/antgroup/hugescm/modules/mime"
	"github.com/antgroup/hugescm/modules/streamio"
)

// /*
//  * xdiff isn't equipped to handle content over a gigabyte;
//  * we make the cutoff 1GB - 1MB to give some breathing
//  * room for constant-sized additions (e.g., merge markers)
//  */
//  #define MAX_XDIFF_SIZE (1024UL * 1024 * 1023)

const (
	MAX_DIFF_SIZE = 100 << 20 // MAX_DIFF_SIZE 100MiB
	BINARY        = "binary"
	UTF8          = "UTF-8"
	sniffLen      = 8000
)

var (
	ErrNonTextContent = errors.New("non-text content")
)

func checkCharset(s string) string {
	if _, charset, ok := strings.Cut(s, ";"); ok {
		return strings.TrimPrefix(strings.TrimSpace(charset), "charset=")
	}
	return UTF8
}

func detectCharset(payload []byte) string {
	result := mime.DetectAny(payload)
	for p := result; p != nil; p = p.Parent() {
		if p.Is("text/plain") {
			return checkCharset(p.String())
		}
	}
	return BINARY
}

func readUnifiedText(r io.Reader) (string, string, error) {
	sniffBytes, err := streamio.ReadMax(r, sniffLen)
	if err != nil {
		return "", "", err
	}
	charset := detectCharset(sniffBytes)
	if charset == BINARY {
		return "", "", ErrNonTextContent
	}
	reader := io.MultiReader(bytes.NewReader(sniffBytes), r)
	if strings.EqualFold(charset, UTF8) {
		var b strings.Builder
		if _, err := io.Copy(&b, reader); err != nil {
			return "", "", err
		}
		return b.String(), UTF8, nil
	}
	var b bytes.Buffer
	if _, err := b.ReadFrom(reader); err != nil {
		return "", "", err
	}
	buf, err := chardet.DecodeFromCharset(b.Bytes(), charset)
	if err != nil {
		return "", "", ErrNonTextContent
	}
	if len(buf) == 0 {
		return "", "", nil
	}
	return unsafe.String(unsafe.SliceData(buf), len(buf)), charset, nil
}

func readRawText(r io.Reader, size int) (string, error) {
	var b bytes.Buffer
	if _, err := b.ReadFrom(io.LimitReader(r, sniffLen)); err != nil {
		return "", err
	}
	if bytes.IndexByte(b.Bytes(), 0) != -1 {
		return "", ErrNonTextContent
	}
	b.Grow(size)
	if _, err := b.ReadFrom(r); err != nil {
		return "", err
	}
	content := b.Bytes()
	return unsafe.String(unsafe.SliceData(content), len(content)), nil
}

func ReadUnifiedText(r io.Reader, size int64, textConv bool) (content string, charset string, err error) {
	if size > MAX_DIFF_SIZE {
		return "", "", ErrNonTextContent
	}
	if textConv {
		return readUnifiedText(r)
	}
	content, err = readRawText(r, int(size))
	return content, UTF8, err
}

func NewUnifiedReader(r io.Reader) (io.Reader, error) {
	sniffBytes, err := streamio.ReadMax(r, sniffLen)
	if err != nil {
		return nil, err
	}
	charset := detectCharset(sniffBytes)
	reader := io.MultiReader(bytes.NewReader(sniffBytes), r)
	// binary or UTF-8 not need convert
	if charset == BINARY || strings.EqualFold(charset, UTF8) {
		return reader, nil
	}
	return chardet.NewReader(reader, charset), nil
}

func NewTextReader(r io.Reader) (io.Reader, error) {
	sniffBytes, err := streamio.ReadMax(r, sniffLen)
	if err != nil {
		return nil, err
	}
	if bytes.IndexByte(sniffBytes, 0) != -1 {
		return nil, ErrNonTextContent
	}
	return io.MultiReader(bytes.NewReader(sniffBytes), r), nil
}
