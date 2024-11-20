package git

import (
	"crypto/sha1"
	"crypto/sha256"
	"fmt"
	"hash"
)

const (
	GIT_HASH_UNKNOWN      = 0
	GIT_HASH_SHA1         = 1
	GIT_HASH_SHA256       = 2
	GIT_SHA1_RAWSZ        = 20
	GIT_SHA1_HEXSZ        = GIT_SHA1_RAWSZ * 2
	GIT_SHA256_RAWSZ      = 32
	GIT_SHA256_HEXSZ      = GIT_SHA256_RAWSZ * 2
	GIT_MAX_RAWSZ         = GIT_SHA256_RAWSZ
	GIT_MAX_HEXSZ         = GIT_SHA256_HEXSZ
	GIT_SHA1_ZERO_HEX     = "0000000000000000000000000000000000000000"
	GIT_SHA256_ZERO_HEX   = "0000000000000000000000000000000000000000000000000000000000000000"
	GIT_SHA1_EMPTY_TREE   = "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	GIT_SHA1_EMPTY_BLOB   = "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"
	GIT_SHA256_EMPTY_TREE = "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321"
	GIT_SHA256_EMPTY_BLOB = "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813"
	GIT_SHA1_NAME         = "sha1"
	GIT_SHA256_NAME       = "sha256"
	HashKey               = "hash-algo"
	ReferenceNameDefault  = "refs/heads/master"
)

// HashAlgo: https://git-scm.com/docs/hash-function-transition/
type HashAlgo int

const (
	HashAlgoUNKNOWN HashAlgo = iota // UNKNOWN
	HashAlgoSHA1                    // SHA1
	HashAlgoSHA256                  // SHA256
)

func (h HashAlgo) String() string {
	switch h {
	case HashAlgoSHA1:
		return GIT_SHA1_NAME
	case HashAlgoSHA256:
		return GIT_SHA256_NAME
	}
	return "unknown"
}

// RawSize: raw length
func (h HashAlgo) RawSize() int {
	switch h {
	case HashAlgoSHA1:
		return GIT_SHA1_RAWSZ
	case HashAlgoSHA256:
		return GIT_SHA256_RAWSZ
	}
	return 0
}

// HexSize: hex size
func (h HashAlgo) HexSize() int {
	switch h {
	case HashAlgoSHA1:
		return GIT_SHA1_HEXSZ
	case HashAlgoSHA256:
		return GIT_SHA256_HEXSZ
	}
	return 0
}

func (h HashAlgo) EmptyTreeID() string {
	switch h {
	case HashAlgoSHA1:
		return GIT_SHA1_EMPTY_TREE
	case HashAlgoSHA256:
		return GIT_SHA256_EMPTY_TREE
	}
	return ""
}

func (h HashAlgo) EmptyBlobID() string {
	switch h {
	case HashAlgoSHA1:
		return GIT_SHA1_EMPTY_BLOB
	case HashAlgoSHA256:
		return GIT_SHA256_EMPTY_BLOB
	}
	return ""
}

func (h HashAlgo) ZeroOID() string {
	switch h {
	case HashAlgoSHA1:
		return GIT_SHA1_ZERO_HEX
	case HashAlgoSHA256:
		return GIT_SHA256_ZERO_HEX
	}
	return ""
}

func (h HashAlgo) Hasher() hash.Hash {
	switch h {
	case HashAlgoSHA1:
		return sha1.New()
	case HashAlgoSHA256:
		return sha256.New()
	}
	return sha1.New()
}

func HashAlgoFromName(algo string) HashAlgo {
	switch algo {
	case GIT_SHA1_NAME:
		return HashAlgoSHA1
	case GIT_SHA256_NAME:
		return HashAlgoSHA256
	}
	return HashAlgoSHA1
}

func HashAlgoFromHexSize(hlen int) HashAlgo {
	switch hlen {
	case GIT_SHA1_HEXSZ:
		return HashAlgoSHA1
	case GIT_SHA256_HEXSZ:
		return HashAlgoSHA256
	}
	return HashAlgoUNKNOWN
}

func HashAlgoFromRawSize(rlen int) HashAlgo {
	switch rlen {
	case GIT_SHA1_RAWSZ:
		return HashAlgoSHA1
	case GIT_SHA256_RAWSZ:
		return HashAlgoSHA256
	}
	return HashAlgoUNKNOWN
}

const (
	reverseHexTable = "" +
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

// var (
// 	sha1Regex   = regexp.MustCompile(`\A[0-9a-f]{40}\z`)
// 	sha256Regex = regexp.MustCompile(`\A[0-9a-f]{64}\z`)
// )

func ValidateHexLax(hs string) bool {
	bs := []byte(hs)
	if len(bs) < 5 || len(bs) > GIT_SHA256_HEXSZ {
		return false
	}
	for _, b := range bs {
		if c := reverseHexTable[b]; c > 0x0f {
			return false
		}
	}
	return true
}

func ValidateNumber(s string) bool {
	bs := []byte(s)
	for _, b := range bs {
		if c := reverseHexTable[b]; c > 0x9 {
			return false
		}
	}
	return true
}

func ValidateHex(hs string) error {
	bs := []byte(hs)
	if len(bs) != GIT_SHA1_HEXSZ && len(bs) != GIT_SHA256_HEXSZ {
		return fmt.Errorf("object id: %q was not a valid character hexidecimal, len=%d", hs, len(bs))
	}
	for _, b := range bs {
		if c := reverseHexTable[b]; c > 0x0f {
			return fmt.Errorf("object id: %q was not a valid character hexidecimal", hs)
		}
	}
	return nil
}

func IsHashZero(hexOID string) bool {
	if len(hexOID) == GIT_SHA256_HEXSZ {
		return hexOID == GIT_SHA256_ZERO_HEX
	}
	return hexOID == GIT_SHA1_ZERO_HEX
}

func ConformingHashZero(hexOID string) string {
	if len(hexOID) == GIT_SHA256_HEXSZ {
		return GIT_SHA256_ZERO_HEX
	}
	return GIT_SHA1_ZERO_HEX
}

func ConformingEmptyTree(hexOID string) string {
	if len(hexOID) == GIT_SHA256_HEXSZ {
		return GIT_SHA256_EMPTY_TREE
	}
	return GIT_SHA1_EMPTY_TREE
}

func ConformingEmptyBlob(hexOID string) string {
	if len(hexOID) == GIT_SHA256_HEXSZ {
		return GIT_SHA256_EMPTY_BLOB
	}
	return GIT_SHA1_EMPTY_BLOB
}
