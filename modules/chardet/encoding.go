package chardet

import (
	"fmt"
	"io"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
	"golang.org/x/text/encoding/unicode"
)

var encodings = map[string]encoding.Encoding{
	"iso-8859-2":   charmap.ISO8859_2,
	"iso-8859-3":   charmap.ISO8859_3,
	"iso-8859-4":   charmap.ISO8859_4,
	"iso-8859-5":   charmap.ISO8859_5,
	"iso-8859-6":   charmap.ISO8859_6,
	"iso-8859-7":   charmap.ISO8859_7,
	"iso-8859-8":   charmap.ISO8859_8,
	"iso-8859-8I":  charmap.ISO8859_8I,
	"iso-8859-10":  charmap.ISO8859_10,
	"iso-8859-13":  charmap.ISO8859_13,
	"iso-8859-14":  charmap.ISO8859_14,
	"iso-8859-15":  charmap.ISO8859_15,
	"iso-8859-16":  charmap.ISO8859_16,
	"koi8-r":       charmap.KOI8R,
	"koi8-u":       charmap.KOI8U,
	"windows-874":  charmap.Windows874,
	"windows-1250": charmap.Windows1250,
	"windows-1251": charmap.Windows1251,
	"windows-1252": charmap.Windows1252,
	"windows-1253": charmap.Windows1253,
	"windows-1254": charmap.Windows1254,
	"windows-1255": charmap.Windows1255,
	"windows-1256": charmap.Windows1256,
	"windows-1257": charmap.Windows1257,
	"windows-1258": charmap.Windows1258,
	"gbk":          simplifiedchinese.GBK,
	"gb18030":      simplifiedchinese.GB18030,
	"big5":         traditionalchinese.Big5,
	"euc-jp":       japanese.EUCJP,
	"iso-2022-jp":  japanese.ISO2022JP,
	"shift_jis":    japanese.ShiftJIS,
	"euc-kr":       korean.EUCKR,
	"utf-16be":     unicode.UTF16(unicode.BigEndian, unicode.UseBOM),
	"utf-16le":     unicode.UTF16(unicode.LittleEndian, unicode.UseBOM),
}

// NewReader: convert text from other encodings to UTF-8
func NewReader(r io.Reader, charset string) io.Reader {
	if e, ok := encodings[strings.ToLower(charset)]; ok {
		return e.NewDecoder().Reader(r)
	}
	return r
}

// NewWriter: convert UTF-8 encoding to other encodings
func NewWriter(w io.Writer, charset string) io.Writer {
	if e, ok := encodings[strings.ToLower(charset)]; ok {
		return e.NewEncoder().Writer(w)
	}
	return w
}

// DecodeFromCharset decode input to utf8
func DecodeFromCharset(input []byte, charset string) ([]byte, error) {
	if enc, ok := encodings[strings.ToLower(charset)]; ok {
		return enc.NewDecoder().Bytes(input)
	}
	return nil, fmt.Errorf("unrecognized charset %s", charset)
}

// EncodeToCharset encode input to charset
func EncodeToCharset(input []byte, charset string) ([]byte, error) {
	if e, ok := encodings[strings.ToLower(charset)]; ok {
		return e.NewEncoder().Bytes(input)
	}
	return nil, fmt.Errorf("unrecognized charset %s", charset)
}
