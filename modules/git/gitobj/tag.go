package gitobj

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"strings"
)

type Tag struct {
	Object     []byte
	ObjectType ObjectType
	Name       string
	Tagger     string

	Message string
}

// https://git-scm.com/docs/signature-format
// https://github.blog/changelog/2022-08-23-ssh-commit-verification-now-supported/
func (t *Tag) Extract() (message string, signature string) {
	if i := strings.Index(t.Message, "-----BEGIN"); i > 0 {
		return t.Message[:i], t.Message[i:]
	}
	return t.Message, ""
}

func (t *Tag) StrictMessage() string {
	m, _ := t.Extract()
	return m
}

// Decode implements Object.Decode and decodes the uncompressed tag being
// read. It returns the number of uncompressed bytes being consumed off of the
// stream, which should be strictly equal to the size given.
//
// If any error was encountered along the way it will be returned, and the
// receiving *Tag is considered invalid.
func (t *Tag) Decode(hash hash.Hash, r io.Reader, size int64) (int, error) {
	br := bufio.NewReader(io.LimitReader(r, size))

	var (
		finishedHeaders bool
	)

	var message strings.Builder

	for {
		line, readErr := br.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return 0, readErr
		}

		if finishedHeaders {
			message.WriteString(line)
		} else {
			text := strings.TrimSuffix(line, "\n")
			if len(text) == 0 {
				finishedHeaders = true
				continue
			}

			field, value, ok := strings.Cut(text, " ")
			if !ok {
				return 0, fmt.Errorf("git/object: invalid tag header: %s", text)
			}

			switch field {
			case "object":
				sha, err := hex.DecodeString(value)
				if err != nil {
					return 0, fmt.Errorf("git/object: unable to decode SHA-1: %s", err)
				}

				t.Object = sha
			case "type":
				t.ObjectType = ObjectTypeFromString(value)
			case "tag":
				t.Name = value
			case "tagger":
				t.Tagger = value
			default:
				return 0, fmt.Errorf("git/object: unknown tag header: %s", field)
			}
		}
		if readErr == io.EOF {
			break
		}
	}

	t.Message = message.String()

	return int(size), nil
}

// Encode encodes the Tag's contents to the given io.Writer, "w". If there was
// any error copying the Tag's contents, that error will be returned.
//
// Otherwise, the number of bytes written will be returned.
func (t *Tag) Encode(w io.Writer) (int, error) {
	headers := []string{
		fmt.Sprintf("object %s", hex.EncodeToString(t.Object)),
		fmt.Sprintf("type %s", t.ObjectType),
		fmt.Sprintf("tag %s", t.Name),
		fmt.Sprintf("tagger %s", t.Tagger),
	}

	return fmt.Fprintf(w, "%s\n\n%s", strings.Join(headers, "\n"), t.Message)
}

// Equal returns whether the receiving and given Tags are equal, or in other
// words, whether they are represented by the same SHA-1 when saved to the
// object database.
func (t *Tag) Equal(other *Tag) bool {
	if (t == nil) != (other == nil) {
		return false
	}

	if t != nil {
		return bytes.Equal(t.Object, other.Object) &&
			t.ObjectType == other.ObjectType &&
			t.Name == other.Name &&
			t.Tagger == other.Tagger &&
			t.Message == other.Message
	}

	return true
}

// Type implements Object.ObjectType by returning the correct object type for
// Tags, TagObjectType.
func (t *Tag) Type() ObjectType {
	return TagObjectType
}
