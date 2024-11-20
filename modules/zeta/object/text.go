// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package object

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
	// MAX_DIFF_SIZE  100MiB
	MAX_DIFF_SIZE = 100 * 1024 * 1024
	BINARY        = "binary"
	sniffLen      = 8000
	UTF8          = "UTF-8"
)

var (
	ErrNotTextContent = errors.New("not a text content")
)

func textCharset(s string) string {
	if _, charset, ok := strings.Cut(s, ";"); ok {
		return strings.TrimPrefix(strings.TrimSpace(charset), "charset=")
	}
	return "UTF-8"
}

func resolveCharset(payload []byte) string {
	result := mime.DetectAny(payload)
	for p := result; p != nil; p = p.Parent() {
		if p.Is("text/plain") {
			return textCharset(p.String())
		}
	}
	return BINARY
}

// readText: Read all text content: automatically detect text encoding and convert to UTF-8, binary will return ErrNotTextContent
func readText(r io.Reader) (string, string, error) {
	sniffBytes, err := streamio.ReadMax(r, sniffLen)
	if err != nil {
		return "", "", err
	}
	charset := resolveCharset(sniffBytes)
	if charset == BINARY {
		return "", "", ErrNotTextContent
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
		return "", "", ErrNotTextContent
	}
	if len(buf) == 0 {
		return "", "", nil
	}
	return unsafe.String(unsafe.SliceData(buf), len(buf)), charset, nil
}

func readTextUTF8(r io.Reader) (string, error) {
	var b bytes.Buffer
	if _, err := b.ReadFrom(io.LimitReader(r, sniffLen)); err != nil {
		return "", err
	}
	if bytes.IndexByte(b.Bytes(), 0) != -1 {
		return "", ErrNotTextContent
	}
	if _, err := b.ReadFrom(r); err != nil {
		return "", err
	}
	return b.String(), nil
}

// GetUnifiedText: Read all text content.
func GetUnifiedText(r io.Reader, size int64, codecvt bool) (string, string, error) {
	if size > MAX_DIFF_SIZE {
		return "", "", ErrNotTextContent
	}
	if codecvt {
		return readText(r)
	}
	s, err := readTextUTF8(r)
	return s, UTF8, err
}

func NewUnifiedReader(r io.Reader) (io.Reader, error) {
	sniffBytes, err := streamio.ReadMax(r, sniffLen)
	if err != nil {
		return nil, err
	}
	charset := resolveCharset(sniffBytes)
	reader := io.MultiReader(bytes.NewReader(sniffBytes), r)
	// binary or UTF-8 not need convert
	if charset == BINARY || strings.EqualFold(charset, UTF8) {
		return reader, nil
	}
	return chardet.NewReader(reader, charset), nil
}
