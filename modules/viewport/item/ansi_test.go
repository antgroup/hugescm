package item

import (
	"regexp"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/viewport/internal"
)

func TestAnsi_reapplyAnsi(t *testing.T) {
	tests := []struct {
		name            string
		original        string
		truncated       string
		truncByteOffset int
		expected        string
	}{
		{
			name:            "no ansi, no offset",
			original:        "1234567890123456789012345",
			truncated:       "12345",
			truncByteOffset: 0,
			expected:        "12345",
		},
		{
			name:            "no ansi, offset",
			original:        "1234567890123456789012345",
			truncated:       "2345",
			truncByteOffset: 1,
			expected:        "2345",
		},
		{
			name:            "multi ansi, no offset",
			original:        "\x1b[38;2;255;0;0m1" + RST + "\x1b[38;2;0;0;255m2" + RST + "\x1b[38;2;255;0;0m3" + RST + "45",
			truncated:       "123",
			truncByteOffset: 0,
			expected:        "\x1b[38;2;255;0;0m1" + RST + "\x1b[38;2;0;0;255m2" + RST + "\x1b[38;2;255;0;0m3" + RST,
		},
		{
			name:            "surrounding ansi, no offset",
			original:        "\x1b[38;2;255;0;0m12345" + RST,
			truncated:       "123",
			truncByteOffset: 0,
			expected:        "\x1b[38;2;255;0;0m123" + RST,
		},
		{
			name:            "surrounding ansi, offset",
			original:        "\x1b[38;2;255;0;0m12345" + RST,
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "\x1b[38;2;255;0;0m234" + RST,
		},
		{
			name:            "left ansi, no offset",
			original:        "\x1b[38;2;255;0;0m1" + RST + "2345",
			truncated:       "123",
			truncByteOffset: 0,
			expected:        "\x1b[38;2;255;0;0m1" + RST + "23",
		},
		{
			name:            "left ansi, offset",
			original:        "\x1b[38;2;255;0;0m12" + RST + "345",
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "\x1b[38;2;255;0;0m2" + RST + "34",
		},
		{
			name:            "right ansi, no offset",
			original:        "1" + "\x1b[38;2;255;0;0m2345" + RST,
			truncated:       "123",
			truncByteOffset: 0,
			expected:        "1" + "\x1b[38;2;255;0;0m23" + RST,
		},
		{
			name:            "right ansi, offset",
			original:        "12" + "\x1b[38;2;255;0;0m345" + RST,
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "2" + "\x1b[38;2;255;0;0m34" + RST,
		},
		{
			name:            "left and right ansi, no offset",
			original:        "\x1b[38;2;255;0;0m1" + RST + "2" + "\x1b[38;2;255;0;0m345" + RST,
			truncated:       "123",
			truncByteOffset: 0,
			expected:        "\x1b[38;2;255;0;0m1" + RST + "2" + "\x1b[38;2;255;0;0m3" + RST,
		},
		{
			name:            "left and right ansi, offset",
			original:        "\x1b[38;2;255;0;0m12" + RST + "3" + "\x1b[38;2;255;0;0m45" + RST,
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "\x1b[38;2;255;0;0m2" + RST + "3" + "\x1b[38;2;255;0;0m4" + RST,
		},
		{
			name:            "truncated right ansi, no offset",
			original:        "\x1b[38;2;255;0;0m1" + RST + "234" + "\x1b[38;2;255;0;0m5" + RST,
			truncated:       "123",
			truncByteOffset: 0,
			expected:        "\x1b[38;2;255;0;0m1" + RST + "23",
		},
		{
			name:            "truncated right ansi, offset",
			original:        "\x1b[38;2;255;0;0m12" + RST + "34" + "\x1b[38;2;255;0;0m5" + RST,
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "\x1b[38;2;255;0;0m2" + RST + "34",
		},
		{
			name:            "truncated left ansi, offset",
			original:        "\x1b[38;2;255;0;0m1" + RST + "23" + "\x1b[38;2;255;0;0m45" + RST,
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "23" + "\x1b[38;2;255;0;0m4" + RST,
		},
		{
			name:            "nested color sequences",
			original:        "\x1b[31m1\x1b[32m2\x1b[33m3" + RST + RST + RST + "45",
			truncated:       "123",
			truncByteOffset: 0,
			expected:        "\x1b[31m1\x1b[32m2\x1b[33m3" + RST,
		},
		{
			name:            "nested color sequences with offset",
			original:        "\x1b[31m1\x1b[32m2\x1b[33m3" + RST + RST + RST + "45",
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "\x1b[31m\x1b[32m2\x1b[33m3" + RST + "4",
		},
		{
			name:            "nested style sequences",
			original:        "\x1b[1m1\x1b[4m2\x1b[3m3" + RST + RST + RST + "45",
			truncated:       "123",
			truncByteOffset: 0,
			expected:        "\x1b[1m1\x1b[4m2\x1b[3m3" + RST,
		},
		{
			name:            "mixed nested sequences",
			original:        "\x1b[31m1\x1b[1m2\x1b[4;32m3" + RST + RST + RST + "45",
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "\x1b[31m\x1b[1m2\x1b[4;32m3" + RST + "4",
		},
		{
			name:            "deeply nested sequences",
			original:        "\x1b[31m1\x1b[1m2\x1b[4m3\x1b[32m4" + RST + RST + RST + RST + "5",
			truncated:       "123",
			truncByteOffset: 0,
			expected:        "\x1b[31m1\x1b[1m2\x1b[4m3" + RST,
		},
		{
			name:            "partial nested sequences",
			original:        "1\x1b[31m2\x1b[1m3\x1b[4m4" + RST + RST + RST + "5",
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "\x1b[31m2\x1b[1m3\x1b[4m4" + RST,
		},
		{
			name:            "overlapping nested sequences",
			original:        "\x1b[31m1\x1b[1m2" + RST + "3\x1b[4m4" + RST + "5",
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "\x1b[31m\x1b[1m2" + RST + "3\x1b[4m4" + RST,
		},
		{
			name:            "complex RGB nested sequences",
			original:        "\x1b[38;2;255;0;0m1\x1b[1m2\x1b[38;2;0;255;0m3" + RST + RST + "45",
			truncated:       "123",
			truncByteOffset: 0,
			expected:        "\x1b[38;2;255;0;0m1\x1b[1m2\x1b[38;2;0;255;0m3" + RST,
		},
		{
			name:            "nested sequences with background colors",
			original:        "\x1b[31;44m1\x1b[1m2\x1b[32;45m3" + RST + RST + "45",
			truncated:       "234",
			truncByteOffset: 1,
			expected:        "\x1b[31;44m\x1b[1m2\x1b[32;45m3" + RST + "4",
		},
		{
			name:            "emoji basic",
			original:        "1️⃣2️⃣3️⃣4️⃣5️⃣",
			truncated:       "1️⃣2️⃣3️⃣",
			truncByteOffset: 0,
			expected:        "1️⃣2️⃣3️⃣",
		},
		{
			name:            "emoji with ansi",
			original:        "\x1b[31m1️⃣\x1b[32m2️⃣\x1b[33m3️⃣" + RST,
			truncated:       "1️⃣2️⃣",
			truncByteOffset: 0,
			expected:        "\x1b[31m1️⃣\x1b[32m2️⃣" + RST,
		},
		{
			name:            "chinese characters",
			original:        "你好世界星星",
			truncated:       "你好世",
			truncByteOffset: 0,
			expected:        "你好世",
		},
		{
			name:            "simple with ansi and offset",
			original:        "\x1b[31ma\x1b[32mb\x1b[33mc" + RST + "de",
			truncated:       "bcd",
			truncByteOffset: 1,
			expected:        "\x1b[31m\x1b[32mb\x1b[33mc" + RST + "d",
		},
		{
			name:            "chinese with ansi and offset",
			original:        "\x1b[31m你\x1b[32m好\x1b[33m世" + RST + "界星",
			truncated:       "好世界",
			truncByteOffset: 3, // 你 is 3 bytes
			expected:        "\x1b[31m\x1b[32m好\x1b[33m世" + RST + "界",
		},
		{
			name:            "lots of leading empty ansi",
			original:        "\x1b[38;2;255;0;0mr" + RST + "\x1b[38;2;255;0;0mr" + RST + "\x1b[38;2;255;0;0mr" + RST + "\x1b[38;2;255;0;0mr" + RST + "\x1b[38;2;255;0;0mr" + RST + "\x1b[38;2;255;0;0mr" + RST + "\x1b[38;2;255;0;0mr" + RST + "\x1b[38;2;255;0;0mr" + RST + "\x1b[38;2;255;0;0mr" + RST + "\x1b[38;2;255;0;0mr" + RST + "\x1b[38;2;255;0;0mr" + RST,
			truncated:       "r",
			truncByteOffset: 10,
			expected:        "\x1b[38;2;255;0;0mr" + RST,
		},
		{
			name:            "complex ansi, no offset",
			original:        "\x1b[38;2;0;0;255msome " + RST + "\x1b[38;2;255;0;0mred" + RST + "\x1b[38;2;0;0;255m t" + RST,
			truncated:       "some red t",
			truncByteOffset: 0,
			expected:        "\x1b[38;2;0;0;255msome " + RST + "\x1b[38;2;255;0;0mred" + RST + "\x1b[38;2;0;0;255m t" + RST,
		},
		{
			name:            "unicode with ansi",
			original:        internal.RedBg.Render("A💖") + "中é",
			truncated:       "A💖中é",
			truncByteOffset: 0,
			expected:        internal.RedBg.Render("A💖") + "中é",
		},
	}

	ansiRegex := regexp.MustCompile("\x1b\\[[0-9;]*m")

	toUInt32 := func(indexes [][]int) [][]uint32 {
		uint32Indexes := make([][]uint32, len(indexes))
		for i, idx := range indexes {
			uint32Indexes[i] = []uint32{clampIntToUint32(idx[0]), clampIntToUint32(idx[1])}
		}
		return uint32Indexes
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ansiCodeIndexes := toUInt32(ansiRegex.FindAllStringIndex(tt.original, -1))
			actual := reapplyAnsi(tt.original, tt.truncated, tt.truncByteOffset, ansiCodeIndexes)
			internal.CmpStr(t, tt.expected, actual)
		})
	}
}

