package strengthen

import (
	"bytes"
	"errors"
	"strconv"
	"strings"
)

var (
	ErrNotUserID = errors.New("not user id")
	ErrNotKeyID  = errors.New("not key id")
)

// StrSplitSkipEmpty skip empty string
func StrSplitSkipEmpty(s string, sep byte, cap int) []string {
	sv := make([]string, 0, cap)
	var first, i int
	for ; i < len(s); i++ {
		if s[i] != sep {
			continue
		}
		if first != i {
			sv = append(sv, s[first:i])
		}
		first = i + 1
	}
	if first < len(s) {
		sv = append(sv, s[first:])
	}
	return sv
}

// StrCat cat strings:
// You should know that StrCat gradually builds advantages
// only when the number of parameters is> 2.
func StrCat(sv ...string) string {
	var sb strings.Builder
	var size int
	for _, s := range sv {
		size += len(s)
	}
	sb.Grow(size)
	for _, s := range sv {
		_, _ = sb.WriteString(s)
	}
	return sb.String()
}

// ByteCat cat strings:
// You should know that StrCat gradually builds advantages
// only when the number of parameters is> 2.
func ByteCat(sv ...[]byte) string {
	var b strings.Builder
	var size int
	for _, s := range sv {
		size += len(s)
	}
	b.Grow(size)
	for _, s := range sv {
		_, _ = b.Write(s)
	}
	return b.String()
}

// BufferCat todo
func BufferCat(sv ...string) []byte {
	var buf bytes.Buffer
	var size int
	for _, s := range sv {
		size += len(s)
	}
	buf.Grow(size)
	for _, s := range sv {
		_, _ = buf.WriteString(s)
	}
	return buf.Bytes()
}

// ErrorCat todo
func ErrorCat(sv ...string) error {
	return errors.New(StrCat(sv...))
}

func SimpleAtob(s string, dv bool) bool {
	switch strings.ToLower(s) {
	case "true", "yes", "on", "1":
		return true
	case "false", "no", "off", "0":
		return false
	}
	return dv
}

const (
	Byte = 1 << (iota * 10) // Byte
	KiByte
	MiByte
	GiByte
	TiByte
	PiByte
	EiByte
)

var (
	sizeRatio = map[string]int64{
		"k": KiByte,
		"m": MiByte,
		"g": GiByte,
		"t": TiByte,
		"p": PiByte,
		"e": EiByte,
	}
)

var (
	ErrSyntaxSize = errors.New("size syntax error")
)

func ParseSize(text string) (int64, error) {
	text = strings.TrimSuffix(strings.ToLower(text), "b")
	for rs, ratio := range sizeRatio {
		if prefix, ok := strings.CutSuffix(text, rs); ok {
			v, err := strconv.ParseInt(strings.TrimSpace(prefix), 10, 64)
			if err != nil {
				return 0, ErrSyntaxSize
			}
			return v * ratio, nil
		}
	}
	v, err := strconv.ParseInt(strings.TrimSpace(text), 10, 64)
	if err != nil {
		return 0, ErrSyntaxSize
	}
	return v, nil
}
