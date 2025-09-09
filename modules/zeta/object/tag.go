// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"fmt"
	"io"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
)

var (
	TAG_MAGIC = [4]byte{'Z', 'G', 0x00, 0x01}
)

type Tag struct {
	Hash       plumbing.Hash `json:"hash"`
	Object     plumbing.Hash `json:"object"`
	ObjectType ObjectType    `json:"type"`
	Name       string        `json:"name"`
	Tagger     Signature     `json:"tagger"`

	Content string `json:"content"`
}

// https://git-scm.com/docs/signature-format
// https://github.blog/changelog/2022-08-23-ssh-commit-verification-now-supported/
func (t *Tag) Extract() (message string, signature string) {
	if i := strings.Index(t.Content, "-----BEGIN"); i > 0 {
		return t.Content[:i], t.Content[i:]
	}
	return t.Content, ""
}

func (t *Tag) Message() string {
	m, _ := t.Extract()
	return m
}

// Decode implements Object.Decode and decodes the uncompressed tag being
// read. It returns the number of uncompressed bytes being consumed off of the
// stream, which should be strictly equal to the size given.
//
// If any error was encountered along the way it will be returned, and the
// receiving *Tag is considered invalid.
func (t *Tag) Decode(reader Reader) error {
	if reader.Type() != TagObject {
		return ErrUnsupportedObject
	}
	br := streamio.GetBufioReader(reader)
	defer streamio.PutBufioReader(br)
	t.Hash = reader.Hash()
	var (
		finishedHeaders bool
	)

	var message strings.Builder

	for {
		line, readErr := br.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return readErr
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
				return fmt.Errorf("zeta: invalid tag header: %s", text)
			}

			switch field {
			case "object":
				if !plumbing.ValidateHashHex(value) {
					return fmt.Errorf("zeta: unable to decode BLAKE3: %s", value)
				}

				t.Object = plumbing.NewHash(value)
			case "type":
				t.ObjectType = ObjectTypeFromString(value)
			case "tag":
				t.Name = value
			case "tagger":
				t.Tagger.Decode([]byte(value))
			default:
				return fmt.Errorf("zeta: unknown tag header: %s", field)
			}
		}
		if readErr == io.EOF {
			break
		}
	}

	t.Content = message.String()

	return nil
}

// Encode encodes the Tag's contents to the given io.Writer, "w". If there was
// any error copying the Tag's contents, that error will be returned.
//
// Otherwise, the number of bytes written will be returned.
func (t *Tag) Encode(w io.Writer) error {
	_, err := w.Write(TAG_MAGIC[:])
	if err != nil {
		return err
	}
	headers := []string{
		fmt.Sprintf("object %s", t.Object),
		fmt.Sprintf("type %s", t.ObjectType),
		fmt.Sprintf("tag %s", t.Name),
		fmt.Sprintf("tagger %s", t.Tagger.String()),
	}

	_, err = fmt.Fprintf(w, "%s\n\n%s", strings.Join(headers, "\n"), t.Content)
	return err
}

// Equal returns whether the receiving and given Tags are equal, or in other
// words, whether they are represented by the same SHA-1 when saved to the
// object database.
func (t *Tag) Equal(other *Tag) bool {
	if (t == nil) != (other == nil) {
		return false
	}

	if t != nil {
		return t.Object == other.Object &&
			t.ObjectType == other.ObjectType &&
			t.Name == other.Name &&
			t.Tagger == other.Tagger &&
			t.Content == other.Content
	}

	return true
}

func (t *Tag) Copy() *Tag {
	newTag := *t
	return &newTag
}
