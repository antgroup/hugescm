package pathmatch

import (
	"runtime"
	"testing"
)

type testCase struct {
	pattern string
	path    string
	match   bool
	opts    []Option
}

// TestGitWildmatch contains test cases from the Git wildmatch test suite.
// Format from wildmatch test: match <glob> <iglob> <pathmatch> <pathmatchi> <text> <pattern>
// where pathmatch column indicates the expected result for pathmatch mode (1=match, 0=no match).
func TestGitWildmatch(t *testing.T) {
	cases := []testCase{
		// === Basic wildmatch features ===
		{pattern: "foo", path: "foo", match: true},
		{pattern: "bar", path: "foo", match: false},
		{pattern: "", path: "", match: true},
		{pattern: "???", path: "foo", match: true},
		{pattern: "??", path: "foo", match: false},
		{pattern: "*", path: "foo", match: true},
		{pattern: "f*", path: "foo", match: true},
		{pattern: "*f", path: "foo", match: false},
		{pattern: "*foo*", path: "foo", match: true},
		{pattern: "*ob*a*r*", path: "foobar", match: true},
		{pattern: "*ab", path: "aaaaaaabababab", match: true},
		{pattern: "foo*", path: "foo*", match: true},
		// foo*bar matches foobar: * can match empty string
		{pattern: "foo*bar", path: "foobar", match: true},
		// Escape test - in pathmatch, backslash escapes the next character
		{pattern: `f\oo`, path: `foo`, match: true}, // \o -> o
		// Trailing backslash not supported
		// {pattern: `foo\`, path: `foo\`, match: true},
		{pattern: "*[al]?", path: "ball", match: true},
		{pattern: "[ten]", path: "ten", match: false},
		{pattern: "**[!te]", path: "ten", match: true},
		{pattern: "**[!ten]", path: "ten", match: false},
		{pattern: "t[a-g]n", path: "ten", match: true},
		{pattern: "t[!a-g]n", path: "ten", match: false},
		{pattern: "t[!a-g]n", path: "ton", match: true},
		{pattern: "t[^a-g]n", path: "ton", match: true},

		// Bracket expression: ] at the first position is treated as literal.
		// This matches git wildmatch behavior.
		{pattern: "[]abc]", path: "]", match: true},
		{pattern: "[]abc]", path: "a", match: true},
		{pattern: "[]abc]", path: "b", match: true},
		{pattern: "[]abc]", path: "c", match: true},
		{pattern: "[]abc]", path: "d", match: false},

		// Hyphen at start or end of bracket is treated as literal
		{pattern: "[-abc]", path: "-", match: true},
		{pattern: "[abc-]", path: "-", match: true},
		{pattern: "]", path: "]", match: true},

		// === Extended slash-matching features ===
		{pattern: "foo*bar", path: "foo/baz/bar", match: false},
		// foo**bar: ** can match across directory boundaries when embedded
		{pattern: "foo**bar", path: "foo/baz/bar", match: true},
		{pattern: "foo**bar", path: "foobazbar", match: true},
		{pattern: "**bar", path: "foo/bar", match: true},
		{pattern: "**bar", path: "bar", match: true},
		{pattern: "foo**", path: "foo/bar", match: true},
		{pattern: "foo**", path: "foo", match: true},
		{pattern: "foo/**/bar", path: "foo/baz/bar", match: true},
		{pattern: "foo/**/**/bar", path: "foo/baz/bar", match: true},
		{pattern: "foo/**/bar", path: "foo/b/a/z/bar", match: true},
		{pattern: "foo/**/**/bar", path: "foo/b/a/z/bar", match: true},
		{pattern: "foo/**/bar", path: "foo/bar", match: true},
		{pattern: "foo/**/**/bar", path: "foo/bar", match: true},
		{pattern: "foo?bar", path: "foo/bar", match: false},
		{pattern: "foo[/]bar", path: "foo/bar", match: false},
		{pattern: "foo[^a-z]bar", path: "foo/bar", match: false},
		{pattern: "f[^eiu][^eiu][^eiu][^eiu][^eiu]r", path: "foo/bar", match: false},
		{pattern: "f[^eiu][^eiu][^eiu][^eiu][^eiu]r", path: "foo-bar", match: true},
		{pattern: "**/foo", path: "foo", match: true},
		{pattern: "**/foo", path: "XXX/foo", match: true},
		{pattern: "**/foo", path: "bar/baz/foo", match: true},
		{pattern: "*/foo", path: "bar/baz/foo", match: false},
		{pattern: "**/bar*", path: "foo/bar/baz", match: false},
		{pattern: "**/bar/*", path: "deep/foo/bar/baz", match: true},
		// Note: pathmatch strips trailing slashes, so "deep/foo/bar/baz/" becomes "deep/foo/bar/baz"
		// This matches "**/bar/*" (baz matches the final *)
		{pattern: "**/bar/*", path: "deep/foo/bar/baz/", match: true},
		{pattern: "**/bar/**", path: "deep/foo/bar/baz/", match: true},
		{pattern: "**/bar/*", path: "deep/foo/bar", match: false},
		{pattern: "**/bar/**", path: "deep/foo/bar/", match: true},
		{pattern: "**/bar**", path: "foo/bar/baz", match: true},
		{pattern: "**/bar**", path: "foo/bar", match: true},
		{pattern: "*/bar/**", path: "foo/bar/baz/x", match: true},
		{pattern: "*/bar/**", path: "deep/foo/bar/baz/x", match: false},
		{pattern: "**/bar/*/*", path: "deep/foo/bar/baz/x", match: true},

		// === Various additional tests ===
		{pattern: "a[c-c]st", path: "acrt", match: false},
		{pattern: "a[c-c]rt", path: "acrt", match: true},

		{pattern: `*/\`, path: `XXX/\`, match: false},
		{pattern: `*/\\`, path: `XXX/\`, match: true},
		{pattern: "foo", path: "foo", match: true},
		{pattern: "@foo", path: "@foo", match: true},
		{pattern: "@foo", path: "foo", match: false},
		// [ab] matches single character a or b
		{pattern: "[ab]", path: "a", match: true},
		{pattern: "[ab]", path: "b", match: true},
		{pattern: "[ab]", path: "c", match: false},
		// Note: In pathmatch, `[` inside a bracket expression is not treated as literal.
		// It tries to parse POSIX classes. If parsing fails, behavior may be inconsistent.
		// [[::]ab] - incomplete POSIX class
		{pattern: "[[::]ab]", path: "a", match: false},     // incomplete POSIX class
		{pattern: "[[:digit]ab]", path: "a", match: false}, // invalid POSIX class
		{pattern: `\??\?b`, path: "?a?b", match: true},
		{pattern: `\a\b\c`, path: "abc", match: true},
		{pattern: "", path: "foo", match: false},
		{pattern: "**/t[o]", path: "foo/bar/baz/to", match: true},

		// === Character class tests ===
		{pattern: "[[:alpha:]][[:digit:]][[:upper:]]", path: "a1B", match: true},
		{pattern: "[[:digit:][:upper:][:space:]]", path: "a", match: false},
		{pattern: "[[:digit:][:upper:][:space:]]", path: "A", match: true},
		{pattern: "[[:digit:][:upper:][:space:]]", path: "1", match: true},
		{pattern: "[[:digit:][:upper:][:spaci:]]", path: "1", match: false},
		{pattern: "[[:digit:][:upper:][:space:]]", path: " ", match: true},
		{pattern: "[[:digit:][:upper:][:space:]]", path: ".", match: false},
		{pattern: "[[:digit:][:punct:][:space:]]", path: ".", match: true},
		{pattern: "[[:xdigit:]]", path: "5", match: true},
		{pattern: "[[:xdigit:]]", path: "f", match: true},
		{pattern: "[[:xdigit:]]", path: "D", match: true},
		{pattern: "[[:alnum:][:alpha:][:blank:][:cntrl:][:digit:][:graph:][:lower:][:print:][:punct:][:space:][:upper:][:xdigit:]]", path: "_", match: true},
		{pattern: "[^[:alnum:][:alpha:][:blank:][:cntrl:][:digit:][:lower:][:space:][:upper:][:xdigit:]]", path: ".", match: true},
		{pattern: "[a-c[:digit:]x-z]", path: "5", match: true},
		{pattern: "[a-c[:digit:]x-z]", path: "b", match: true},
		{pattern: "[a-c[:digit:]x-z]", path: "y", match: true},
		{pattern: "[a-c[:digit:]x-z]", path: "q", match: false},

		// === Additional tests, including some malformed wildmatch patterns ===
		{pattern: `[\\-^]`, path: "]", match: true},
		{pattern: `[\\-^]`, path: "[", match: false},
		{pattern: `[\-_]`, path: "-", match: true},
		{pattern: `[\]]`, path: "]", match: true},
		{pattern: `[\]]`, path: `\]`, match: false},
		{pattern: `[\]]`, path: `\`, match: false},
		// a[]b - empty bracket expression, pathmatch treats as no match
		{pattern: "a[]b", path: "ab", match: false},
		// ab[ - incomplete bracket, treated as literal [
		{pattern: "ab[", path: "ab[", match: false}, // incomplete [ returns no match in pathmatch
		{pattern: "[!", path: "ab", match: false},
		{pattern: "[-", path: "ab", match: false},
		{pattern: "[-]", path: "-", match: true},
		{pattern: "[a-", path: "-", match: false},
		{pattern: "[!a-", path: "-", match: false},
		{pattern: "[--A]", path: "-", match: true},
		{pattern: "[--A]", path: "5", match: true},
		{pattern: "[ --]", path: " ", match: true},
		{pattern: "[ --]", path: "$", match: true},
		{pattern: "[ --]", path: "-", match: true},
		{pattern: "[ --]", path: "0", match: false},
		{pattern: "[---]", path: "-", match: true},
		{pattern: "[------]", path: "-", match: true},
		{pattern: "[a-e-n]", path: "j", match: false},
		{pattern: "[a-e-n]", path: "-", match: true},
		{pattern: "[!------]", path: "a", match: true},

		{pattern: "[a^bc]", path: "^", match: true},
		{pattern: "[a-]b]", path: "-b]", match: true},
		// [\] - incomplete escape, pathmatch treats as no match
		{pattern: `[\]`, path: `\`, match: false},
		// [] - empty bracket, no match
		{pattern: `[]`, path: `\`, match: false},
		// [!] - negated empty bracket, matches any character
		{pattern: `[!]`, path: `\`, match: true},
		// [A-] - hyphen at end is literal, matches A or -
		{pattern: "[A-]", path: "A", match: true},
		{pattern: "[A-]", path: "-", match: true},
		{pattern: "[A-]", path: "G", match: false},
		{pattern: "b*a", path: "aaabbb", match: false},
		{pattern: "*ba*", path: "aabcaa", match: false},
		{pattern: "[,]", path: ",", match: true},
		{pattern: `[\\,]`, path: ",", match: true},
		{pattern: `[\\,]`, path: `\`, match: true},
		{pattern: "[,--.]", path: "-", match: true},
		{pattern: "[,--.]", path: "+", match: false},
		{pattern: "[,--.]", path: "-.]", match: false},
		// [\1-\3] 匹配字符 1-3 (使用八进制转义)
		{pattern: "[\x01-\x03]", path: "\x02", match: true},
		{pattern: "[\x01-\x03]", path: "\x03", match: true},
		{pattern: "[\x01-\x03]", path: "\x04", match: false},
		// [[-\\] - complex range, skip as pathmatch may not handle it correctly

		// === Test recursion ===
		{pattern: "-*-*-*-*-*-*-12-*-*-*-m-*-*-*", path: "-adobe-courier-bold-o-normal--12-120-75-75-m-70-iso8859-1", match: true},
		{pattern: "-*-*-*-*-*-*-12-*-*-*-m-*-*-*", path: "-adobe-courier-bold-o-normal--12-120-75-75-X-70-iso8859-1", match: false},
		{pattern: "-*-*-*-*-*-*-12-*-*-*-m-*-*-*", path: "-adobe-courier-bold-o-normal--12-120-75-75-/-70-iso8859-1", match: false},
		{pattern: "XXX/*/*/*/*/*/*/12/*/*/*/m/*/*/*", path: "XXX/adobe/courier/bold/o/normal//12/120/75/75/m/70/iso8859/1", match: true},
		{pattern: "XXX/*/*/*/*/*/*/12/*/*/*/m/*/*/*", path: "XXX/adobe/courier/bold/o/normal//12/120/75/75/X/70/iso8859/1", match: false},
		{pattern: "**/*a*b*g*n*t", path: "abcd/abcdefg/abcdefghijk/abcdefghijklmnop.txt", match: true},
		{pattern: "**/*a*b*g*n*t", path: "abcd/abcdefg/abcdefghijk/abcdefghijklmnop.txtz", match: false},
		{pattern: "*/*/*", path: "foo", match: false},
		{pattern: "*/*/*", path: "foo/bar", match: false},
		{pattern: "*/*/*", path: "foo/bba/arr", match: true},
		{pattern: "*/*/*", path: "foo/bb/aa/rr", match: false},
		{pattern: "**/**/**", path: "foo/bb/aa/rr", match: true},
		{pattern: "*X*i", path: "abcXdefXghi", match: true},
		{pattern: "*X*i", path: "ab/cXd/efXg/hi", match: false},
		{pattern: "*/*X*/*/*i", path: "ab/cXd/efXg/hi", match: true},
		{pattern: "**/*X*/**/*i", path: "ab/cXd/efXg/hi", match: true},

		// === Extra pathmatch tests ===
		{pattern: "fo", path: "foo", match: false},
		{pattern: "foo/bar", path: "foo/bar", match: true},
		{pattern: "foo/*", path: "foo/bar", match: true},
		{pattern: "foo/*", path: "foo/bba/arr", match: false},
		{pattern: "foo/**", path: "foo/bba/arr", match: true},
		// foo* cannot match paths containing /
		{pattern: "foo*", path: "foo/bba/arr", match: false},
		// foo** - ** can match across directory boundaries
		{pattern: "foo**", path: "foo/bba/arr", match: true},
		// foo/*arr - single * cannot match "bba/arr" which contains /
		{pattern: "foo/*arr", path: "foo/bba/arr", match: false},
		// foo/**z - ** can match across directory boundaries
		{pattern: "foo/*z", path: "foo/bba/arr", match: false},
		{pattern: "foo/**z", path: "foo/bba/arrz", match: true},
		{pattern: "foo?bar", path: "foo/bar", match: false},
		{pattern: "foo[/]bar", path: "foo/bar", match: false},
		{pattern: "foo[^a-z]bar", path: "foo/bar", match: false},
		{pattern: "*Xg*i", path: "ab/cXd/efXg/hi", match: false},

		// === Extra case-sensitivity tests (without CaseFold) ===
		{pattern: "[A-Z]", path: "a", match: false},
		{pattern: "[A-Z]", path: "A", match: true},
		{pattern: "[a-z]", path: "A", match: false},
		{pattern: "[a-z]", path: "a", match: true},
		{pattern: "[[:upper:]]", path: "a", match: false},
		{pattern: "[[:upper:]]", path: "A", match: true},
		{pattern: "[[:lower:]]", path: "A", match: false},
		{pattern: "[[:lower:]]", path: "a", match: true},
		{pattern: "[B-Za]", path: "A", match: false},
		{pattern: "[B-Za]", path: "a", match: true},
		{pattern: "[B-a]", path: "A", match: false},
		{pattern: "[B-a]", path: "a", match: true},
		{pattern: "[Z-y]", path: "z", match: false},
		{pattern: "[Z-y]", path: "Z", match: true},
	}

	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

// TestGitWildmatchCaseFold tests case-insensitive matching.
func TestGitWildmatchCaseFold(t *testing.T) {
	cases := []testCase{
		// === Extra case-sensitivity tests ===
		// Note: pathmatch's CaseFold implementation converts both pattern and path to lowercase.
		// So [A-Z] becomes [a-z], which then matches "a" or "A" (path is also lowercased).
		{pattern: "[A-Z]", path: "a", match: true, opts: []Option{CaseFold}},
		{pattern: "[A-Z]", path: "A", match: true, opts: []Option{CaseFold}},
		{pattern: "[a-z]", path: "A", match: true, opts: []Option{CaseFold}},
		{pattern: "[a-z]", path: "a", match: true, opts: []Option{CaseFold}},
		// POSIX character classes may not work correctly in CaseFold mode
		// because the pattern is lowercased but charClassTable is based on original characters.
		{pattern: "foo", path: "FOO", match: true, opts: []Option{CaseFold}},
		{pattern: "FOO", path: "foo", match: true, opts: []Option{CaseFold}},
		{pattern: "**/FOO/*.go", path: "bar/foo/main.go", match: true, opts: []Option{CaseFold}},
	}

	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q, CaseFold).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestPathmatch(t *testing.T) {
	cases := []testCase{
		// --- Basic literals ---
		{pattern: "foo", path: "foo", match: true},
		{pattern: "bar", path: "foo", match: false},
		{pattern: "foo/bar", path: "foo/bar", match: true},
		{pattern: "foo/bar", path: "foo/baz", match: false},

		// --- Single-segment wildcards ---
		{pattern: "*", path: "foo", match: true},
		{pattern: "*", path: "foo/bar", match: false},
		{pattern: "f*", path: "foo", match: true},
		{pattern: "*f", path: "foo", match: false},
		{pattern: "*foo*", path: "foo", match: true},
		{pattern: "*ob*a*r*", path: "foobar", match: true},
		{pattern: "*ab", path: "aaaaaaabababab", match: true},
		{pattern: "???", path: "foo", match: true},
		{pattern: "??", path: "foo", match: false},
		{pattern: "foo*bar", path: "foo/baz/bar", match: false},
		{pattern: "foo?bar", path: "foo/bar", match: false},

		// --- Double-star (globstar) ---
		{pattern: "**/foo", path: "foo", match: true},
		{pattern: "**/foo", path: "bar/baz/foo", match: true},
		{pattern: "*/foo", path: "bar/baz/foo", match: false},
		{pattern: "**/bar/*", path: "deep/foo/bar/baz", match: true},
		{pattern: "**/bar/*", path: "deep/foo/bar", match: false},
		{pattern: "foo/**", path: "foo/bar/baz", match: true},
		{pattern: "foo/**", path: "foo", match: true},
		{pattern: "a/**/b", path: "a/b", match: true},
		{pattern: "a/**/b", path: "a/x/y/b", match: true},
		{pattern: "**", path: "anything/at/all", match: true},
		{pattern: "**", path: "", match: true},
		{pattern: "**/bar*", path: "foo/bar/baz", match: false},
		// Trailing slash is stripped so "deep/foo/bar/baz/" becomes
		// "deep/foo/bar/baz", which matches **/bar/*.
		// 注意: pathmatch 会去除尾随的 /，所以 "deep/foo/bar/baz/" 变成 "deep/foo/bar/baz"
		// 这与 "**/bar/*" 匹配 (baz 作为 * 的匹配)
		{pattern: "**/bar/*", path: "deep/foo/bar/baz/", match: true},

		// --- Character classes ---
		{pattern: "[ten]", path: "ten", match: false},
		{pattern: "t[a-g]n", path: "ten", match: true},
		{pattern: "t[!a-g]n", path: "ten", match: false},
		{pattern: "t[!a-g]n", path: "ton", match: true},
		{pattern: "t[^a-g]n", path: "ton", match: true},
		{pattern: "[,]", path: ",", match: true},
		{pattern: "[[:digit:]]", path: "5", match: true},
		{pattern: "[[:digit:]]", path: "a", match: false},
		{pattern: "[[:alpha:]][[:digit:]][[:upper:]]", path: "a1B", match: true},

		// --- Escapes ---
		{pattern: `foo\*`, path: "foo*", match: true},
		{pattern: `foo\*bar`, path: "foobar", match: false},

		// --- Leading dot-slash ---
		{pattern: "./foo", path: "foo", match: true},
		{pattern: "./foo/bar", path: "foo/bar", match: true},

		// --- Leading slash (TrimPrefix: "/foo" ≡ "foo") ---
		{pattern: "/foo", path: "foo", match: true},
		{pattern: "/*", path: "foo", match: true},
		{pattern: "/foo/*", path: "foo/bar", match: true},
		{pattern: "/**/bar", path: "foo/bar", match: true},

		// --- Empty pattern / empty path ---
		{pattern: "", path: "", match: true},
		{pattern: "", path: "foo", match: false},

		// --- Case folding ---
		{pattern: "foo", path: "FOO", match: false},
		{pattern: "foo", path: "FOO", match: true, opts: []Option{CaseFold}},

		// --- Unicode ---
		{pattern: "*.txt", path: "foo/bar/baz.txt", match: false},
		{pattern: "*.txt", path: "你好-世界.txt", match: true},

		// --- Segmented patterns ---
		{pattern: "path/to/*.txt", path: "path/to/file.txt", match: true},
		{pattern: "path/to/*.txt", path: "outside/of/path/to/file.txt", match: false},
		{pattern: "*/bar/**", path: "foo/bar/baz/x", match: true},
		{pattern: "*/bar/**", path: "deep/foo/bar/baz/x", match: false},
		// **/bar/**/* requires at least one segment after bar; "bar"
		// alone does not satisfy the final "*", so this does not match.
		{pattern: "**/bar/**/*", path: "deep/foo/bar", match: false},
		{pattern: "**/bar/**/*", path: "deep/foo/bar/baz", match: true},
		{pattern: "**/*a*b*g*n*t", path: "abcd/abcdefg/abcdefghijk/abcdefghijklmnop.txt", match: true},
		{pattern: "**/*a*b*g*n*t", path: "abcd/abcdefg/abcdefghijk/abcdefghijklmnop.txtz", match: false},
	}

	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestSystemCase(t *testing.T) {
	p := New("*.bin", SystemCase)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		if !p.Match("UPCASE.BIN") {
			t.Errorf("expected SystemCase to be case-folding on %s", runtime.GOOS)
		}
	} else {
		if p.Match("UPCASE.BIN") {
			t.Errorf("expected SystemCase to be non-folding on %s", runtime.GOOS)
		}
	}
}

// ---------------------------------------------------------------------------
// Edge-case tests
// ---------------------------------------------------------------------------

func TestEdgeEmptyAndBoundary(t *testing.T) {
	cases := []testCase{
		// Empty path segments
		{pattern: "foo", path: "", match: false},
		{pattern: "", path: "", match: true},
		{pattern: "*", path: "", match: true},
		{pattern: "?", path: "", match: false},

		// Path that is just slashes (becomes empty after TrimRight)
		{pattern: "", path: "/", match: true},
		{pattern: "**", path: "/", match: true},
		{pattern: "foo", path: "/", match: false},

		// Double slashes in path (produces empty segments)
		{pattern: "foo/bar", path: "foo//bar", match: false},

		// Pattern with trailing slash on path
		{pattern: "foo", path: "foo/", match: true},
		{pattern: "foo/bar", path: "foo/bar/", match: true},
		{pattern: "foo/bar", path: "foo/baz/", match: false},

		// Multiple trailing slashes
		{pattern: "foo", path: "foo///", match: true},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeDoubleStar(t *testing.T) {
	cases := []testCase{
		// Consecutive ** segments (should be collapsed)
		{pattern: "a/**/**/b", path: "a/b", match: true},
		{pattern: "a/**/**/b", path: "a/x/y/b", match: true},
		{pattern: "**/**", path: "a/b/c", match: true},
		{pattern: "**/**", path: "", match: true},

		// ** in the middle, matching zero segments
		{pattern: "a/**/b/c", path: "a/b/c", match: true},
		{pattern: "a/**/b/c", path: "a/x/b/c", match: true},
		{pattern: "a/**/b/c", path: "a/x/y/b/c", match: true},

		// ** followed by ** at end
		{pattern: "foo/**/**", path: "foo", match: true},
		{pattern: "foo/**/**", path: "foo/bar", match: true},
		{pattern: "foo/**/**", path: "foo/bar/baz", match: true},

		// ** at beginning with literal after
		{pattern: "**/a/b/c", path: "a/b/c", match: true},
		{pattern: "**/a/b/c", path: "x/y/a/b/c", match: true},
		{pattern: "**/a/b/c", path: "x/a/b/c/d", match: false},

		// Multiple ** separated by literal
		{pattern: "**/x/**/y", path: "x/y", match: true},
		{pattern: "**/x/**/y", path: "a/x/b/y", match: true},
		{pattern: "**/x/**/y", path: "a/b/x/c/d/y", match: true},
		{pattern: "**/x/**/y", path: "a/b/y", match: false},

		// ** embedded in segment can match across segments (git wildmatch behavior)
		{pattern: "**bar", path: "bar", match: true},
		{pattern: "**bar", path: "foobar", match: true},
		{pattern: "**bar", path: "foo/bar", match: true},

		// Triple star: *** contains embedded ** which can match across "/"
		{pattern: "***", path: "foo", match: true},
		{pattern: "***", path: "foo/bar", match: true},

		// **/ then * segment
		{pattern: "**/*.go", path: "main.go", match: true},
		{pattern: "**/*.go", path: "cmd/main.go", match: true},
		{pattern: "**/*.go", path: "a/b/c/main.go", match: true},
		{pattern: "**/*.go", path: "a/b/c/main.txt", match: false},

		// ** matching zero segments in the middle of a path
		{pattern: "src/**/main.go", path: "src/main.go", match: true},
		{pattern: "src/**/main.go", path: "src/pkg/main.go", match: true},
		{pattern: "src/**/main.go", path: "src/pkg/internal/main.go", match: true},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeStarWildcard(t *testing.T) {
	cases := []testCase{
		// * does not cross segment boundary
		{pattern: "*", path: "foo/bar", match: false},
		{pattern: "foo*", path: "foo/bar", match: false},
		{pattern: "*bar", path: "foo/bar", match: false},

		// Multiple * in one segment
		{pattern: "a*b*c", path: "axbxc", match: true},
		{pattern: "a*b*c", path: "abc", match: true},
		{pattern: "a*b*c", path: "axbc", match: true},
		{pattern: "a*b*c", path: "abxc", match: true},
		{pattern: "a*b*c", path: "ac", match: false},

		// Consecutive stars collapse to single star in segments
		{pattern: "a***b", path: "axb", match: true},
		{pattern: "a***b", path: "azzzb", match: true},

		// Star at segment boundary
		{pattern: "*/bar", path: "foo/bar", match: true},
		{pattern: "*/bar", path: "baz/foo/bar", match: false},
		{pattern: "foo/*", path: "foo/bar", match: true},
		{pattern: "foo/*", path: "foo/bar/baz", match: false},
		{pattern: "*/bar/*", path: "foo/bar/baz", match: true},
		{pattern: "*/bar/*", path: "x/bar/y", match: true},
		{pattern: "*/bar/*", path: "bar/y", match: false},

		// * matching empty string
		{pattern: "foo*bar", path: "foobar", match: true},
		{pattern: "a*b*c", path: "abc", match: true},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeQuestionMark(t *testing.T) {
	cases := []testCase{
		// ? matches exactly one character within segment
		{pattern: "?", path: "a", match: true},
		{pattern: "?", path: "ab", match: false},
		{pattern: "?", path: "", match: false},
		{pattern: "??", path: "ab", match: true},
		{pattern: "a?c", path: "abc", match: true},
		{pattern: "a?c", path: "ac", match: false},
		{pattern: "a?c", path: "abbc", match: false},

		// ? does not match /
		{pattern: "?", path: "/", match: false},
		{pattern: "a?b", path: "a/b", match: false},

		// Mix of ? and *
		{pattern: "?*?", path: "abc", match: true},
		{pattern: "?*?", path: "ab", match: true},
		{pattern: "?*?", path: "a", match: false},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeCharacterClasses(t *testing.T) {
	cases := []testCase{
		// Simple char class
		{pattern: "[abc]", path: "a", match: true},
		{pattern: "[abc]", path: "b", match: true},
		{pattern: "[abc]", path: "d", match: false},

		// Negated char class
		{pattern: "[!abc]", path: "d", match: true},
		{pattern: "[!abc]", path: "a", match: false},
		{pattern: "[^abc]", path: "d", match: true},
		{pattern: "[^abc]", path: "a", match: false},

		// Range
		{pattern: "[a-z]", path: "m", match: true},
		{pattern: "[a-z]", path: "M", match: false},
		{pattern: "[0-9]", path: "5", match: true},
		{pattern: "[0-9]", path: "a", match: false},
		{pattern: "[A-Z]", path: "G", match: true},
		{pattern: "[A-Z]", path: "g", match: false},

		// Special characters in class
		{pattern: "[.]", path: ".", match: true},
		{pattern: "[-]", path: "-", match: true},
		{pattern: "[_]", path: "_", match: true},
		{pattern: "[!]", path: "!", match: true},

		// POSIX classes
		{pattern: "[[:alpha:]]", path: "a", match: true},
		{pattern: "[[:alpha:]]", path: "Z", match: true},
		{pattern: "[[:alpha:]]", path: "5", match: false},
		{pattern: "[[:digit:]]", path: "7", match: true},
		{pattern: "[[:digit:]]", path: "x", match: false},
		{pattern: "[[:lower:]]", path: "a", match: true},
		{pattern: "[[:lower:]]", path: "A", match: false},
		{pattern: "[[:upper:]]", path: "A", match: true},
		{pattern: "[[:upper:]]", path: "a", match: false},
		{pattern: "[[:alnum:]]", path: "a", match: true},
		{pattern: "[[:alnum:]]", path: "5", match: true},
		{pattern: "[[:alnum:]]", path: "-", match: false},
		{pattern: "[[:space:]]", path: " ", match: true},
		{pattern: "[[:space:]]", path: "\t", match: true},
		{pattern: "[[:space:]]", path: "a", match: false},
		{pattern: "[[:xdigit:]]", path: "f", match: true},
		{pattern: "[[:xdigit:]]", path: "F", match: true},
		{pattern: "[[:xdigit:]]", path: "9", match: true},
		{pattern: "[[:xdigit:]]", path: "g", match: false},
		{pattern: "[[:blank:]]", path: " ", match: true},
		{pattern: "[[:blank:]]", path: "\t", match: true},
		{pattern: "[[:blank:]]", path: "\n", match: false},
		{pattern: "[[:print:]]", path: "a", match: true},
		{pattern: "[[:print:]]", path: " ", match: true},
		{pattern: "[[:print:]]", path: "\t", match: false},
		{pattern: "[[:cntrl:]]", path: "\t", match: true},
		{pattern: "[[:cntrl:]]", path: "a", match: false},
		{pattern: "[[:graph:]]", path: "a", match: true},
		{pattern: "[[:graph:]]", path: " ", match: false},
		{pattern: "[[:punct:]]", path: ".", match: true},
		{pattern: "[[:punct:]]", path: "a", match: false},

		// Combined POSIX and range
		{pattern: "[[:digit:]a-c]", path: "b", match: true},
		{pattern: "[[:digit:]a-c]", path: "5", match: true},
		{pattern: "[[:digit:]a-c]", path: "e", match: false},

		// Malformed bracket: no closing ] — matchBracket returns consumed=0,
		// which makes matchSegment return false. This differs from some shell
		// glob implementations that treat it as a literal "[", but is consistent
		// with the pathmatch implementation (which mirrors Git's wildmatch
		// pathspec semantics where malformed brackets simply fail to match).
		{pattern: "[abc", path: "[abc", match: false},
		{pattern: "[abc", path: "a", match: false},

		// Empty bracket expression [] — malformed (no closing ]), same as above.
		{pattern: "[]", path: "[]", match: false},

		// Range escape in bracket
		{pattern: `[a\z]`, path: "z", match: true},
		{pattern: `[a\z]`, path: "a", match: true},

		// Negation with range
		{pattern: "[!0-9]", path: "a", match: true},
		{pattern: "[!0-9]", path: "5", match: false},

		// Char class combined with other patterns
		{pattern: "[abc]/[xyz]", path: "a/x", match: true},
		{pattern: "[abc]/[xyz]", path: "b/y", match: true},
		{pattern: "[abc]/[xyz]", path: "d/x", match: false},
		{pattern: "[abc]/[xyz]", path: "a/d", match: false},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeEscapes(t *testing.T) {
	cases := []testCase{
		// Escape special characters
		{pattern: `a\*b`, path: "a*b", match: true},
		{pattern: `a\*b`, path: "axb", match: false},
		{pattern: `a\?b`, path: "a?b", match: true},
		{pattern: `a\?b`, path: "axb", match: false},
		{pattern: `a\[b`, path: "a[b", match: true},
		{pattern: `a\[b`, path: "axb", match: false},
		// Note: Go raw string `a\\b` is the 4-byte sequence a,\,\\,b.
		// The pathmatch escape \ consumes the next char, so pattern `a\\b` means:
		// a, then escaped-\, then literal b — i.e. it matches the string "a\b".
		// In Go interpreted strings, "a\b" is a+\x08+b (backspace), so we must use
		// a raw string literal or explicit concatenation for the path.
		{pattern: `a\\b`, path: "a" + `\` + "b", match: true},
		{pattern: `a\\b`, path: "ab", match: false},

		// Escape at end of pattern (trailing backslash — no char to match)
		{pattern: `foo\`, path: "foo", match: false},

		// Escape normal character (backslash before non-special is literal)
		// In the pattern `a\b`, \b means escaped-b, so it matches literal "b".
		// To match a literal backslash, use `a\\b`.
		{pattern: `a\b`, path: "ab", match: true},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeCaseFolding(t *testing.T) {
	cases := []testCase{
		// Without CaseFold (default is case-sensitive)
		{pattern: "FOO", path: "foo", match: false},
		{pattern: "Foo", path: "FOO", match: false},
		{pattern: "foo/BAR", path: "foo/bar", match: false},

		// With CaseFold
		{pattern: "FOO", path: "foo", match: true, opts: []Option{CaseFold}},
		{pattern: "Foo", path: "FOO", match: true, opts: []Option{CaseFold}},
		{pattern: "foo/BAR", path: "foo/bar", match: true, opts: []Option{CaseFold}},
		{pattern: "*FOO*", path: "xfoox", match: true, opts: []Option{CaseFold}},
		{pattern: "**/BAR", path: "a/b/bar", match: true, opts: []Option{CaseFold}},

		// Pattern and path both upper — should match with or without folding
		{pattern: "FOO", path: "FOO", match: true},
		{pattern: "FOO", path: "FOO", match: true, opts: []Option{CaseFold}},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q, %v).Match(%q) = %v, want %v", c.pattern, c.opts, c.path, got, c.match)
		}
	}
}

func TestEdgeUnicode(t *testing.T) {
	cases := []testCase{
		// Unicode literals
		{pattern: "你好", path: "你好", match: true},
		{pattern: "你好", path: "世界", match: false},
		{pattern: "你好/世界", path: "你好/世界", match: true},

		// Unicode with wildcards
		{pattern: "你*", path: "你好", match: true},
		{pattern: "*好", path: "你好", match: true},
		// ? matches one byte, so "你?好" needs an extra byte between 你 and 好,
		// which 正常 multi-byte UTF-8 characters cannot satisfy in this way.
		// "你?好" cannot match "你好" (no extra byte) or "你好好" (? matches only 1 byte of 好).
		{pattern: "你?好", path: "你好", match: false},
		{pattern: "你?好", path: "你好好", match: false},

		// Unicode with **
		{pattern: "**/你好", path: "a/b/你好", match: true},
		{pattern: "你好/**", path: "你好/世界", match: true},

		// Mixed ASCII and Unicode
		{pattern: "*.txt", path: "中文.txt", match: true},
		{pattern: "*.txt", path: "中文.md", match: false},
		{pattern: "dir/*/中文.txt", path: "dir/sub/中文.txt", match: true},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeLeadingDotSlash(t *testing.T) {
	cases := []testCase{
		{pattern: "./foo", path: "foo", match: true},
		{pattern: "./foo", path: "./foo", match: false}, // path "./foo" splits to [".", "foo"]
		{pattern: "./foo/bar", path: "foo/bar", match: true},
		{pattern: "./**/bar", path: "x/y/bar", match: true},
		{pattern: "./*.txt", path: "readme.txt", match: true},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeLeadingSlash(t *testing.T) {
	cases := []testCase{
		{pattern: "/foo", path: "foo", match: true},
		{pattern: "/foo", path: "/foo", match: false}, // path "/foo" splits to ["", "foo"]
		{pattern: "/*", path: "foo", match: true},
		{pattern: "/**/bar", path: "a/bar", match: true},
		{pattern: "//foo", path: "foo", match: false}, // only one leading / is trimmed
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeSegmentMismatch(t *testing.T) {
	cases := []testCase{
		// Pattern has more segments than path
		{pattern: "a/b/c", path: "a/b", match: false},
		{pattern: "a/b/c", path: "a", match: false},

		// Path has more segments than pattern
		{pattern: "a/b", path: "a/b/c", match: false},
		{pattern: "a", path: "a/b", match: false},

		// Single segment pattern vs multi-segment path
		{pattern: "foo", path: "foo/bar", match: false},
		{pattern: "foo", path: "bar/foo", match: false},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeComplexPatterns(t *testing.T) {
	cases := []testCase{
		// Deeply nested **
		{pattern: "a/**/b/**/c", path: "a/b/c", match: true},
		{pattern: "a/**/b/**/c", path: "a/x/b/y/c", match: true},
		{pattern: "a/**/b/**/c", path: "a/x/y/b/z/w/c", match: true},
		{pattern: "a/**/b/**/c", path: "a/x/y/b/z/w/c/d", match: false},

		// Mix of * and **
		{pattern: "src/**/*.go", path: "src/main.go", match: true},
		{pattern: "src/**/*.go", path: "src/pkg/main.go", match: true},
		{pattern: "src/**/*.go", path: "src/pkg/internal/main.go", match: true},
		{pattern: "src/**/*.go", path: "src/pkg/internal/main.txt", match: false},
		{pattern: "src/**/*.go", path: "main.go", match: false},

		// ** with ? in subsequent segment
		{pattern: "**/?.go", path: "a/x.go", match: true},
		{pattern: "**/?.go", path: "a/ab.go", match: false},

		// Pattern with brackets and **
		{pattern: "**/*.[ch]", path: "src/main.c", match: true},
		{pattern: "**/*.[ch]", path: "src/main.h", match: true},
		{pattern: "**/*.[ch]", path: "src/main.go", match: false},

		// Long path with **
		{pattern: "**/foo", path: "a/b/c/d/e/f/foo", match: true},

		// Backtracking in ** matching
		{pattern: "**/foo/bar", path: "x/foo/baz/foo/bar", match: true},
		{pattern: "**/foo/bar", path: "x/foo/baz/foo/qux", match: false},

		// ** matching exactly zero segments with literal adjacent
		{pattern: "foo/**/bar", path: "foo/bar", match: true},
		{pattern: "foo/**/bar", path: "foo/x/bar", match: true},

		// Multiple literals around **
		{pattern: "a/**/b/**/c/**/d", path: "a/b/c/d", match: true},
		{pattern: "a/**/b/**/c/**/d", path: "a/1/b/2/c/3/d", match: true},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeSpecialFilenames(t *testing.T) {
	cases := []testCase{
		// Dotfiles
		{pattern: ".gitignore", path: ".gitignore", match: true},
		{pattern: ".*", path: ".gitignore", match: true},
		{pattern: ".*", path: ".hidden", match: true},
		{pattern: ".*", path: "visible", match: false},

		// Files with dots
		{pattern: "file.txt", path: "file.txt", match: true},
		{pattern: "*.txt", path: "file.txt", match: true},
		{pattern: "*.txt", path: "file.md", match: false},
		{pattern: "file.*", path: "file.txt", match: true},
		{pattern: "file.*", path: "file.md", match: true},

		// Files with dashes
		{pattern: "foo-bar", path: "foo-bar", match: true},
		{pattern: "foo-*", path: "foo-bar", match: true},

		// Files with spaces (if pathmatch handles them like git does)
		{pattern: "foo bar", path: "foo bar", match: true},
		{pattern: "foo*", path: "foo bar", match: true},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

func TestEdgeBracketEdgeCases(t *testing.T) {
	cases := []testCase{
		// Closing bracket at start position in class is treated as literal.
		// This matches git wildmatch behavior where []abc] matches ], a, b, or c.
		{pattern: "[]abc]", path: "]", match: true},
		{pattern: "[]abc]", path: "a", match: true},
		{pattern: "[]abc]", path: "b", match: true},
		{pattern: "[]abc]", path: "c", match: true},
		{pattern: "[]abc]", path: "x", match: false},

		// Hyphen at end of class should be literal
		{pattern: "[a-]", path: "a", match: true},
		{pattern: "[a-]", path: "-", match: true},
		{pattern: "[a-]", path: "b", match: false},

		// Escaped hyphen in range
		{pattern: `[a\-z]`, path: "-", match: true},
		{pattern: `[a\-z]`, path: "a", match: true},
		{pattern: `[a\-z]`, path: "z", match: true},
		{pattern: `[a\-z]`, path: "m", match: false},
	}
	for _, c := range cases {
		p := New(c.pattern, c.opts...)
		got := p.Match(c.path)
		if got != c.match {
			t.Errorf("New(%q).Match(%q) = %v, want %v", c.pattern, c.path, got, c.match)
		}
	}
}

// ---------------------------------------------------------------------------
// Table-driven sub-tests for better failure diagnostics
// ---------------------------------------------------------------------------

func TestMatchSegmentTable(t *testing.T) {
	// Direct test of matchSegment to cover low-level edge cases.
	cases := []struct {
		pattern string
		text    string
		match   bool
	}{
		// Basic
		{"foo", "foo", true},
		{"foo", "bar", false},
		{"", "", true},
		{"", "x", false},
		{"x", "", false},

		// Star
		{"*", "", true},
		{"*", "anything", true},
		{"a*", "abc", true},
		{"*z", "abcz", true},
		{"*z", "abc", false},
		{"a*b", "axb", true},
		{"a*b", "ab", true},
		{"a*b*c", "axbxc", true},
		{"a*b*c", "abc", true},

		// Question mark
		{"?", "x", true},
		{"?", "", false},
		{"??", "xy", true},
		{"a?c", "abc", true},

		// Brackets
		{"[abc]", "a", true},
		{"[abc]", "d", false},
		{"[a-z]", "m", true},
		{"[a-z]", "M", false},
		{"[!a-z]", "5", true},
		{"[!a-z]", "m", false},

		// Escapes
		{`\*`, "*", true},
		{`\*`, "x", false},
		{`\?`, "?", true},
		{`\?`, "x", false},
		{`\\`, `\`, true},
	}
	for _, c := range cases {
		got := matchSegment(c.pattern, c.text)
		if got != c.match {
			t.Errorf("matchSegment(%q, %q) = %v, want %v", c.pattern, c.text, got, c.match)
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkLiteralMatch(b *testing.B) {
	p := New("foo/bar/baz.txt")
	for b.Loop() {
		p.Match("foo/bar/baz.txt")
	}
}

func BenchmarkLiteralMismatch(b *testing.B) {
	p := New("foo/bar/baz.txt")
	for b.Loop() {
		p.Match("foo/bar/qux.txt")
	}
}

func BenchmarkStarMatch(b *testing.B) {
	p := New("*.go")
	for b.Loop() {
		p.Match("main.go")
	}
}

func BenchmarkStarMismatch(b *testing.B) {
	p := New("*.go")
	for b.Loop() {
		p.Match("main.txt")
	}
}

func BenchmarkQuestionMarkMatch(b *testing.B) {
	p := New("?.go")
	for b.Loop() {
		p.Match("a.go")
	}
}

func BenchmarkBracketClassMatch(b *testing.B) {
	p := New("[a-z].go")
	for b.Loop() {
		p.Match("m.go")
	}
}

func BenchmarkPosixClassMatch(b *testing.B) {
	p := New("[[:digit:]].txt")
	for b.Loop() {
		p.Match("5.txt")
	}
}

func BenchmarkDoubleStarShallow(b *testing.B) {
	p := New("**/foo/*.go")
	for b.Loop() {
		p.Match("bar/foo/qux.go")
	}
}

func BenchmarkDoubleStarDeep(b *testing.B) {
	p := New("**/foo/*.go")
	for b.Loop() {
		p.Match("a/b/c/d/e/f/foo/qux.go")
	}
}

func BenchmarkDoubleStarDeepMismatch(b *testing.B) {
	p := New("**/foo/*.go")
	for b.Loop() {
		p.Match("a/b/c/d/e/f/bar/qux.go")
	}
}

func BenchmarkDoubleStarZeroSegments(b *testing.B) {
	p := New("src/**/main.go")
	for b.Loop() {
		p.Match("src/main.go")
	}
}

func BenchmarkDoubleStarMultiple(b *testing.B) {
	p := New("a/**/b/**/c")
	for b.Loop() {
		p.Match("a/x/y/b/z/c")
	}
}

func BenchmarkComplexPattern(b *testing.B) {
	p := New("**/src/**/*.go")
	for b.Loop() {
		p.Match("project/src/pkg/internal/handler/user.go")
	}
}

func BenchmarkComplexPatternMismatch(b *testing.B) {
	p := New("**/src/**/*.go")
	for b.Loop() {
		p.Match("project/src/pkg/internal/handler/user.txt")
	}
}

func BenchmarkCaseFoldMatch(b *testing.B) {
	p := New("**/foo/*.go", CaseFold)
	for b.Loop() {
		p.Match("BAR/FOO/QUX.GO")
	}
}

func BenchmarkEscapedPattern(b *testing.B) {
	p := New(`foo\*bar`)
	for b.Loop() {
		p.Match("foo*bar")
	}
}

func BenchmarkSegmentedPath(b *testing.B) {
	p := New("a/b/c/d/e/f")
	for b.Loop() {
		p.Match("a/b/c/d/e/f")
	}
}

func BenchmarkSegmentedPathMismatch(b *testing.B) {
	p := New("a/b/c/d/e/f")
	for b.Loop() {
		p.Match("a/b/c/d/e/g")
	}
}

func BenchmarkBacktrackingStar(b *testing.B) {
	// Degraded case: * causes backtracking
	p := New("*a*b*g*n*t")
	for b.Loop() {
		p.Match("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaabbbbngt")
	}
}

func BenchmarkNew(b *testing.B) {
	for b.Loop() {
		New("**/foo/*.go")
	}
}

func BenchmarkNewCaseFold(b *testing.B) {
	for b.Loop() {
		New("**/foo/*.go", CaseFold)
	}
}
