package diferenco

import (
	"testing"
	"unicode"
)

func TestIsCJK(t *testing.T) {
	tests := []struct {
		r      rune
		expect bool
		desc   string
	}{
		// Chinese characters
		{'中', true, "Chinese character 中"},
		{'文', true, "Chinese character 文"},
		{'你', true, "Chinese character 你"},
		{'好', true, "Chinese character 好"},
		{'龙', true, "Chinese character 龙"},
		{0x4E00, true, "First CJK Unified Ideograph"},
		{0x9FFF, true, "Near end of CJK Unified Ideographs"},

		// Japanese Hiragana
		{'あ', true, "Hiragana あ"},
		{'い', true, "Hiragana い"},
		{0x3041, true, "Hiragana start"},
		{0x309F, true, "Hiragana end"},

		// Japanese Katakana
		{'ア', true, "Katakana ア"},
		{'イ', true, "Katakana イ"},
		{0x30A0, true, "Katakana start"},
		{0x30FF, true, "Katakana end"},

		// Korean Hangul
		{'한', true, "Hangul 한"},
		{'글', true, "Hangul 글"},
		{0xAC00, true, "Hangul start"},
		{0xD7A3, true, "Hangul end"},

		// ASCII - should be false
		{'a', false, "ASCII a"},
		{'Z', false, "ASCII Z"},
		{'0', false, "ASCII 0"},
		{' ', false, "ASCII space"},

		// Punctuation
		{'.', false, "period"},
		{',', false, "comma"},
		{'!', false, "exclamation"},
	}

	for _, tt := range tests {
		got := isCJK(tt.r)
		if got != tt.expect {
			t.Errorf("isCJK(%q U+%04X %s) = %v, want %v", tt.r, tt.r, tt.desc, got, tt.expect)
		}
	}
}

func TestIsEmoji(t *testing.T) {
	tests := []struct {
		r      rune
		expect bool
		desc   string
	}{
		// Common emojis (using hex to avoid encoding issues)
		{0x1F600, true, "Grinning Face"},
		{0x1F389, true, "Party Popper"},
		{0x2764, true, "Heavy Black Heart"},
		{0x1F44D, true, "Thumbs Up"},
		{0x1F31F, true, "Glowing Star"},

		// Emoji numbers and symbols
		{'0', true, "Emoji digit 0"},
		{'9', true, "Emoji digit 9"},
		{'#', true, "Emoji #"},
		{'*', true, "Emoji *"},

		// ASCII letters - not emoji
		{'a', false, "ASCII a"},
		{'Z', false, "ASCII Z"},

		// Chinese characters - not emoji
		{'中', false, "Chinese character"},

		// Variation Selector
		{0xFE0F, true, "Variation Selector-16"},

		// Zero Width Joiner
		{0x200D, true, "Zero Width Joiner"},
	}

	for _, tt := range tests {
		got := isEmoji(tt.r)
		if got != tt.expect {
			t.Errorf("isEmoji(%q U+%04X %s) = %v, want %v", tt.r, tt.r, tt.desc, got, tt.expect)
		}
	}
}

// TestInRangeBinarySearch tests that binary search works correctly
func TestInRangeBinarySearch(t *testing.T) {
	// Test boundary conditions
	// First and last elements
	if !inRange(cjkRanges, 0x1100) {
		t.Error("First CJK range element not found")
	}
	if !inRange(cjkRanges, 0x115F) {
		t.Error("End of first CJK range not found")
	}

	// Elements just outside ranges
	if inRange(cjkRanges, 0x10FF) {
		t.Error("Element before first range incorrectly found")
	}
	if inRange(cjkRanges, 0x1160) {
		t.Error("Element after first range incorrectly found")
	}
}

// TestCJKVsUnicodeLibrary compares our implementation with unicode.In
func TestCJKVsUnicodeLibrary(t *testing.T) {
	// Test a range of characters
	for r := rune(0x4E00); r <= 0x4E50; r++ {
		got := isCJK(r)
		want := unicode.In(r, unicode.Han)
		if got != want {
			t.Errorf("isCJK(U+%04X) = %v, unicode.In = %v", r, got, want)
		}
	}

	// Test Hiragana
	for r := rune(0x3041); r <= 0x3050; r++ {
		got := isCJK(r)
		want := unicode.In(r, unicode.Hiragana)
		if got != want {
			t.Errorf("isCJK(U+%04X) = %v, unicode.In = %v", r, got, want)
		}
	}

	// Test Katakana
	for r := rune(0x30A1); r <= 0x30B0; r++ {
		got := isCJK(r)
		want := unicode.In(r, unicode.Katakana)
		if got != want {
			t.Errorf("isCJK(U+%04X) = %v, unicode.In = %v", r, got, want)
		}
	}

	// Test Hangul
	for r := rune(0xAC00); r <= 0xAC10; r++ {
		got := isCJK(r)
		want := unicode.In(r, unicode.Hangul)
		if got != want {
			t.Errorf("isCJK(U+%04X) = %v, unicode.In = %v", r, got, want)
		}
	}
}
