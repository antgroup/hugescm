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
	// ErrBinaryData is returned when the content is detected as binary
	// 当内容被检测为二进制时返回此错误
	ErrBinaryData = errors.New("binary data")
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
	// Read initial bytes for charset detection / 读取初始字节用于字符集检测
	sniffBytes, err := streamio.ReadMax(r, sniffLen)
	if err != nil {
		return "", "", fmt.Errorf("failed to read initial bytes for charset detection: %w", err)
	}

	// Detect charset / 检测字符集
	charset := detectCharset(sniffBytes)
	if charset == BINARY {
		return "", "", fmt.Errorf("%w: content appears to be binary", ErrBinaryData)
	}

	// Create combined reader / 创建组合读取器
	reader := io.MultiReader(bytes.NewReader(sniffBytes), r)

	// Handle UTF-8 content / 处理 UTF-8 内容
	if strings.EqualFold(charset, UTF8) {
		var b strings.Builder
		if _, err := io.Copy(&b, reader); err != nil {
			return "", "", fmt.Errorf("failed to read UTF-8 content: %w", err)
		}
		return b.String(), UTF8, nil
	}

	// Handle other charsets / 处理其他字符集
	var b bytes.Buffer
	if _, err := b.ReadFrom(reader); err != nil {
		return "", "", fmt.Errorf("failed to read content: %w", err)
	}

	// Convert from detected charset / 从检测到的字符集转换
	buf, err := chardet.DecodeFromCharset(b.Bytes(), charset)
	if err != nil {
		return "", "", fmt.Errorf("failed to convert from charset '%s': %w", charset, err)
	}

	if len(buf) == 0 {
		return "", charset, nil
	}

	return unsafe.String(unsafe.SliceData(buf), len(buf)), charset, nil
}

func readRawText(r io.Reader, size int) (string, error) {
	var b bytes.Buffer

	// Read initial bytes for binary detection / 读取初始字节进行二进制检测
	if _, err := b.ReadFrom(io.LimitReader(r, sniffLen)); err != nil {
		return "", fmt.Errorf("failed to read initial bytes: %w", err)
	}

	// Check for null bytes (binary content) / 检查空字节（二进制内容）
	if bytes.IndexByte(b.Bytes(), 0) != -1 {
		return "", fmt.Errorf("%w: detected null byte in content", ErrBinaryData)
	}

	// Pre-allocate buffer for remaining content / 为剩余内容预分配缓冲区
	b.Grow(size)

	// Read remaining content / 读取剩余内容
	if _, err := b.ReadFrom(r); err != nil {
		return "", fmt.Errorf("failed to read remaining content: %w", err)
	}

	content := b.Bytes()
	return unsafe.String(unsafe.SliceData(content), len(content)), nil
}

func ReadUnifiedText(r io.Reader, size int64, textconv bool) (content string, charset string, err error) {
	// Validate size / 验证大小
	if size > MAX_DIFF_SIZE {
		return "", "", fmt.Errorf("file size %d bytes exceeds limit %d bytes", size, MAX_DIFF_SIZE)
	}

	if textconv {
		return readUnifiedText(r)
	}

	content, err = readRawText(r, int(size))
	if err != nil {
		return "", "", fmt.Errorf("failed to read raw text: %w", err)
	}

	return content, UTF8, nil
}

func NewUnifiedReaderEx(r io.Reader, textconv bool) (io.Reader, string, error) {
	sniffBytes, err := streamio.ReadMax(r, sniffLen)
	if err != nil {
		return nil, "", err
	}
	reader := io.MultiReader(bytes.NewReader(sniffBytes), r)
	if !textconv {
		if bytes.IndexByte(sniffBytes, 0) != -1 {
			return reader, BINARY, nil
		}
		return reader, UTF8, nil
	}
	charset := detectCharset(sniffBytes)
	// binary or UTF-8 not need convert
	if charset == BINARY || strings.EqualFold(charset, UTF8) {
		return reader, charset, nil
	}
	return chardet.NewReader(reader, charset), charset, nil
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
		return nil, ErrBinaryData
	}
	return io.MultiReader(bytes.NewReader(sniffBytes), r), nil
}
