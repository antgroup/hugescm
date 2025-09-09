package git

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
)

func JoinTagPrefix(tag string) string {
	if strings.HasPrefix(tag, refTagPrefix) {
		return tag
	}
	return refTagPrefix + tag
}

type Tag struct {
	// Hash of the tag.
	Hash string `json:"hash"`
	// Name of the tag.
	Name string `json:"name"`
	// Object is the hash of the target object.
	Object string `json:"object"`
	// Type is the object type of the target.
	Type string `json:"type"`
	// Tagger is the one who created the tag.
	Tagger Signature `json:"tagger"`
	// Content is an arbitrary text message.
	Content string `json:"content"`
	size    int64
}

func (t *Tag) Size() int64 {
	return t.size
}

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

func (t *Tag) ExtractCommitGPGSignature() *CommitGPGSignature {
	message, signature := t.Extract()
	if len(signature) == 0 {
		return nil
	}
	var w strings.Builder
	var err error

	if _, err = fmt.Fprintf(&w,
		"object %s\ntype %s\ntag %s\ntagger ",
		t.Object, t.Type, t.Name); err != nil {
		return nil
	}

	if err = t.Tagger.Encode(&w); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, "\n\n"); err != nil {
		return nil
	}

	if _, err = w.WriteString(message); err != nil {
		return nil
	}

	return &CommitGPGSignature{
		Signature: signature,
		Payload:   strings.TrimSpace(w.String()) + "\n",
	}
}

// https://git-scm.com/docs/signature-format
// https://github.blog/changelog/2022-08-23-ssh-commit-verification-now-supported/

func (t *Tag) Decode(hash string, reader io.Reader, size int64) error {
	t.Hash = hash
	t.size = size
	r, ok := reader.(*bufio.Reader)
	if !ok {
		r = bufio.NewReader(reader)
	}
	for {
		line, readErr := r.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return readErr
		}

		line = strings.TrimSpace(line)
		if len(line) == 0 {
			break // Start of message
		}

		field, value, ok := strings.Cut(line, " ")
		if !ok {
			break
		}
		switch string(field) {
		case "object":
			t.Object = value
		case "type":
			t.Type = value
		case "tag":
			t.Name = value
		case "tagger":
			t.Tagger.Decode([]byte(value))
		}

		if readErr == io.EOF {
			return nil
		}
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	t.Content = string(data)
	return nil
}

func FindTag(ctx context.Context, repoPath string, name string) (*Reference, error) {
	stderr := command.NewStderr()
	reader, err := NewReader(ctx, &command.RunOpts{RepoPath: repoPath, Stderr: stderr}, "tag", "-l", "--format", ReferenceLineFormat, "--", name)
	if err != nil {
		return nil, err
	}
	defer reader.Close() // nolint
	scanner := bufio.NewScanner(reader)
	if scanner.Scan() {
		return ParseOneReference(scanner.Text())
	}
	return nil, NewTagNotFound(name)
}
