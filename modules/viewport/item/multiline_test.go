package item

import (
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func TestMultiLineItem_Width(t *testing.T) {
	tests := []struct {
		name     string
		items    []SingleItem
		expected int
	}{
		{
			name:     "empty",
			items:    nil,
			expected: 0,
		},
		{
			name:     "single item",
			items:    []SingleItem{NewItem("hello")},
			expected: 5,
		},
		{
			name:     "two items",
			items:    []SingleItem{NewItem("hello"), NewItem("world")},
			expected: 10,
		},
		{
			name:     "item with empty line",
			items:    []SingleItem{NewItem("hello"), NewItem(""), NewItem("world")},
			expected: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMultiLineItem(tt.items...)
			if actual := m.Width(); actual != tt.expected {
				t.Errorf("expected width %d, got %d", tt.expected, actual)
			}
		})
	}
}

func TestMultiLineItem_Content(t *testing.T) {
	tests := []struct {
		name     string
		items    []SingleItem
		expected string
	}{
		{
			name:     "empty",
			items:    nil,
			expected: "",
		},
		{
			name:     "single item",
			items:    []SingleItem{NewItem("hello")},
			expected: "hello",
		},
		{
			name:     "two items joined with newline",
			items:    []SingleItem{NewItem("hello"), NewItem("world")},
			expected: "hello\nworld",
		},
		{
			name:     "three items with empty middle",
			items:    []SingleItem{NewItem("a"), NewItem(""), NewItem("b")},
			expected: "a\n\nb",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMultiLineItem(tt.items...)
			if actual := m.Content(); actual != tt.expected {
				t.Errorf("expected content %q, got %q", tt.expected, actual)
			}
			if actual := m.ContentNoAnsi(); actual != tt.expected {
				t.Errorf("expected contentNoAnsi %q, got %q", tt.expected, actual)
			}
		})
	}
}

func TestMultiLineItem_NumWrappedLines(t *testing.T) {
	tests := []struct {
		name      string
		items     []SingleItem
		wrapWidth int
		expected  int
	}{
		{
			name:      "empty items",
			items:     nil,
			wrapWidth: 10,
			expected:  1,
		},
		{
			name:      "single short item",
			items:     []SingleItem{NewItem("hello")},
			wrapWidth: 10,
			expected:  1,
		},
		{
			name:      "single item wraps",
			items:     []SingleItem{NewItem("hello world")},
			wrapWidth: 5,
			expected:  3,
		},
		{
			name:      "two items no wrapping",
			items:     []SingleItem{NewItem("hello"), NewItem("world")},
			wrapWidth: 10,
			expected:  2,
		},
		{
			name:      "two items both wrap",
			items:     []SingleItem{NewItem("hello world"), NewItem("foo bar baz")},
			wrapWidth: 5,
			expected:  6, // 3 + 3
		},
		{
			name:      "item with empty line",
			items:     []SingleItem{NewItem("hello"), NewItem(""), NewItem("world")},
			wrapWidth: 10,
			expected:  3, // 1 + 1 (empty) + 1
		},
		{
			name:      "zero wrap width",
			items:     []SingleItem{NewItem("hello")},
			wrapWidth: 0,
			expected:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMultiLineItem(tt.items...)
			if actual := m.NumWrappedLines(tt.wrapWidth); actual != tt.expected {
				t.Errorf("expected %d wrapped lines, got %d", tt.expected, actual)
			}
		})
	}
}

func TestMultiLineItem_LineBrokenItems(t *testing.T) {
	items := []SingleItem{NewItem("hello"), NewItem("world")}
	m := NewMultiLineItem(items...)
	broken := m.LineBrokenItems()
	if len(broken) != 2 {
		t.Fatalf("expected 2 line-broken items, got %d", len(broken))
	}
	if broken[0].Content() != "hello" {
		t.Errorf("expected first item content 'hello', got %q", broken[0].Content())
	}
	if broken[1].Content() != "world" {
		t.Errorf("expected second item content 'world', got %q", broken[1].Content())
	}
}

func TestMultiLineItem_Take_Panics(t *testing.T) {
	m := NewMultiLineItem(NewItem("hello"), NewItem("world"))
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected Take() to panic on MultiLineItem, but it didn't")
		}
	}()
	m.Take(0, 10, "", nil)
}

