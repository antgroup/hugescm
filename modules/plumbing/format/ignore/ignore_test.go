package ignore

import (
	"testing"
)

// ---------------------------------------------------------------------------
// matchPattern unit tests (single-segment matching)
// ---------------------------------------------------------------------------

func TestWildmatchLiterals(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{"foo", "foo", true},
		{"foo", "bar", false},
		{"foo", "fooBar", false},
		{"", "", true},
		{"", "foo", false},
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestWildmatchQuestionMark(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{"?", "x", true},
		{"?", "", false},
		{"???", "foo", true},
		{"??", "foo", false},
		{"f?o", "foo", true},
		{"f?o", "fbo", true},
		{"f?o", "fo", false},
		{"f?o", "fxoo", false},
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestWildmatchStar(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{"*", "foo", true},
		{"*", "", true},
		{"f*", "foo", true},
		{"f*", "f", true},
		{"f*", "barfoo", false},
		{"*f", "foo", false},
		{"*f", "f", true},
		{"*f", "xxf", true},
		{"*foo*", "foo", true},
		{"*foo*", "afoob", true},
		{"*ob*a*r*", "foobar", true},
		{"*ab", "aaaaaaabababab", true},
		{"b*a", "aaabbb", false},
		{"*ba*", "aabcaa", false},
		{"foo*bar", "foobar", true},
		{"foo*bar", "fooXbar", true},
		{"foo*bar", "fooXbarY", false},
		{"-*-*-*-*-*-*-12-*-*-*-m-*-*-*", "-adobe-courier-bold-o-normal--12-120-75-75-m-70-iso8859-1", true},
		{"-*-*-*-*-*-*-12-*-*-*-m-*-*-*", "-adobe-courier-bold-o-normal--12-120-75-75-X-70-iso8859-1", false},
		{"*a*b*g*t", "abcdefgopqrst", true},
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestWildmatchDoubleStar(t *testing.T) {
	// Within a single segment, '**' should behave the same as '*'
	// because paths are pre-split on '/' before reaching matchPattern.
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{"**", "foo", true},
		{"**", "", true},
		{"foo**", "foobar", true},
		{"**bar", "foobar", true},
		{"foo**bar", "foobar", true},
		{"foo**bar", "fooXbar", true},
		{"f**o", "fo", true},
		{"f**o", "fxo", true},
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestWildmatchEscape(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{`foo\*`, "foo*", true},
		{`foo\*`, "foobar", false},
		{`foo\?`, "foo?", true},
		{`foo\?`, "foox", false},
		{`f\\oo`, `f\oo`, true},
		{`\[ab]`, "[ab]", true},
		{`\?a\?b`, "?a?b", true},
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestWildmatchBracket(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{"[ten]", "t", true},
		{"[ten]", "e", true},
		{"[ten]", "n", true},
		{"[ten]", "ten", false}, // matches only one char
		{"t[a-g]n", "ten", true},
		{"t[a-g]n", "ton", false},
		{"t[!a-g]n", "ten", false},
		{"t[!a-g]n", "ton", true},
		{"t[^a-g]n", "ten", false},
		{"t[^a-g]n", "ton", true},
		{`[\\]`, `\`, true},
		{`[!\\]`, `\`, false},
		{`[,]`, `,`, true},
		{"[,-.]", "-", true},
		{"[,-.]", "+", false},
		{"[-]", "-", true},
		{"[------]", "-", true},
		{`[!------]`, "a", true},
		{"[a^bc]", "^", true},
		{`]`, "]", true}, // literal ']'
		{"[a-c[:digit:]x-z]", "5", true},
		{"[a-c[:digit:]x-z]", "b", true},
		{"[a-c[:digit:]x-z]", "y", true},
		{"[a-c[:digit:]x-z]", "q", false},
		{`[\\-^]`, "]", true},
		{`[\\-^]`, "[", false},
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestWildmatchPOSIXClass(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{"[[:alpha:]][[:digit:]][[:upper:]]", "a1B", true},
		{"[[:digit:][:upper:][:space:]]", "a", false},
		{"[[:digit:][:upper:][:space:]]", "A", true},
		{"[[:digit:][:upper:][:space:]]", "1", true},
		{"[[:digit:][:upper:][:space:]]", ".", false},
		{"[[:digit:][:punct:][:space:]]", ".", true},
		{"[[:xdigit:]]", "5", true},
		{"[[:xdigit:]]", "f", true},
		{"[[:xdigit:]]", "D", true},
		{"[[:xdigit:]]", "g", false},
		{`**[!te]`, "ten", true},
		{`**[!ten]`, "ten", false},
		{"[[:alnum:][:alpha:][:blank:][:cntrl:][:digit:][:graph:][:lower:][:print:][:punct:][:space:][:upper:][:xdigit:]]", "_", true},
		{"[^[:alnum:][:alpha:][:blank:][:cntrl:][:digit:][:lower:][:space:][:upper:][:xdigit:]]", ".", true},
		{"[[:digit:][:spaci:]]", "1", false}, // unknown class
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestWildmatchMalformed(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		// Unclosed bracket
		{"a[b", "ab", false},
		// Trailing backslash
		{`\`, ``, false},
		{`\`, `\`, false},
		// Empty bracket
		{"a[]b", "ab", false},
		{"a[]b", "a[]b", false},
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestWildmatchUnicode(t *testing.T) {
	tests := []struct {
		pattern string
		text    string
		want    bool
	}{
		{"*.txt", "你好-世界.txt", true},
		{"你好-世界.txt", "你好-世界.txt", true},
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("matchPattern(%q, %q) = %v, want %v", tt.pattern, tt.text, got, tt.want)
		}
	}
}

func TestWildmatchFileNAme(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		text    string
		want    bool
	}{
		{"foo* matches foobar", "foo*", "foobar", true},
		{"*foo* matches somethingfoobar", "*foo*", "somethingfoobar", true},
		{"*foo matches barfoo", "*foo", "barfoo", true},
		{"a[c-c]st does not match acrt", "a[c-c]st", "acrt", false},
		{"a[c-c]rt matches acrt", "a[c-c]rt", "acrt", true},
		{"file[[:space:]]with[[:space:]]spaces.# matches", "file[[:space:]]with[[:space:]]spaces.#", "file with spaces.#", true},
	}
	for _, tt := range tests {
		got := matchPattern(tt.pattern, tt.text)
		if got != tt.want {
			t.Errorf("%s: matchPattern(%q, %q) = %v, want %v", tt.name, tt.pattern, tt.text, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// ParsePattern + Match integration tests (full path matching)
// ---------------------------------------------------------------------------

func TestParsePatternBasic(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    []string
		isDir   bool
		want    MatchResult
	}{
		// Simple name matching (no '/' in pattern)
		{"simple match", "foo", []string{"foo"}, false, Exclude},
		{"simple no match", "foo", []string{"bar"}, false, NoMatch},
		{"simple match in subdir", "foo", []string{"dir", "foo"}, false, Exclude},
		{"simple wildcard", "*.txt", []string{"file.txt"}, false, Exclude},
		{"simple wildcard no match", "*.txt", []string{"file.go"}, false, NoMatch},
		{"question mark wildcard", "f?o", []string{"foo"}, false, Exclude},
		{"question mark no match", "f?o", []string{"fo"}, false, NoMatch},

		// Inclusion (negation)
		{"inclusion", "!foo", []string{"foo"}, false, Include},
		{"inclusion no match", "!foo", []string{"bar"}, false, NoMatch},

		// Dir-only patterns (trailing /)
		{"dir only matches dir", "foo/", []string{"foo"}, true, Exclude},
		{"dir only no match on file", "foo/", []string{"foo"}, false, NoMatch},
		{"dir only matches subdir of dir", "foo/", []string{"foo", "bar"}, false, Exclude},
		{"dir only matches subdir of dir (isDir)", "foo/", []string{"foo"}, true, Exclude},

		// Glob patterns (containing /)
		{"glob match", "foo/bar", []string{"foo", "bar"}, false, Exclude},
		{"glob no match", "foo/bar", []string{"foo", "baz"}, false, NoMatch},
		{"glob missing segment", "foo/bar/baz", []string{"foo", "bar"}, false, NoMatch},
		{"glob wildcard", "foo/*.txt", []string{"foo", "file.txt"}, false, Exclude},
		{"glob wildcard no match", "foo/*.txt", []string{"foo", "file.go"}, false, NoMatch},

		// Leading slash (rooted pattern)
		{"anchored match", "/foo", []string{"foo"}, false, Exclude},
		{"anchored no match in subdir", "/foo", []string{"dir", "foo"}, false, NoMatch},

		// Double-star patterns
		{"double star prefix", "**/foo", []string{"foo"}, false, Exclude},
		{"double star prefix deep", "**/foo", []string{"dir", "subdir", "foo"}, false, Exclude},
		{"double star middle", "foo/**/bar", []string{"foo", "bar"}, false, Exclude},
		{"double star middle deep", "foo/**/bar", []string{"foo", "x", "y", "bar"}, false, Exclude},
		{"double star suffix", "foo/**", []string{"foo", "bar"}, false, Exclude},
		{"double star suffix is dir", "foo/**", []string{"foo"}, true, Exclude},
		{"double star suffix no match empty", "foo/**", []string{"foo"}, false, NoMatch},

		// Bracket expressions
		{"bracket match", "foo[abc]", []string{"foob"}, false, Exclude},
		{"bracket negated", "foo[!abc]", []string{"food"}, false, Exclude},
		{"bracket negated no match", "foo[!abc]", []string{"foob"}, false, NoMatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParsePattern(tt.pattern, nil)
			got := p.Match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ParsePattern(%q).Match(%v, %v) = %v, want %v",
					tt.pattern, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestParsePatternWithDomain(t *testing.T) {
	// Patterns with a domain should only match paths that start with the domain.
	p := ParsePattern("*.go", []string{"cmd", "zeta"})
	if p.Match([]string{"cmd", "zeta", "main.go"}, false) != Exclude {
		t.Error("expected *.go in cmd/zeta to match cmd/zeta/main.go")
	}
	if p.Match([]string{"cmd", "other", "main.go"}, false) != NoMatch {
		t.Error("expected *.go in cmd/zeta not to match cmd/other/main.go")
	}
	if p.Match([]string{"cmd"}, false) != NoMatch {
		t.Error("expected *.go in cmd/zeta not to match just cmd")
	}
}

func TestParsePatternDoubleStarInSegment(t *testing.T) {
	// Patterns where '**' appears inside a segment (e.g., "foo**bar") should
	// be treated as regular wildcard patterns, not as zero-or-more-dirs.
	tests := []struct {
		name    string
		pattern string
		path    []string
		isDir   bool
		want    MatchResult
	}{
		{
			name:    "embedded double star in segment matches",
			pattern: "foo**bar",
			path:    []string{"fooXbar"},
			isDir:   false,
			want:    Exclude,
		},
		{
			name:    "embedded double star matches zero chars",
			pattern: "foo**bar",
			path:    []string{"foobar"},
			isDir:   false,
			want:    Exclude,
		},
		{
			name:    "embedded double star no match",
			pattern: "foo**bar",
			path:    []string{"foobaz"},
			isDir:   false,
			want:    NoMatch,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParsePattern(tt.pattern, nil)
			got := p.Match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ParsePattern(%q).Match(%v, %v) = %v, want %v",
					tt.pattern, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestParsePatternTrailingDoubleStar(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    []string
		isDir   bool
		want    MatchResult
	}{
		{"trailing ** matches deep content",
			"src/**", []string{"src", "main.go"}, false, Exclude},
		{"trailing ** matches dir itself",
			"src/**", []string{"src"}, true, Exclude},
		{"trailing ** no match for non-dir with no children",
			"src/**", []string{"src"}, false, NoMatch},
		{"trailing ** matches nested",
			"src/**", []string{"src", "pkg", "util.go"}, false, Exclude},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParsePattern(tt.pattern, nil)
			got := p.Match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ParsePattern(%q).Match(%v, %v) = %v, want %v",
					tt.pattern, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestParsePatternDirOnlyWithGlob(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    []string
		isDir   bool
		want    MatchResult
	}{
		{"dir-only glob matches dir",
			"build/", []string{"build"}, true, Exclude},
		{"dir-only glob no match file",
			"build/", []string{"build"}, false, NoMatch},
		{"dir-only glob matches contents",
			"build/", []string{"build", "output"}, false, Exclude},
		{"dir-only double-star matches dir",
			"build/**/", []string{"build", "tmp"}, true, Exclude},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParsePattern(tt.pattern, nil)
			got := p.Match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ParsePattern(%q).Match(%v, %v) = %v, want %v",
					tt.pattern, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestParsePatternBracketInName(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    []string
		isDir   bool
		want    MatchResult
	}{
		{"bracket in simple name",
			"file[0-9].txt", []string{"file5.txt"}, false, Exclude},
		{"bracket no match",
			"file[0-9].txt", []string{"fileA.txt"}, false, NoMatch},
		{"bracket in glob",
			"src/file[0-9].txt", []string{"src", "file3.txt"}, false, Exclude},
		{"negated bracket in simple name",
			"[!abc]*", []string{"def"}, false, Exclude},
		{"negated bracket no match",
			"[!abc]*", []string{"abc"}, false, NoMatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParsePattern(tt.pattern, nil)
			got := p.Match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ParsePattern(%q).Match(%v, %v) = %v, want %v",
					tt.pattern, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

func TestParsePatternEscapeInPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    []string
		isDir   bool
		want    MatchResult
	}{
		{"escaped star in simple name",
			`foo\*`, []string{"foo*"}, false, Exclude},
		{"escaped star no match literal",
			`foo\*`, []string{"foobar"}, false, NoMatch},
		{"escaped question mark",
			`foo\?`, []string{"foo?"}, false, Exclude},
		{"escaped question no match",
			`foo\?`, []string{"foox"}, false, NoMatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParsePattern(tt.pattern, nil)
			got := p.Match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ParsePattern(%q).Match(%v, %v) = %v, want %v",
					tt.pattern, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

// TestParsePatternComplexDoubleStar tests various ** combinations that were
// previously broken by the filepath.Match limitation.
func TestParsePatternComplexDoubleStar(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    []string
		isDir   bool
		want    MatchResult
	}{
		// ** at the start
		{"** at start matches root",
			"**/foo", []string{"foo"}, false, Exclude},
		{"** at start matches deep",
			"**/foo", []string{"a", "b", "c", "foo"}, false, Exclude},
		{"** at start with glob after",
			"**/*.txt", []string{"a", "b", "c.txt"}, false, Exclude},
		{"** at start no match wrong ext",
			"**/*.txt", []string{"a", "b", "c.go"}, false, NoMatch},

		// ** in the middle
		{"** in middle matches zero dirs",
			"src/**/test", []string{"src", "test"}, true, Exclude},
		{"** in middle matches one dir",
			"src/**/test", []string{"src", "pkg", "test"}, true, Exclude},
		{"** in middle matches many dirs",
			"src/**/test", []string{"src", "a", "b", "c", "test"}, true, Exclude},
		{"** in middle no match",
			"src/**/test", []string{"lib", "test"}, true, NoMatch},

		// ** at the end
		{"** at end matches all",
			"src/**", []string{"src", "a", "b", "c"}, false, Exclude},
		{"** at end matches one level",
			"src/**", []string{"src", "a"}, false, Exclude},
		{"** at end matches dir itself",
			"src/**", []string{"src"}, true, Exclude},
		{"** at end no match for file with same name",
			"src/**", []string{"src"}, false, NoMatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParsePattern(tt.pattern, nil)
			got := p.Match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ParsePattern(%q).Match(%v, %v) = %v, want %v",
					tt.pattern, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

// TestParsePatternPOSIXClassInName tests that POSIX character classes work
// correctly in the new matchPattern-based matcher.
func TestParsePatternPOSIXClassInName(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    []string
		isDir   bool
		want    MatchResult
	}{
		{"alpha class match",
			"[[:alpha:]].go", []string{"a.go"}, false, Exclude},
		{"alpha class no match digit",
			"[[:alpha:]].go", []string{"1.go"}, false, NoMatch},
		{"digit class match",
			"[[:digit:]].go", []string{"5.go"}, false, Exclude},
		{"digit class no match alpha",
			"[[:digit:]].go", []string{"a.go"}, false, NoMatch},
		{"alnum class match",
			"[[:alnum:]]", []string{"x"}, false, Exclude},
		{"alnum class no match punct",
			"[[:alnum:]]", []string{"."}, false, NoMatch},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := ParsePattern(tt.pattern, nil)
			got := p.Match(tt.path, tt.isDir)
			if got != tt.want {
				t.Errorf("ParsePattern(%q).Match(%v, %v) = %v, want %v",
					tt.pattern, tt.path, tt.isDir, got, tt.want)
			}
		})
	}
}

// TestOriginalTestcase keeps the original TestMatch assertion as a regression test.
func TestOriginalTestcase(t *testing.T) {
	p := ParsePattern("**/*lue/vol?ano", nil)
	r := p.Match([]string{"head", "value", "volcano", "tail"}, false)
	if r != Exclude {
		t.Errorf("expected Exclude, got %v", r)
	}
}

// Benchmarks

func BenchmarkWildmatchLiteral(b *testing.B) {
	for b.Loop() {
		matchPattern("foobar", "foobar")
	}
}

func BenchmarkWildmatchStar(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matchPattern("*-adobe-courier-bold-o-normal--12-120-75-75-m-70-iso8859-1",
			"-adobe-courier-bold-o-normal--12-120-75-75-m-70-iso8859-1")
	}
}

func BenchmarkWildmatchBracket(b *testing.B) {
	for b.Loop() {
		matchPattern("[a-z][0-9]*.txt", "a5hello.txt")
	}
}

func BenchmarkParsePatternAndMatch(b *testing.B) {
	for b.Loop() {
		p := ParsePattern("**/*.go", nil)
		p.Match([]string{"cmd", "zeta", "main.go"}, false)
	}
}
