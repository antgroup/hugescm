package lfs

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/git/gitobj"
)

const (
	// blobSizeCutoff is used to determine which files to scan for Git LFS
	// pointers.  Any file with a size below this cutoff will be scanned.
	blobSizeCutoff = 1024
)

const (
	PointerMIME = "text/vnd.git-lfs"
)

var (
	v1Aliases = []string{
		"http://git-media.io/v/2",            // alpha
		"https://hawser.github.com/spec/v1",  // pre-release
		"https://git-lfs.github.com/spec/v1", // public launch
	}
	latest            = "https://git-lfs.github.com/spec/v1"
	oidType           = "sha256"
	oidRE             = regexp.MustCompile(`\A[0-9a-f]{64}\z`)
	matcherRE         = regexp.MustCompile("git-media|hawser|git-lfs")
	extRE             = regexp.MustCompile(`\Aext-\d{1}-\w+`)
	pointerKeys       = []string{"version", "oid", "size"}
	EmptyObjectSHA256 = hex.EncodeToString(sha256.New().Sum(nil))
)

type Pointer struct {
	Version    string
	Oid        string
	Size       int64
	OidType    string
	Extensions []*PointerExtension
	Canonical  bool
}

// A PointerExtension is parsed from the Git LFS Pointer file.
type PointerExtension struct {
	Name     string
	Priority int
	Oid      string
	OidType  string
}

type ByPriority []*PointerExtension

func (p ByPriority) Len() int           { return len(p) }
func (p ByPriority) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p ByPriority) Less(i, j int) bool { return p[i].Priority < p[j].Priority }

func NewPointer(oid string, size int64, exts []*PointerExtension) *Pointer {
	return &Pointer{latest, oid, size, oidType, exts, true}
}

func NewPointerExtension(name string, priority int, oid string) *PointerExtension {
	return &PointerExtension{name, priority, oid, oidType}
}

func (p *Pointer) Encode(writer io.Writer) (int, error) {
	return EncodePointer(writer, p)
}

func (p *Pointer) Encoded() string {
	if p.Size == 0 {
		return ""
	}

	var buffer bytes.Buffer
	buffer.WriteString(fmt.Sprintf("version %s\n", latest))
	for _, ext := range p.Extensions {
		buffer.WriteString(fmt.Sprintf("ext-%d-%s %s:%s\n", ext.Priority, ext.Name, ext.OidType, ext.Oid))
	}
	buffer.WriteString(fmt.Sprintf("oid %s:%s\n", p.OidType, p.Oid))
	buffer.WriteString(fmt.Sprintf("size %d\n", p.Size))
	return buffer.String()
}

func EmptyPointer() *Pointer {
	return NewPointer(EmptyObjectSHA256, 0, nil)
}

func EncodePointer(writer io.Writer, pointer *Pointer) (int, error) {
	return writer.Write([]byte(pointer.Encoded()))
}

func DecodePointerFromBlob(b *gitobj.Blob) (*Pointer, error) {
	// Check size before reading
	if b.Size >= blobSizeCutoff {
		return nil, NewNotAPointerError("blob size exceeds Git LFS pointer size cutoff")
	}
	return DecodePointer(b.Contents)
}

func DecodePointer(reader io.Reader) (*Pointer, error) {
	p, _, err := DecodeFrom(reader)
	return p, err
}

// DecodeFrom decodes an *lfs.Pointer from the given io.Reader, "reader".
// If the pointer encoded in the reader could successfully be read and decoded,
// it will be returned with a nil error.
//
// If the pointer could not be decoded, an io.Reader containing the entire
// blob's data will be returned, along with a parse error.
func DecodeFrom(reader io.Reader) (*Pointer, io.Reader, error) {
	buf := make([]byte, blobSizeCutoff)
	n, err := reader.Read(buf)
	buf = buf[:n]

	var contents io.Reader = bytes.NewReader(buf)
	if err != io.EOF {
		contents = io.MultiReader(contents, reader)
	}

	if err != nil && err != io.EOF {
		return nil, contents, err
	}

	if len(buf) == 0 {
		return EmptyPointer(), contents, nil
	}

	p, err := decodeKV(bytes.TrimSpace(buf))
	if err == nil && p != nil {
		p.Canonical = p.Encoded() == string(buf)
	}
	return p, contents, err
}

