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

// StrSplitSkipEmpty skip empty string suggestcap is suggest cap
func StrSplitSkipEmpty(s string, sep byte, suggestcap int) []string {
	sv := make([]string, 0, suggestcap)
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

func ParseUID(glid string) (int64, error) {
	if !strings.HasPrefix(glid, "user-") {
		return 0, ErrNotUserID
	}
	return strconv.ParseInt(glid[5:], 10, 64)
}

func DecodeUID(glid string) int64 {
	if !strings.HasPrefix(glid, "user-") {
		uid, _ := strconv.ParseInt(glid, 10, 64)
		return uid
	}
	uid, _ := strconv.ParseInt(glid[5:], 10, 64)
	return uid
}

func ParseKID(glid string) (int64, error) {
	if !strings.HasPrefix(glid, "key-") {
		return 0, ErrNotKeyID
	}
	return strconv.ParseInt(glid[4:], 10, 64)
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
	Byte int64 = 1 << (iota * 10)
	KiByte
	MiByte
	GiByte
	TiByte
	PiByte
	EiByte
)

func toLower(c byte) byte {
	if 'A' <= c && c <= 'Z' {
		c += 'a' - 'A'
	}
	return c
}

var (
	ErrSyntaxSize = errors.New("size synatx error")
)

func ParseSize(data string) (int64, error) {
	var ratio int64 = Byte
	if len(data) == 0 {
		return 0, ErrSyntaxSize
	}
	switch toLower(data[len(data)-1]) {
	case 'k':
		ratio = KiByte
		data = data[0 : len(data)-1]
	case 'm':
		ratio = MiByte
		data = data[0 : len(data)-1]
	case 'g':
		ratio = GiByte
		data = data[0 : len(data)-1]
	case 't':
		ratio = GiByte
		data = data[0 : len(data)-1]
	case 'p':
		ratio = PiByte
		data = data[0 : len(data)-1]
	case 'e':
		ratio = EiByte
		data = data[0 : len(data)-1]
	}
	sz, err := strconv.ParseInt(strings.TrimSpace(data), 10, 64)
	if err != nil {
		return 0, ErrSyntaxSize
	}
	return sz * ratio, nil
}
