package fuzzy

import (
	"fmt"
	"testing"
)

func TestMatch(t *testing.T) {
	tests := []struct {
		str            string
		query          string
		matchedIndexes []int // nil means no match expected
	}{
		// Basic ASCII
		{str: "abc", query: "", matchedIndexes: []int{}},
		{str: "abc", query: "a", matchedIndexes: []int{0}},
		{str: "abc", query: "ab", matchedIndexes: []int{0, 1}},
		{str: "abc", query: "ac", matchedIndexes: []int{0, 2}},
		{str: "abc", query: "abc", matchedIndexes: []int{0, 1, 2}},
		{str: "abc", query: "b", matchedIndexes: []int{1}},
		{str: "abc", query: "bc", matchedIndexes: []int{1, 2}},
		{str: "abc", query: "c", matchedIndexes: []int{2}},

		// Non-matches
		{str: "abc", query: "cba"},
		{str: "abc", query: "d"},
		{str: "abc", query: "abcd"},

		// With gaps
		{str: "xaxbxc", query: "a", matchedIndexes: []int{1}},
		{str: "xaxbxc", query: "ab", matchedIndexes: []int{1, 3}},
		{str: "xaxbxc", query: "ac", matchedIndexes: []int{1, 5}},
		{str: "xaxbxc", query: "abc", matchedIndexes: []int{1, 3, 5}},
		{str: "xaxbxc", query: "b", matchedIndexes: []int{3}},
		{str: "xaxbxc", query: "bc", matchedIndexes: []int{3, 5}},
		{str: "xaxbxc", query: "c", matchedIndexes: []int{5}},
		{str: "xaxbxc", query: "cba"},
		{str: "xaxbxc", query: "d"},
		{str: "xaxbxc", query: "abcd"},

		// Unicode
		{str: "こんにちは", query: "こ", matchedIndexes: []int{0}},
		{str: "こんにちは", query: "こん", matchedIndexes: []int{0, 1}},
		{str: "こんにちは", query: "こには", matchedIndexes: []int{0, 2, 4}},
		{str: "こんにちは", query: "こんにちは", matchedIndexes: []int{0, 1, 2, 3, 4}},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("#%d_%s/%s", i, tt.str, tt.query), func(t *testing.T) {
			m, ok := match(tt.str, tt.query, option{})
			if tt.matchedIndexes == nil {
				if ok {
					t.Fatalf("expected no match, got %+v", m)
				}
				return
			}
			if !ok {
				t.Fatalf("expected match with indexes %v, got no match", tt.matchedIndexes)
			}
			if len(m.MatchedIndexes) != len(tt.matchedIndexes) {
				t.Fatalf("expected %d matched indexes, got %d: %v", len(tt.matchedIndexes), len(m.MatchedIndexes), m.MatchedIndexes)
			}
			for j := range tt.matchedIndexes {
				if m.MatchedIndexes[j] != tt.matchedIndexes[j] {
					t.Errorf("index %d: expected %d, got %d", j, tt.matchedIndexes[j], m.MatchedIndexes[j])
				}
			}
			if m.Str != tt.str {
				t.Errorf("expected Str=%q, got %q", tt.str, m.Str)
			}
		})
	}
}

func TestMatchCaseSensitive(t *testing.T) {
	tests := []struct {
		str            string
		query          string
		matchedIndexes []int
	}{
		{str: "abc", query: "abc", matchedIndexes: []int{0, 1, 2}},
		{str: "abc", query: "Abc"},
		{str: "abc", query: "ABC"},
		{str: "Abc", query: "abc"},
		{str: "Abc", query: "Abc", matchedIndexes: []int{0, 1, 2}},
		{str: "Abc", query: "ABC"},
		{str: "ABC", query: "abc"},
		{str: "ABC", query: "Abc"},
		{str: "ABC", query: "ABC", matchedIndexes: []int{0, 1, 2}},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("#%d_%s/%s", i, tt.str, tt.query), func(t *testing.T) {
			m, ok := match(tt.str, tt.query, option{caseSensitive: true})
			if tt.matchedIndexes == nil {
				if ok {
					t.Fatalf("expected no match, got %+v", m)
				}
				return
			}
			if !ok {
				t.Fatalf("expected match, got no match")
			}
			for j := range tt.matchedIndexes {
				if m.MatchedIndexes[j] != tt.matchedIndexes[j] {
					t.Errorf("index %d: expected %d, got %d", j, tt.matchedIndexes[j], m.MatchedIndexes[j])
				}
			}
		})
	}
}

func TestMatchCaseInsensitiveDefault(t *testing.T) {
	// Default (case-insensitive): "Hello" should match query "hello"
	m, ok := match("Hello World", "hello", option{})
	if !ok {
		t.Fatal("expected case-insensitive match")
	}
	expected := []int{0, 1, 2, 3, 4}
	if len(m.MatchedIndexes) != len(expected) {
		t.Fatalf("expected %d indexes, got %d", len(expected), len(m.MatchedIndexes))
	}
	for i, v := range expected {
		if m.MatchedIndexes[i] != v {
			t.Errorf("index %d: expected %d, got %d", i, v, m.MatchedIndexes[i])
		}
	}
}

func TestFind(t *testing.T) {
	items := []string{
		"apple",
		"banana",
		"application",
		"grape",
		"pineapple",
	}

	results := Find(items, "apl")
	// Should match: "apple" (0), "application" (2), "pineapple" (4)
	if len(results) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(results))
	}

	// Results should be sorted: "apple" and "application" have matched indexes
	// [0,2,3] and [0,3,4] so apple comes first; pineapple has [4,6,7]
	if results[0].Str != "apple" {
		t.Errorf("expected first match to be 'apple', got %q", results[0].Str)
	}
}

