package git

import (
	"strings"
	"testing"
)

// TestCommitDecodeWithMultipleParents tests decoding with multiple parents
func TestCommitDecodeWithMultipleParents(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
parent a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2
parent b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3
parent c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700

test message`
	commit := new(Commit)
	err := commit.Decode("test", strings.NewReader(input), int64(len(input)))
	if err != nil {
		t.Fatalf("Decode error: %v", err)
	}
	if len(commit.Parents) != 3 {
		t.Errorf("Expected 3 parents, got %d", len(commit.Parents))
	}
}

// TestCommitDecodeWithSpecialCharacters tests decoding with special characters
func TestCommitDecodeWithSpecialCharacters(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
author 张三 <zhangsan@example.com> 1337892984 +0800
committer 张三 <zhangsan@example.com> 1337892984 +0800
custom value with spaces & special!@#$%^&*()_+-=[]{}|;':",./<>?

test message with 中文 and 日本語`

	commit := new(Commit)
	err := commit.Decode("test", strings.NewReader(input), int64(len(input)))
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if !strings.Contains(commit.Author.String(), "张三") {
		t.Errorf("Expected to contain '张三' in author")
	}
	if len(commit.ExtraHeaders) != 1 {
		t.Errorf("Expected %v, got %v", 1, len(commit.ExtraHeaders))
	}
	if commit.ExtraHeaders[0].K != "custom" {
		t.Errorf("Expected %v, got %v", "custom", commit.ExtraHeaders[0].K)
	}
	if commit.ExtraHeaders[0].V != "value with spaces & special!@#$%^&*()_+-=[]{}|;':\",./<>?" {
		t.Errorf("Expected %v, got %v", "value with spaces & special!@#$%^&*()_+-=[]{}|;':\",./<>?", commit.ExtraHeaders[0].V)
	}
	if !strings.Contains(commit.Message, "中文") {
		t.Errorf("Expected message to contain '中文'")
	}
	if !strings.Contains(commit.Message, "日本語") {
		t.Errorf("Expected message to contain '日本語'")
	}
}

// TestCommitDecodeWithExtraHeaderBeforeStandard tests extra header before standard headers
func TestCommitDecodeWithExtraHeaderBeforeStandard(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
custom extra header before standard
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700

test message`
	commit := new(Commit)
	err := commit.Decode("test", strings.NewReader(input), int64(len(input)))
	if err != nil {
		t.Fatalf("Error: %v", err)
	}
	if len(commit.ExtraHeaders) != 1 {
		t.Errorf("Expected %v, got %v", 1, len(commit.ExtraHeaders))
	}
	if commit.ExtraHeaders[0].K != "custom" {
		t.Errorf("Expected %v, got %v", "custom", commit.ExtraHeaders[0].K)
	}
	if commit.ExtraHeaders[0].V != "extra header before standard" {
		t.Errorf("Expected %v, got %v", "extra header before standard", commit.ExtraHeaders[0].V)
	}
}

// TestCommitDecodeWithComplexHeaders tests complex multi-line headers
func TestCommitDecodeWithComplexHeaders(t *testing.T) {
	input := `tree e8ad84c41c2acde27c77fa212b8865cd3acfe6fb
parent b343c8beec664ef6f0e9964d3001c7c7966331ae
author Pat Doe <pdoe@example.org> 1337892984 -0700
committer Pat Doe <pdoe@example.org> 1337892984 -0700
mergetag object 1e8a52e18cfb381bc9cc1f0b720540364d2a6edd
 type commit
 tag random
 tagger J. Roe <jroe@example.ca> 1337889148 -0600

Random changes`

	commit := new(Commit)
	err := commit.Decode("test", strings.NewReader(input), int64(len(input)))
	if err != nil {
		t.Fatalf("Error: %v", err)
	}

	// Verify ExtraHeaders
	if len(commit.ExtraHeaders) != 1 {
		t.Fatalf("Expected %v, got %v", 1, len(commit.ExtraHeaders))
	}
	if commit.ExtraHeaders[0].K != "mergetag" {
		t.Fatalf("Expected %v, got %v", "mergetag", commit.ExtraHeaders[0].K)
	}
	if !strings.Contains(commit.ExtraHeaders[0].V, "object 1e8a52e18cfb381bc9cc1f0b720540364d2a6edd") {
		t.Errorf("Expected to contain 'object 1e8a52e18cfb381bc9cc1f0b720540364d2a6edd'")
	}
	if !strings.Contains(commit.ExtraHeaders[0].V, "type commit") {
		t.Errorf("Expected to contain 'type commit'")
	}
	if !strings.Contains(commit.ExtraHeaders[0].V, "tag random") {
		t.Errorf("Expected to contain 'tag random'")
	}
	if !strings.Contains(commit.ExtraHeaders[0].V, "tagger J. Roe <jroe@example.ca> 1337889148 -0600") {
		t.Errorf("Expected to contain 'tagger J. Roe <jroe@example.ca> 1337889148 -0600'")
	}
}