func TestHighlightString(t *testing.T) {
	for _, tt := range []struct {
		name                      string
		plainLine                 string // used for extracting highlights
		styledSegment             string // line segment with ANSI codes
		toHighlight               string // unstyled text to highlight in segment
		highlightStyle            lipgloss.Style
		plainLineSegmentStartByte int // byte offset in plainLine where segment starts
		plainLineSegmentEndByte   int // byte offset in plainLine where segment ends
		expected                  string
	}{
		{
			name:                      "empty",
			plainLine:                 "",
			styledSegment:             "",
			toHighlight:               "",
			highlightStyle:            internal.RedFg,
			plainLineSegmentStartByte: 0,
			plainLineSegmentEndByte:   0,
			expected:                  "",
		},
		{
			name:                      "no highlight",
			plainLine:                 "hello",
			styledSegment:             "hello",
			toHighlight:               "",
			highlightStyle:            internal.RedFg,
			plainLineSegmentStartByte: 0,
			plainLineSegmentEndByte:   5,
			expected:                  "hello",
		},
		{
			name:                      "simple highlight",
			plainLine:                 "hello",
			styledSegment:             "hello",
			toHighlight:               "ell",
			highlightStyle:            internal.RedFg,
			plainLineSegmentStartByte: 0,
			plainLineSegmentEndByte:   5,
			expected:                  "h" + internal.RedFg.Render("ell") + "o",
		},
		{
			name:                      "highlight with existing style",
			plainLine:                 "first line",
			styledSegment:             internal.RedFg.Render("first line"),
			toHighlight:               "first",
			highlightStyle:            internal.BlueFg,
			plainLineSegmentStartByte: 0,
			plainLineSegmentEndByte:   10,
			expected:                  internal.BlueFg.Render("first") + internal.RedFg.Render(" line"),
		},
		{
			name:                      "left overflow",
			plainLine:                 "hello world",
			styledSegment:             "ello world",
			toHighlight:               "hello",
			highlightStyle:            internal.RedFg,
			plainLineSegmentStartByte: 1,
			plainLineSegmentEndByte:   11,
			expected:                  internal.RedFg.Render("ello") + " world",
		},
		{
			name:                      "right overflow",
			plainLine:                 "hello world",
			styledSegment:             "hello wo",
			toHighlight:               "world",
			highlightStyle:            internal.RedFg,
			plainLineSegmentStartByte: 0,
			plainLineSegmentEndByte:   8,
			expected:                  "hello " + internal.RedFg.Render("wo"),
		},
		{
			name:                      "both overflow with existing style",
			plainLine:                 "hello world",
			styledSegment:             internal.RedFg.Render("ello wor"),
			toHighlight:               "hello world",
			highlightStyle:            internal.BlueFg,
			plainLineSegmentStartByte: 1,
			plainLineSegmentEndByte:   9,
			expected:                  internal.BlueFg.Render("ello wor"),
		},
		{
			name:                      "no match in segment",
			plainLine:                 "outside middle outside",
			styledSegment:             "middle",
			toHighlight:               "outside",
			highlightStyle:            internal.RedFg,
			plainLineSegmentStartByte: 8,
			plainLineSegmentEndByte:   14,
			expected:                  "middle",
		},
		{
			name:                      "across ansi styles",
			plainLine:                 "hello world",
			styledSegment:             internal.RedBg.Render("hello") + " " + internal.BlueBg.Render("world"),
			toHighlight:               "lo wo",
			highlightStyle:            internal.GreenBg,
			plainLineSegmentStartByte: 0,
			plainLineSegmentEndByte:   11,
			expected:                  internal.RedBg.Render("hel") + internal.GreenBg.Render("lo wo") + internal.BlueBg.Render("rld"),
		},
		{
			name: "unicode",
			// A (1w, 1b), 💖 (2w, 4b), 中 (2w, 3b), é (1w, 3b), A (1w, 1b)
			plainLine:                 "A💖中éA",
			styledSegment:             internal.RedFg.Render("💖中éA"),
			toHighlight:               "💖中",
			highlightStyle:            internal.GreenBg,
			plainLineSegmentStartByte: 1,
			plainLineSegmentEndByte:   12,
			expected:                  internal.GreenBg.Render("💖中") + internal.RedFg.Render("éA"),
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			matches := NewItem(tt.plainLine).ExtractExactMatches(tt.toHighlight)
			highlights := toHighlights(matches, tt.highlightStyle)
			result := highlightString(
				tt.styledSegment,
				highlights,
				tt.plainLineSegmentStartByte,
				tt.plainLineSegmentEndByte,
			)
			internal.CmpStr(t, tt.expected, result)
		})
	}
}