func TestMultiLineItem_ExtractExactMatches(t *testing.T) {
	tests := []struct {
		name       string
		items      []SingleItem
		exactMatch string
		expected   []Match
	}{
		{
			name:       "no match",
			items:      []SingleItem{NewItem("hello"), NewItem("world")},
			exactMatch: "xyz",
			expected:   nil,
		},
		{
			name:       "match in first item",
			items:      []SingleItem{NewItem("hello"), NewItem("world")},
			exactMatch: "hello",
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 0, End: 5},
					WidthRange: WidthRange{Start: 0, End: 5},
				},
			},
		},
		{
			name:       "match in second item",
			items:      []SingleItem{NewItem("hello"), NewItem("world")},
			exactMatch: "world",
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 6, End: 11},  // "hello\n" = 6 bytes offset
					WidthRange: WidthRange{Start: 5, End: 10}, // width offset = 5 (width of "hello")
				},
			},
		},
		{
			name:       "match spanning newline",
			items:      []SingleItem{NewItem("hello"), NewItem("world")},
			exactMatch: "o\nw",
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 4, End: 7},
					WidthRange: WidthRange{Start: 4, End: 6}, // "o" width=1 at offset 4, "\n" not counted, "w" at offset 5+0=5, end at 5+1=6
				},
			},
		},
		{
			name:       "empty match",
			items:      []SingleItem{NewItem("hello")},
			exactMatch: "",
			expected:   nil,
		},
		{
			name:       "single item delegates",
			items:      []SingleItem{NewItem("hello world")},
			exactMatch: "world",
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 6, End: 11},
					WidthRange: WidthRange{Start: 6, End: 11},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMultiLineItem(tt.items...)
			actual := m.ExtractExactMatches(tt.exactMatch)
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestMultiLineItem_ExtractRegexMatches(t *testing.T) {
	tests := []struct {
		name     string
		items    []SingleItem
		pattern  string
		expected []Match
	}{
		{
			name:    "simple match",
			items:   []SingleItem{NewItem("hello"), NewItem("world")},
			pattern: "world",
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 6, End: 11},
					WidthRange: WidthRange{Start: 5, End: 10},
				},
			},
		},
		{
			name:    "match in multiple items",
			items:   []SingleItem{NewItem("abc"), NewItem("abcd")},
			pattern: "abc",
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 0, End: 3},
					WidthRange: WidthRange{Start: 0, End: 3},
				},
				{
					ByteRange:  ByteRange{Start: 4, End: 7},
					WidthRange: WidthRange{Start: 3, End: 6},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMultiLineItem(tt.items...)
			actual := m.ExtractRegexMatches(regexp.MustCompile(tt.pattern))
			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

func TestMultiLineItem_Repr(t *testing.T) {
	m := NewMultiLineItem(NewItem("a"), NewItem("b"))
	repr := m.repr()
	if repr != `MultiLine(Item("a"), Item("b"))` {
		t.Errorf("unexpected repr: %s", repr)
	}
}

func TestMultiLineItem_ByteRangesToMatches(t *testing.T) {
	tests := []struct {
		name       string
		items      []SingleItem
		byteRanges []ByteRange
		expected   []Match
	}{
		{
			name:       "nil byte ranges",
			items:      []SingleItem{NewItem("hello"), NewItem("world")},
			byteRanges: nil,
			expected:   nil,
		},
		{
			name:       "empty byte ranges",
			items:      []SingleItem{NewItem("hello"), NewItem("world")},
			byteRanges: []ByteRange{},
			expected:   nil,
		},
		{
			name:       "empty items",
			items:      []SingleItem{},
			byteRanges: []ByteRange{{Start: 0, End: 5}},
			expected:   nil,
		},
		{
			name:  "single item delegates to SingleItem",
			items: []SingleItem{NewItem("hello world")},
			byteRanges: []ByteRange{
				{Start: 6, End: 11},
			},
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 6, End: 11},
					WidthRange: WidthRange{Start: 6, End: 11},
				},
			},
		},
		{
			name:  "range in first item",
			items: []SingleItem{NewItem("hello"), NewItem("world")},
			// ContentNoAnsi = "hello\nworld"
			byteRanges: []ByteRange{
				{Start: 0, End: 5}, // "hello"
			},
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 0, End: 5},
					WidthRange: WidthRange{Start: 0, End: 5},
				},
			},
		},
		{
			name:  "range in second item",
			items: []SingleItem{NewItem("hello"), NewItem("world")},
			// ContentNoAnsi = "hello\nworld", "world" starts at byte 6
			byteRanges: []ByteRange{
				{Start: 6, End: 11}, // "world"
			},
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 6, End: 11},
					WidthRange: WidthRange{Start: 5, End: 10}, // width offset = 5 (width of "hello")
				},
			},
		},
		{
			name:  "range spanning newline",
			items: []SingleItem{NewItem("hello"), NewItem("world")},
			// ContentNoAnsi = "hello\nworld", "o\nw" = bytes 4-7
			byteRanges: []ByteRange{
				{Start: 4, End: 7}, // "o\nw"
			},
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 4, End: 7},
					WidthRange: WidthRange{Start: 4, End: 6}, // "o" ends at width 5, "w" starts at width 5, ends at 6
				},
			},
		},
		{
			name:  "multiple ranges across items",
			items: []SingleItem{NewItem("abc"), NewItem("def")},
			// ContentNoAnsi = "abc\ndef"
			byteRanges: []ByteRange{
				{Start: 0, End: 3}, // "abc"
				{Start: 4, End: 7}, // "def"
			},
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 0, End: 3},
					WidthRange: WidthRange{Start: 0, End: 3},
				},
				{
					ByteRange:  ByteRange{Start: 4, End: 7},
					WidthRange: WidthRange{Start: 3, End: 6},
				},
			},
		},
		{
			name:  "three items with match in middle",
			items: []SingleItem{NewItem("aaa"), NewItem("bbb"), NewItem("ccc")},
			// ContentNoAnsi = "aaa\nbbb\nccc", "bbb" starts at byte 4
			byteRanges: []ByteRange{
				{Start: 4, End: 7}, // "bbb"
			},
			expected: []Match{
				{
					ByteRange:  ByteRange{Start: 4, End: 7},
					WidthRange: WidthRange{Start: 3, End: 6},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMultiLineItem(tt.items...)
			actual := m.ByteRangesToMatches(tt.byteRanges)

			if !reflect.DeepEqual(actual, tt.expected) {
				t.Errorf("expected %v, got %v", tt.expected, actual)
			}
		})
	}
}

