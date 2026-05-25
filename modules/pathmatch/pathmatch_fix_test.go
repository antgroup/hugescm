package pathmatch

import "testing"

// TestEmbeddedDoubleStarFixes tests the fixes for embedded ** matching logic
func TestEmbeddedDoubleStarFixes(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		// Embedded ** should match across segments
		{
			name:    "foo**bar matches foo/baz/bar",
			pattern: "foo**bar",
			path:    "foo/baz/bar",
			want:    true,
		},
		{
			name:    "foo**bar matches foobazbar",
			pattern: "foo**bar",
			path:    "foobazbar",
			want:    true,
		},
		{
			name:    "foo**bar matches foo/bar",
			pattern: "foo**bar",
			path:    "foo/bar",
			want:    true,
		},
		{
			name:    "foo**bar matches foobar",
			pattern: "foo**bar",
			path:    "foobar",
			want:    true,
		},
		{
			name:    "**bar matches foo/bar",
			pattern: "**bar",
			path:    "foo/bar",
			want:    true,
		},
		{
			name:    "**bar matches bar",
			pattern: "**bar",
			path:    "bar",
			want:    true,
		},
		{
			name:    "foo** matches foo/bar",
			pattern: "foo**",
			path:    "foo/bar",
			want:    true,
		},
		{
			name:    "foo** matches foo",
			pattern: "foo**",
			path:    "foo",
			want:    true,
		},
		{
			name:    "**/bar** matches foo/bar/baz",
			pattern: "**/bar**",
			path:    "foo/bar/baz",
			want:    true,
		},
		{
			name:    "**/bar** matches foo/bar",
			pattern: "**/bar**",
			path:    "foo/bar",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.pattern)
			got := p.Match(tt.path)
			if got != tt.want {
				t.Errorf("Pattern(%q).Match(%q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

// TestCharacterClassBoundaries tests character class edge cases
func TestCharacterClassBoundaries(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		// ] at first position should be treated as literal
		{
			name:    "[]abc] matches ]",
			pattern: "[]abc]",
			path:    "]",
			want:    true,
		},
		{
			name:    "[]abc] matches a",
			pattern: "[]abc]",
			path:    "a",
			want:    true,
		},
		{
			name:    "[]abc] matches b",
			pattern: "[]abc]",
			path:    "b",
			want:    true,
		},
		{
			name:    "[]abc] matches c",
			pattern: "[]abc]",
			path:    "c",
			want:    true,
		},
		{
			name:    "[]abc] does not match d",
			pattern: "[]abc]",
			path:    "d",
			want:    false,
		},
		// - at start or end should be treated as literal
		{
			name:    "[-abc] matches -",
			pattern: "[-abc]",
			path:    "-",
			want:    true,
		},
		{
			name:    "[abc-] matches -",
			pattern: "[abc-]",
			path:    "-",
			want:    true,
		},
		{
			name:    "[-abc] matches a",
			pattern: "[-abc]",
			path:    "a",
			want:    true,
		},
		{
			name:    "[abc-] matches c",
			pattern: "[abc-]",
			path:    "c",
			want:    true,
		},
		// Negated empty set [!] should match any character
		{
			name:    "[!] matches a",
			pattern: "[!]",
			path:    "a",
			want:    true,
		},
		{
			name:    "[!] matches ]",
			pattern: "[!]",
			path:    "]",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.pattern)
			got := p.Match(tt.path)
			if got != tt.want {
				t.Errorf("Pattern(%q).Match(%q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

// TestEscapeHandling tests escape character handling
func TestEscapeHandling(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{
			name:    `\* escapes asterisk`,
			pattern: `foo\*bar`,
			path:    "foo*bar",
			want:    true,
		},
		{
			name:    `\* does not match literal text`,
			pattern: `foo\*bar`,
			path:    "foobar",
			want:    false,
		},
		{
			name:    `\? escapes question mark`,
			pattern: `\?\?`,
			path:    "??",
			want:    true,
		},
		{
			name:    `\[ escapes bracket`,
			pattern: `\[abc\]`,
			path:    "[abc]",
			want:    true,
		},
		{
			name:    `\\ escapes backslash`,
			pattern: `foo\\bar`,
			path:    `foo\bar`,
			want:    true,
		},
		{
			name:    `\a escapes to literal a`,
			pattern: `\a\b\c`,
			path:    "abc",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New(tt.pattern)
			got := p.Match(tt.path)
			if got != tt.want {
				t.Errorf("Pattern(%q).Match(%q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}