func Decode(buf []byte) (*Pointer, error) {
	if len(buf) >= blobSizeCutoff {
		return nil, NewNotAPointerError("blob size exceeds Git LFS pointer size cutoff")
	}
	p, err := decodeKV(bytes.TrimSpace(buf))
	if err == nil && p != nil {
		p.Canonical = p.Encoded() == string(buf)
	}
	return p, err
}

func verifyVersion(version string) error {
	if len(version) == 0 {
		return NewNotAPointerError("Missing version")
	}

	for _, v := range v1Aliases {
		if v == version {
			return nil
		}
	}

	return fmt.Errorf("invalid version: %s", version)
}

func decodeKV(data []byte) (*Pointer, error) {
	kvps, exts, err := decodeKVData(data)
	if err != nil {
		if IsBadPointerKeyError(err) {
			return nil, NewNotAPointerError(err.Error())
		}
		return nil, err
	}

	if err := verifyVersion(kvps["version"]); err != nil {
		return nil, err
	}

	value, ok := kvps["oid"]
	if !ok {
		return nil, errors.New("invalid OID")
	}

	oid, err := parseOid(value)
	if err != nil {
		return nil, err
	}

	value = kvps["size"]
	size, err := strconv.ParseInt(value, 10, 64)
	if err != nil || size < 0 {
		return nil, fmt.Errorf("invalid size: %q", value)
	}

	var extensions []*PointerExtension
	if exts != nil {
		for key, value := range exts {
			ext, err := parsePointerExtension(key, value)
			if err != nil {
				return nil, err
			}
			extensions = append(extensions, ext)
		}
		if err = validatePointerExtensions(extensions); err != nil {
			return nil, err
		}
		sort.Sort(ByPriority(extensions))
	}

	return NewPointer(oid, size, extensions), nil
}

func parseOid(value string) (string, error) {
	parts := strings.SplitN(value, ":", 2)
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid OID value: %s", value)
	}
	if parts[0] != oidType {
		return "", fmt.Errorf("invalid OID type: %s", parts[0])
	}
	oid := parts[1]
	if !oidRE.Match([]byte(oid)) {
		return "", fmt.Errorf("invalid OID: %s", oid)
	}
	return oid, nil
}

func parsePointerExtension(key string, value string) (*PointerExtension, error) {
	keyParts := strings.SplitN(key, "-", 3)
	if len(keyParts) != 3 || keyParts[0] != "ext" {
		return nil, fmt.Errorf("invalid extension value: %s", value)
	}

	p, err := strconv.Atoi(keyParts[1])
	if err != nil || p < 0 {
		return nil, fmt.Errorf("invalid priority: %s", keyParts[1])
	}

	name := keyParts[2]

	oid, err := parseOid(value)
	if err != nil {
		return nil, err
	}

	return NewPointerExtension(name, p, oid), nil
}

func validatePointerExtensions(exts []*PointerExtension) error {
	m := make(map[int]struct{})
	for _, ext := range exts {
		if _, exist := m[ext.Priority]; exist {
			return fmt.Errorf("duplicate priority found: %d", ext.Priority)
		}
		m[ext.Priority] = struct{}{}
	}
	return nil
}

func decodeKVData(data []byte) (kvps map[string]string, exts map[string]string, err error) {
	kvps = make(map[string]string)

	if !matcherRE.Match(data) {
		err = NewNotAPointerError("invalid header")
		return
	}

	scanner := bufio.NewScanner(bytes.NewBuffer(data))
	line := 0
	numKeys := len(pointerKeys)
	for scanner.Scan() {
		text := scanner.Text()
		if len(text) == 0 {
			continue
		}

		parts := strings.SplitN(text, " ", 2)
		if len(parts) < 2 {
			err = NewNotAPointerError(fmt.Sprintf("error reading line %d: %s", line, text))
			return
		}

		key := parts[0]
		value := parts[1]

		if numKeys <= line {
			err = NewNotAPointerError(fmt.Sprintf("extra line: %s", text))
			return
		}

		if expected := pointerKeys[line]; key != expected {
			if !extRE.Match([]byte(key)) {
				err = NewBadPointerKeyError(fmt.Sprintf("got %s want: %s", expected, key))
				return
			}
			if exts == nil {
				exts = make(map[string]string)
			}
			exts[key] = value
			continue
		}

		line += 1
		kvps[key] = value
	}

	err = scanner.Err()
	return
}

func EncodeSimple(oid string, size int64) string {
	p := &Pointer{Oid: oid, Size: size, OidType: oidType}
	return p.Encoded()
}
