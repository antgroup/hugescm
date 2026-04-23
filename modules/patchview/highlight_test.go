package patchview

import (
	"testing"
)

func TestSanitizeLine(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal text",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "tab replacement",
			input:    "hello\tworld",
			expected: "hello    world", // 4 spaces
		},
		{
			name:     "control characters",
			input:    "hello\x00world",
			expected: "hello\u2400world", // NUL -> ␀
		},
		{
			name:     "DEL character",
			input:    "hello\x7fworld",
			expected: "hello\u2421world", // DEL -> ␡
		},
		{
			name:     "mixed content",
			input:    "\t\x00\x1b",
			expected: "    \u2400\u241b", // tab -> 4 spaces, NUL -> ␀, ESC -> ␛
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeLine(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeLine(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