// TestMultiLineItem_ByteRangesToMatches_ConsistentWithExtract verifies that
// ByteRangesToMatches and ExtractExactMatches produce the same results.
func TestMultiLineItem_ByteRangesToMatches_ConsistentWithExtract(t *testing.T) {
	tests := []struct {
		name  string
		items []SingleItem
		query string
	}{
		{
			name:  "match in first line",
			items: []SingleItem{NewItem("hello world"), NewItem("foo bar")},
			query: "hello",
		},
		{
			name:  "match in second line",
			items: []SingleItem{NewItem("hello"), NewItem("world")},
			query: "world",
		},
		{
			name:  "match in multiple lines",
			items: []SingleItem{NewItem("abc"), NewItem("abcd")},
			query: "abc",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMultiLineItem(tt.items...)

			// Get matches via ExtractExactMatches
			exactMatches := m.ExtractExactMatches(tt.query)

			// Manually find byte ranges in ContentNoAnsi
			content := m.ContentNoAnsi()
			var byteRanges []ByteRange
			start := 0
			for {
				idx := strings.Index(content[start:], tt.query)
				if idx == -1 {
					break
				}
				actualStart := start + idx
				end := actualStart + len(tt.query)
				byteRanges = append(byteRanges, ByteRange{Start: actualStart, End: end})
				start = end
			}

			// Get matches via ByteRangesToMatches
			brMatches := m.ByteRangesToMatches(byteRanges)

			if !reflect.DeepEqual(exactMatches, brMatches) {
				t.Errorf("ExtractExactMatches=%+v, ByteRangesToMatches=%+v", exactMatches, brMatches)
			}
		})
	}
}