func TestAnsi_getNonAnsiBytes(t *testing.T) {
	tests := []struct {
		name         string
		s            string
		startByteIdx int
		numBytes     int
		expected     string
		shouldPanic  bool
	}{
		{
			name:         "empty",
			s:            "",
			startByteIdx: 0,
			numBytes:     0,
			expected:     "",
		},
		{
			name:         "negative start panics",
			s:            "a",
			startByteIdx: -1,
			numBytes:     1,
			shouldPanic:  true,
		},
		{
			name:         "zero bytes",
			s:            "abc",
			startByteIdx: 1,
			numBytes:     0,
			expected:     "",
		},
		{
			name:         "negative bytes",
			s:            "abc",
			startByteIdx: 1,
			numBytes:     -1,
			expected:     "",
		},
		{
			name:         "all from start",
			s:            "abc",
			startByteIdx: 0,
			numBytes:     3,
			expected:     "abc",
		},
		{
			name:         "some from start",
			s:            "abc",
			startByteIdx: 0,
			numBytes:     2,
			expected:     "ab",
		},
		{
			name:         "rest from offset",
			s:            "abc",
			startByteIdx: 1,
			numBytes:     2,
			expected:     "bc",
		},
		{
			name:         "some from offset",
			s:            "abc",
			startByteIdx: 1,
			numBytes:     1,
			expected:     "b",
		},
		{
			name:         "ignore ansi",
			s:            "abc" + internal.RedBg.Render("def") + "ghi",
			startByteIdx: 1,
			numBytes:     7,
			expected:     "bcdefgh",
		},
		{
			name: "unicode",
			// A (1w, 1b), 💖 (2w, 4b), 中 (2w, 3b), é (1w, 3b)
			s:            "A💖中é",
			startByteIdx: 1,
			numBytes:     7,
			expected:     "💖中",
		},
		{
			name: "unicode with ansi",
			// A (1w, 1b), 💖 (2w, 4b), 中 (2w, 3b), é (1w, 3b)
			s:            "A💖" + internal.RedBg.Render("中") + "é",
			startByteIdx: 0,
			numBytes:     11,
			expected:     "A💖中é",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.shouldPanic {
				assertPanic(t, func() {
					getNonAnsiBytes(tt.s, tt.startByteIdx, tt.numBytes)
				})
				return
			}

			if r := getNonAnsiBytes(tt.s, tt.startByteIdx, tt.numBytes); r != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, r)
			}
		})
	}
}

