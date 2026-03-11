package gitobj

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"testing"
)

func TestTagTypeReturnsCorrectObjectType(t *testing.T) {
	if TagObjectType != new(Tag).Type() {
		t.Errorf("Expected %v, got %v", TagObjectType, new(Tag).Type())
	}
}

func TestTagEncode(t *testing.T) {
	tag := &Tag{
		Object:     []byte("aaaaaaaaaaaaaaaaaaaa"),
		ObjectType: CommitObjectType,
		Name:       "v2.4.0",
		Tagger:     "A U Thor <author@example.com>",

		Message: "The quick brown fox jumps over the lazy dog.",
	}

	buf := new(bytes.Buffer)

	n, err := tag.Encode(buf)

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if int64(buf.Len()) != int64(n) {
		t.Errorf("Expected %v, got %v", buf.Len(), n)
	}

	assertLine(t, buf, "object 6161616161616161616161616161616161616161")
	assertLine(t, buf, "type commit")
	assertLine(t, buf, "tag v2.4.0")
	assertLine(t, buf, "tagger A U Thor <author@example.com>")
	assertLine(t, buf, "")
	assertLine(t, buf, "The quick brown fox jumps over the lazy dog.")

	if buf.Len() != 0 {
		t.Errorf("Expected 0, got %v", buf.Len())
	}
}

func TestTagDecode(t *testing.T) {
	from := new(bytes.Buffer)

	fmt.Fprintf(from, "object 6161616161616161616161616161616161616161\n")
	fmt.Fprintf(from, "type commit\n")
	fmt.Fprintf(from, "tag v2.4.0\n")
	fmt.Fprintf(from, "tagger A U Thor <author@example.com>\n")
	fmt.Fprintf(from, "\n")
	fmt.Fprintf(from, "The quick brown fox jumps over the lazy dog.\n")

	flen := from.Len()

	tag := new(Tag)
	n, err := tag.Decode(sha1.New(), from, int64(flen))

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if int64(n) != int64(flen) {
		t.Errorf("Expected %v, got %v", flen, n)
	}

	if !bytes.Equal([]byte("aaaaaaaaaaaaaaaaaaaa"), tag.Object) {
		t.Errorf("Expected %v, got %v", []byte("aaaaaaaaaaaaaaaaaaaa"), tag.Object)
	}
	if CommitObjectType != tag.ObjectType {
		t.Errorf("Expected %v, got %v", CommitObjectType, tag.ObjectType)
	}
	if tag.Name != "v2.4.0" {
		t.Errorf("Expected %v, got %v", "v2.4.0", tag.Name)
	}
	if tag.Tagger != "A U Thor <author@example.com>" {
		t.Errorf("Expected %v, got %v", "A U Thor <author@example.com>", tag.Tagger)
	}
	if tag.Message != "The quick brown fox jumps over the lazy dog.\n" {
		t.Errorf("Expected %v, got %v", "The quick brown fox jumps over the lazy dog.\n", tag.Message)
	}
}
