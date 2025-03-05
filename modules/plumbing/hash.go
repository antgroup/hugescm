package plumbing

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"sort"

	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/zeebo/blake3"
)

const (
	HASH_DIGEST_SIZE = 32
	HASH_HEX_SIZE    = 64
	reverseHexTable  = "" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\x00\x01\x02\x03\x04\x05\x06\x07\x08\x09\xff\xff\xff\xff\xff\xff" +
		"\xff\x0a\x0b\x0c\x0d\x0e\x0f\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\x0a\x0b\x0c\x0d\x0e\x0f\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff" +
		"\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff\xff"
)

const (
	BLANK_BLOB = "af1349b9f5f9a1a6a0404dea36dcc9499bcb25c9adc112b7cc9a93cae41f3262"
	ZERO_OID   = "0000000000000000000000000000000000000000000000000000000000000000"
)

// Hash BLAKE3 hashed content
type Hash [HASH_DIGEST_SIZE]byte

func (h Hash) MarshalJSON() ([]byte, error) {
	return strengthen.BufferCat("\"", h.String(), "\""), nil
}

func (h *Hash) UnmarshalJSON(b []byte) error {
	var s string
	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}
	hashBytes, _ := hex.DecodeString(s)
	copy(h[:], hashBytes)
	return nil
}

// TOML
func (h Hash) MarshalText() ([]byte, error) {
	return []byte(h.String()), nil
}

func (h *Hash) UnmarshalText(text []byte) error {
	hashBytes, _ := hex.DecodeString(string(text))
	copy(h[:], hashBytes)
	return nil
}

// ZeroHash is Hash with value zero
var ZeroHash Hash

// NewHash return a new Hash from a hexadecimal hash representation
func NewHash(s string) Hash {
	b, _ := hex.DecodeString(s)

	var h Hash
	copy(h[:], b)

	return h
}

func (h Hash) IsZero() bool {
	var empty Hash
	return h == empty
}

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

func (h Hash) Shorten() int {
	i := HASH_DIGEST_SIZE - 1
	for ; i >= 4; i-- {
		if h[i] != 0 {
			return i + 1
		}
	}
	return i + 1
}

func (h Hash) Prefix() string {
	return hex.EncodeToString(h[:h.Shorten()])
}

// HashesSort sorts a slice of Hashes in increasing order.
func HashesSort(a []Hash) {
	sort.Sort(HashSlice(a))
}

// HashSlice attaches the methods of sort.Interface to []Hash, sorting in
// increasing order.
type HashSlice []Hash

func (p HashSlice) Len() int           { return len(p) }
func (p HashSlice) Less(i, j int) bool { return bytes.Compare(p[i][:], p[j][:]) < 0 }
func (p HashSlice) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }

// ValidateHashHex returns true if the given string is a valid hash.
func ValidateHashHex(s string) bool {
	if len(s) != HASH_HEX_SIZE {
		return false
	}
	bs := []byte(s)
	for _, b := range bs {
		if c := reverseHexTable[b]; c > 0x0f {
			return false
		}
	}
	return true
}

func NewHashEx(s string) (Hash, error) {
	if !ValidateHashHex(s) {
		return ZeroHash, fmt.Errorf("zeta: '%s' not a valid object name", s)
	}
	return NewHash(s), nil
}

func IsLooseDir(s string) bool {
	if len(s) != 2 {
		return false
	}
	bs := []byte(s)
	for _, b := range bs {
		if c := reverseHexTable[b]; c > 0x0f {
			return false
		}
	}
	return true
}

type Hasher struct {
	hash.Hash
}

func NewHasher() Hasher {
	return Hasher{Hash: blake3.New()}
}

func (h Hasher) Sum() (hash Hash) {
	copy(hash[:], h.Hash.Sum(nil))
	return
}