func TestFindCaseSensitive(t *testing.T) {
	items := []string{"Apple", "apple", "APPLE"}
	results := Find(items, "apple", WithCaseSensitive(true))
	if len(results) != 1 {
		t.Fatalf("expected 1 match, got %d", len(results))
	}
	if results[0].Str != "apple" {
		t.Errorf("expected 'apple', got %q", results[0].Str)
	}
}

func TestFindEmpty(t *testing.T) {
	items := []string{"abc", "def"}
	results := Find(items, "")
	// Empty query matches everything
	if len(results) != 2 {
		t.Fatalf("expected 2 matches for empty query, got %d", len(results))
	}
}

func TestFindNoMatches(t *testing.T) {
	items := []string{"abc", "def"}
	results := Find(items, "xyz")
	if len(results) != 0 {
		t.Fatalf("expected 0 matches, got %d", len(results))
	}
}

func TestMatchesSorting(t *testing.T) {
	items := []string{
		"xxaxxbxxc", // indexes: [2, 5, 8] - spread out
		"abcxxxxxx", // indexes: [0, 1, 2] - tightest, leftmost
		"xabcxxxxx", // indexes: [1, 2, 3] - tight but later
	}
	results := Find(items, "abc")
	if len(results) != 3 {
		t.Fatalf("expected 3 matches, got %d", len(results))
	}

	// Sort order: by matched indexes position (leftmost first)
	// "abcxxxxxx" [0,1,2] < "xabcxxxxx" [1,2,3] < "xxaxxbxxc" [2,5,8]
	if results[0].Str != "abcxxxxxx" {
		t.Errorf("expected first result 'abcxxxxxx', got %q", results[0].Str)
	}
	if results[1].Str != "xabcxxxxx" {
		t.Errorf("expected second result 'xabcxxxxx', got %q", results[1].Str)
	}
	if results[2].Str != "xxaxxbxxc" {
		t.Errorf("expected third result 'xxaxxbxxc', got %q", results[2].Str)
	}
}

func TestMatchedByteRanges(t *testing.T) {
	m := Match{
		Str:            "hello",
		MatchedIndexes: []int{0, 2, 4},
	}
	ranges := m.MatchedByteRanges()
	expected := []ByteRange{
		{Start: 0, End: 1}, // h
		{Start: 2, End: 3}, // l
		{Start: 4, End: 5}, // o
	}
	if len(ranges) != len(expected) {
		t.Fatalf("expected %d ranges, got %d", len(expected), len(ranges))
	}
	for i, r := range ranges {
		if r != expected[i] {
			t.Errorf("range %d: expected %+v, got %+v", i, expected[i], r)
		}
	}
}

func TestMatchedByteRangesUnicode(t *testing.T) {
	// "über" — ü is 2 bytes in UTF-8
	m := Match{
		Str:            "über",
		MatchedIndexes: []int{0, 2}, // ü and e
	}
	ranges := m.MatchedByteRanges()
	expected := []ByteRange{
		{Start: 0, End: 2}, // ü (2 bytes)
		{Start: 3, End: 4}, // e (1 byte, after ü(2) + b(1))
	}
	if len(ranges) != len(expected) {
		t.Fatalf("expected %d ranges, got %d", len(expected), len(ranges))
	}
	for i, r := range ranges {
		if r != expected[i] {
			t.Errorf("range %d: expected %+v, got %+v", i, expected[i], r)
		}
	}
}

func TestMatchTightestSpan(t *testing.T) {
	tests := []struct {
		str            string
		query          string
		matchedIndexes []int
	}{
		// "b" appears early in "foobar", but the tightest match for
		// "bar-baz" is the suffix starting at rune index 7.
		{
			str:            "foobar-bar-baz",
			query:          "bar-baz",
			matchedIndexes: []int{7, 8, 9, 10, 11, 12, 13},
		},
		// Should prefer the later, tighter "a_b" over the early spread "a...b".
		{
			str:            "a____b____a_b",
			query:          "ab",
			matchedIndexes: []int{10, 12},
		},
		// Contiguous match at end preferred over spread match from start.
		{
			str:            "xaxbxcxabc",
			query:          "abc",
			matchedIndexes: []int{7, 8, 9},
		},
	}
	for i, tt := range tests {
		t.Run(fmt.Sprintf("#%d_%s/%s", i, tt.str, tt.query), func(t *testing.T) {
			m, ok := match(tt.str, tt.query, option{})
			if !ok {
				t.Fatalf("expected match, got no match")
			}
			if len(m.MatchedIndexes) != len(tt.matchedIndexes) {
				t.Fatalf("expected %d matched indexes, got %d: %v", len(tt.matchedIndexes), len(m.MatchedIndexes), m.MatchedIndexes)
			}
			for j := range tt.matchedIndexes {
				if m.MatchedIndexes[j] != tt.matchedIndexes[j] {
					t.Errorf("index %d: expected %d, got %d (full: %v)", j, tt.matchedIndexes[j], m.MatchedIndexes[j], m.MatchedIndexes)
				}
			}
		})
	}
}

func TestMatchedByteRangesEmpty(t *testing.T) {
	m := Match{Str: "hello", MatchedIndexes: []int{}}
	ranges := m.MatchedByteRanges()
	if ranges != nil {
		t.Errorf("expected nil for empty MatchedIndexes, got %+v", ranges)
	}
}