func TestStripNonSGR(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no escape sequences",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "sgr only preserved",
			input:    "\x1b[31mhello\x1b[m",
			expected: "\x1b[31mhello\x1b[m",
		},
		{
			name:     "complex sgr preserved",
			input:    "\x1b[38;2;255;0;0mhi\x1b[m",
			expected: "\x1b[38;2;255;0;0mhi\x1b[m",
		},
		{
			name:     "cursor up stripped",
			input:    "\x1b[Ahello",
			expected: "hello",
		},
		{
			name:     "cursor down stripped",
			input:    "\x1b[Bhello",
			expected: "hello",
		},
		{
			name:     "cursor forward stripped",
			input:    "\x1b[Chello",
			expected: "hello",
		},
		{
			name:     "cursor back stripped",
			input:    "\x1b[Dhello",
			expected: "hello",
		},
		{
			name:     "cursor position stripped",
			input:    "\x1b[10;20Hhello",
			expected: "hello",
		},
		{
			name:     "erase display stripped",
			input:    "\x1b[2Jhello",
			expected: "hello",
		},
		{
			name:     "erase line 1K stripped",
			input:    "\x1b[1Khello",
			expected: "hello",
		},
		{
			name:     "erase line 2K stripped",
			input:    "\x1b[2Khello",
			expected: "hello",
		},
		{
			name:     "scroll up stripped",
			input:    "\x1b[Shello",
			expected: "hello",
		},
		{
			name:     "scroll down stripped",
			input:    "\x1b[Thello",
			expected: "hello",
		},
		{
			name:     "device status stripped",
			input:    "\x1b[6nhello",
			expected: "hello",
		},
		{
			name:     "private marker stripped",
			input:    "\x1b[?25hhello",
			expected: "hello",
		},
		{
			name:     "sgr mixed with csi",
			input:    "\x1b[31m\x1b[2Jhello\x1b[m",
			expected: "\x1b[31mhello\x1b[m",
		},
		{
			name:     "osc bel terminated stripped",
			input:    "\x1b]0;title\x07hello",
			expected: "hello",
		},
		{
			name:     "osc st terminated stripped",
			input:    "\x1b]0;title\x1b\\hello",
			expected: "hello",
		},
		{
			name:     "osc hyperlink stripped",
			input:    "\x1b]8;;https://example.com\x1b\\click\x1b]8;;\x1b\\",
			expected: "click",
		},
		{
			name:     "esc-M reverse index stripped",
			input:    "\x1bMhello",
			expected: "hello",
		},
		{
			name:     "esc-D index stripped",
			input:    "\x1bDhello",
			expected: "hello",
		},
		{
			name:     "esc-7 dec save cursor stripped",
			input:    "\x1b7hello",
			expected: "hello",
		},
		{
			name:     "esc-8 dec restore cursor stripped",
			input:    "\x1b8hello",
			expected: "hello",
		},
		{
			name:     "ss2 stripped",
			input:    "\x1bNA hello",
			expected: " hello",
		},
		{
			name:     "ss3 stripped",
			input:    "\x1bOA hello",
			expected: " hello",
		},
		{
			name:     "nf designate charset stripped",
			input:    "\x1b(Bhello",
			expected: "hello",
		},
		{
			name:     "multiple non-sgr stripped",
			input:    "\x1b[31m\x1b[2J\x1b[Hhello\x1b[m",
			expected: "\x1b[31mhello\x1b[m",
		},
		{
			name:     "bare esc at end preserved",
			input:    "hello\x1b",
			expected: "hello\x1b",
		},
		{
			name:     "truncated csi preserved",
			input:    "hello\x1b[",
			expected: "hello\x1b[",
		},
		{
			name:     "truncated csi params preserved",
			input:    "hello\x1b[31",
			expected: "hello\x1b[31",
		},
		{
			name:     "unicode with non-sgr",
			input:    "\x1b[2J世界\x1b[31m星\x1b[m",
			expected: "世界\x1b[31m星\x1b[m",
		},
		{
			name:     "all stripped leaves empty",
			input:    "\x1b[2J\x1b[H",
			expected: "",
		},
		{
			name:     "unterminated osc stripped to end",
			input:    "hello\x1b]0;title",
			expected: "hello",
		},
		{
			name:     "esc followed by lowercase kept",
			input:    "\x1b" + "ahello",
			expected: "\x1b" + "ahello",
		},
		// additional CSI variants
		{
			name:     "csi with intermediate bytes stripped",
			input:    "\x1b[ q hello",
			expected: " hello",
		},
		{
			name:     "csi insert line stripped",
			input:    "\x1b[3Lhello",
			expected: "hello",
		},
		{
			name:     "csi delete line stripped",
			input:    "\x1b[3Mhello",
			expected: "hello",
		},
		{
			name:     "csi delete char stripped",
			input:    "\x1b[Phello",
			expected: "hello",
		},
		{
			name:     "csi erase char stripped",
			input:    "\x1b[Xhello",
			expected: "hello",
		},
		{
			name:     "csi set mode stripped",
			input:    "\x1b[4hhello",
			expected: "hello",
		},
		{
			name:     "csi reset mode stripped",
			input:    "\x1b[4lhello",
			expected: "hello",
		},
		{
			name:     "csi cursor save stripped",
			input:    "\x1b[shello",
			expected: "hello",
		},
		{
			name:     "csi cursor restore stripped",
			input:    "\x1b[uhello",
			expected: "hello",
		},
		{
			name:     "csi sgr with intermediate preserved",
			input:    "\x1b[1;31mhello\x1b[m",
			expected: "\x1b[1;31mhello\x1b[m",
		},
		{
			name:     "csi 256-color sgr preserved",
			input:    "\x1b[38;5;196mhello\x1b[m",
			expected: "\x1b[38;5;196mhello\x1b[m",
		},
		{
			name:     "sgr reset 0m preserved",
			input:    "\x1b[0mhello",
			expected: "\x1b[0mhello",
		},
		{
			name:     "sgr bare m preserved",
			input:    "\x1b[mhello",
			expected: "\x1b[mhello",
		},
		// more OSC variants
		{
			name:     "osc with numeric param and bel",
			input:    "\x1b]2;my window\x07text",
			expected: "text",
		},
		{
			name:     "osc empty payload bel terminated",
			input:    "\x1b]\x07text",
			expected: "text",
		},
		{
			name:     "osc with embedded semicolons",
			input:    "\x1b]8;id=link;https://example.com\x1b\\click here\x1b]8;;\x1b\\",
			expected: "click here",
		},
		{
			name:     "osc between text",
			input:    "before\x1b]0;title\x07after",
			expected: "beforeafter",
		},
		// more Fe sequences
		{
			name:     "esc-E next line stripped",
			input:    "\x1bEhello",
			expected: "hello",
		},
		{
			name:     "esc-H set tab stop stripped",
			input:    "\x1bHhello",
			expected: "hello",
		},
		{
			name:     "esc-P (DCS) stripped as Fe",
			input:    "\x1bPhello",
			expected: "hello",
		},
		// more Fp sequences
		{
			name:     "esc-= keypad application mode stripped",
			input:    "\x1b=hello",
			expected: "hello",
		},
		{
			name:     "esc-> keypad numeric mode stripped",
			input:    "\x1b>hello",
			expected: "hello",
		},
		// nF variants
		{
			name:     "nf G0 designate stripped",
			input:    "\x1b(0hello",
			expected: "hello",
		},
		{
			name:     "nf G1 designate stripped",
			input:    "\x1b)Bhello",
			expected: "hello",
		},
		{
			name:     "nf multi-intermediate stripped",
			input:    "\x1b$ Bhello",
			expected: "hello",
		},
		{
			name:     "nf truncated at end stripped",
			input:    "hello\x1b(",
			expected: "hello",
		},
		// SS2/SS3 edge cases
		{
			name:     "ss2 at end of string",
			input:    "hello\x1bN",
			expected: "hello",
		},
		{
			name:     "ss3 at end of string",
			input:    "hello\x1bO",
			expected: "hello",
		},
		// complex mixed sequences
		{
			name:     "sgr surrounded by many non-sgr",
			input:    "\x1b[2J\x1b[H\x1b7\x1b[31mhello\x1b[m\x1b8\x1b[?25h",
			expected: "\x1b[31mhello\x1b[m",
		},
		{
			name:     "alternating sgr and non-sgr",
			input:    "\x1b[1mA\x1b[HB\x1b[32mC\x1b[2JD\x1b[m",
			expected: "\x1b[1mAB\x1b[32mCD\x1b[m",
		},
		{
			name:     "non-sgr between text preserves all text",
			input:    "one\x1b[Htwo\x1b[2Jthree",
			expected: "onetwothree",
		},
		{
			name:     "consecutive non-sgr sequences stripped",
			input:    "\x1b[A\x1b[B\x1b[C\x1b[Dhello",
			expected: "hello",
		},
		{
			name:     "sgr only no alloc returns same string",
			input:    "\x1b[1m\x1b[31m\x1b[42mhello\x1b[m",
			expected: "\x1b[1m\x1b[31m\x1b[42mhello\x1b[m",
		},
		// unicode and wide chars
		{
			name:     "emoji with non-sgr stripped",
			input:    "\x1b[H🎉\x1b[31m🌍\x1b[m",
			expected: "🎉\x1b[31m🌍\x1b[m",
		},
		{
			name:     "cjk wide chars with non-sgr stripped",
			input:    "\x1b[2J你好\x1b[31m世界\x1b[m",
			expected: "你好\x1b[31m世界\x1b[m",
		},
		// malformed sequences
		{
			name:     "csi with invalid final byte kept as text",
			input:    "hello\x1b[\x10world",
			expected: "hello\x1b[\x10world",
		},
		{
			name:     "multiple bare esc preserved",
			input:    "\x1b\x1b\x1bhello",
			expected: "\x1b\x1b\x1bhello",
		},
		{
			name:     "esc followed by space is nf stripped",
			input:    "\x1b Fhello",
			expected: "hello",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			internal.CmpStr(t, tt.expected, stripNonSGR(tt.input))
		})
	}
}

// testing helper
func assertPanic(t *testing.T, f func()) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("did not panic as expected")
		}
	}()
	f()
}
