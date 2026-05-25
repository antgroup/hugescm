// Package pathmatch implements lightweight glob-style path matching for
// pathspecs (e.g. "git diff -- <path>", "git for-each-ref").
//
// It supports the same wildcard semantics as Git's wildmatch.c for the
// pathspec use-case, but avoids the heavy token-parse-tree construction used
// by the wildmatch package.  Paths are pre-split into segments and matched
// with a simple recursive algorithm derived from ignore/pattern.go.
//
// Supported patterns:
//   - *       matches any sequence of characters within a single path segment
//   - ?       matches any single character within a segment
//   - [..]    character class (literal, range a-z, POSIX class [:name:],
//     negation ^ or !)
//   - **      matches zero or more path segments (must appear as a
//     standalone segment between '/' separators or at the ends)
//   - \x      escape: treat the next character as a literal
package pathmatch

import (
	"strings"
)

// Option configures a Pattern during construction.
type Option func(*Pattern)

// CaseFold enables case-insensitive matching.
func CaseFold(p *Pattern) { p.caseFold = true }

// SystemCase enables CaseFold on platforms with case-insensitive
// filesystems (Windows, macOS); on other platforms it is a no-op.  The
// actual value is set by the build-tag–specific init files.
var SystemCase Option

// Pattern is a pre-parsed pathspec pattern.
type Pattern struct {
	segments      []string // path segments after splitting on '/'
	caseFold      bool
	isLiteral     bool   // true if pattern contains no wildcards
	literalPath   string // the literal path if isLiteral is true
	hasDoubleStar bool   // true if pattern contains ** anywhere
}

// doubleStar is the "**" segment that matches zero or more path levels.
// Written as a named constant rather than a literal to avoid editor/linter
// warnings about glob-specific syntax.
const doubleStar = "**"

// New parses pattern and returns a ready-to-use *Pattern.
//
// The pattern is split on '/' into segments.  Segments equal to "**" are
// recognised as double-star (zero-or-more directory levels).  All other
// segments are treated as segment-level glob patterns and matched by
// matchSegment.
func New(pattern string, opts ...Option) *Pattern {
	p := &Pattern{}
	for _, o := range opts {
		o(p)
	}
	if p.caseFold {
		pattern = strings.ToLower(pattern)
	}
	// Trim leading "./" if present; it is not meaningful for matching.
	pattern = strings.TrimPrefix(pattern, "./")
	// A leading '/' in the pattern is not meaningful for pathspec
	// matching (repository paths are always relative).  Trim it so
	// that "/foo" and "foo" match the same paths, matching the
	// behaviour of Git's wildmatch and the original wildmatch package
	// (which skips empty leading segments in parseTokensSimple).
	pattern = strings.TrimPrefix(pattern, "/")
	p.segments = splitPattern(pattern)

	// Pre-compute optimization flags
	p.isLiteral = true
	p.hasDoubleStar = false
	for _, seg := range p.segments {
		if isStandaloneDoubleStar(seg) || hasEmbeddedDoubleStar(seg) {
			p.hasDoubleStar = true
			p.isLiteral = false
		} else if containsWildcard(seg) {
			p.isLiteral = false
		}
	}
	if p.isLiteral {
		p.literalPath = strings.Join(p.segments, "/")
	}

	return p
}

// splitPattern splits pattern on '/' into segments.
// Segments equal to "**" are recognised as double-star (zero-or-more
// directory levels). Segments containing embedded "**" (like "foo**bar")
// are kept as-is and handled specially during matching.
func splitPattern(pattern string) []string {
	return strings.Split(pattern, "/")
}

// isStandaloneDoubleStar reports whether s is exactly "**".
func isStandaloneDoubleStar(s string) bool {
	return s == "**"
}

// hasEmbeddedDoubleStar reports whether s contains "**" but is not exactly "**".
// For example, "foo**bar" has embedded double-star.
func hasEmbeddedDoubleStar(s string) bool {
	return s != "**" && strings.Contains(s, "**")
}

// containsWildcard reports whether s contains any wildcard characters.
// Any backslash means the segment is not a pure literal (needs escape processing).
func containsWildcard(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		// Any backslash means this is not a pure literal
		if c == '\\' {
			return true
		}
		if c == '*' || c == '?' || c == '[' {
			return true
		}
	}
	return false
}

// Match reports whether name matches the pattern.
//
// A trailing '/' in name is stripped before matching; pathspec matching
// does not use the gitignore "Contents" semantics where matching a directory
// implies matching its children.
func (p *Pattern) Match(name string) bool {
	if p.caseFold {
		name = strings.ToLower(name)
	}
	// Normalise: remove trailing slash (produced by some callers for
	// directory paths) so that "foo/bar/" matches the same as "foo/bar".
	// A trailing-slash-only path ("/") becomes "" which Split turns into
	// [""] — a single empty segment representing the root, matching
	// correctly against patterns like "" or "**".
	name = strings.TrimRight(name, "/")

	// Fast path: literal match (no wildcards)
	if p.isLiteral {
		return name == p.literalPath
	}

	segments := strings.Split(name, "/")
	return p.matchPaths(segments, 0, 0)
}

// matchPaths recursively matches path segments against pattern segments.
func (p *Pattern) matchPaths(segs []string, patIdx, segIdx int) bool {
	for {
		// Both exhausted → match.
		if patIdx == len(p.segments) && segIdx == len(segs) {
			return true
		}
		// Pattern exhausted but path remains → no match.
		if patIdx == len(p.segments) {
			return false
		}

		seg := p.segments[patIdx]

		// Handle standalone "**" — matches zero or more path segments.
		if isStandaloneDoubleStar(seg) {
			// Skip consecutive "**" segments (they're redundant).
			for patIdx < len(p.segments) && isStandaloneDoubleStar(p.segments[patIdx]) {
				patIdx++
			}
			// "**" at the end matches everything remaining.
			if patIdx == len(p.segments) {
				return true
			}
			// Try consuming 0..N path segments, then continue matching.
			for i := segIdx; i <= len(segs); i++ {
				if p.matchPaths(segs, patIdx, i) {
					return true
				}
			}
			return false
		}

		// Path exhausted but pattern still has non-"**" segments → no match.
		if segIdx == len(segs) {
			return false
		}

		// Check if this pattern segment contains embedded "**" (e.g., "foo**bar").
		// In this case, "**" can match across segments (including "/").
		if hasEmbeddedDoubleStar(seg) {
			// Try to match the pattern with cross-segment semantics.
			if matchSegmentWithDoubleStar(seg, segs, segIdx) {
				return true
			}
			return false
		}

		// Normal segment: match exactly one path segment.
		if !matchSegment(seg, segs[segIdx]) {
			return false
		}
		patIdx++
		segIdx++
	}
}

// ---------------------------------------------------------------------------
// Segment-level glob matching
// ---------------------------------------------------------------------------
//
// matchSegment is ported from ignore/pattern.go's matchPattern.  It matches a
// single path segment against a glob pattern that may contain *, ?, and [...].

// matchSegment reports whether the text matches the glob pattern within a
// single path segment.
func matchSegment(pattern, text string) bool {
	p := pattern
	for len(p) > 0 {
		pc := p[0]
		p = p[1:]

		if len(text) == 0 && pc != '*' {
			return false
		}

		switch pc {
		case '\\':
			if len(p) == 0 {
				return false
			}
			if len(text) == 0 || text[0] != p[0] {
				return false
			}
			p = p[1:]
			text = text[1:]

		case '?':
			if len(text) == 0 {
				return false
			}
			text = text[1:]

		case '*':
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			if len(p) == 0 {
				return true
			}
			if !isGlobSpecial(p[0]) {
				literal := p[0]
				for {
					idx := strings.IndexByte(text, literal)
					if idx < 0 {
						return false
					}
					if matchSegment(p, text[idx:]) {
						return true
					}
					text = text[idx+1:]
				}
			}
			for {
				if len(text) == 0 {
					return false
				}
				if matchSegment(p, text) {
					return true
				}
				text = text[1:]
			}

		case '[':
			if len(text) == 0 {
				return false
			}
			matched, consumed := matchBracket(p, text[0])
			if consumed == 0 || !matched {
				return false
			}
			p = p[consumed:]
			text = text[1:]

		default:
			if len(text) == 0 || text[0] != pc {
				return false
			}
			text = text[1:]
		}
	}
	return len(text) == 0
}

// matchBracket evaluates a bracket expression "[...]" starting just after the
// opening '['.  It returns whether the character ch matched the set, and the
// number of bytes consumed from pattern (up to and including the closing ']').
// If the bracket expression is malformed (no closing ']'), consumed is 0.
func matchBracket(p string, ch byte) (matched bool, consumed int) {
	if len(p) == 0 {
		return false, 0
	}

	negated := false
	rest := p
	if rest[0] == '!' || rest[0] == '^' {
		negated = true
		rest = rest[1:]
	}

	matched = false
	prev := byte(0)
	// first tracks if we are at the first character position after '[' or '[^!]'.
	// At the first position, ']' is treated as a literal character, not a closing bracket.
	// However, if the bracket is empty (like [!]), we need to handle it specially.
	first := true
	// empty tracks if we have seen any character in the set (for handling [!] case).
	empty := true

	for len(rest) > 0 {
		c := rest[0]
		rest = rest[1:]

		switch {
		case c == ']' && !first:
			// ']' closes the bracket expression only if it's not the first character.
			return matched != negated, len(p) - len(rest)

		case c == ']' && first:
			// ']' at the first position is a literal character.
			if ch == c {
				matched = true
			}
			// Don't set empty = false here because this is just a literal bracket,
			// not a "real" character in the set for the purpose of [!] detection.

		case c == '\\' && len(rest) > 0:
			c = rest[0]
			rest = rest[1:]
			if ch == c {
				matched = true
			}
			empty = false

		case c == '-' && prev != 0 && len(rest) > 0 && rest[0] != ']':
			end := rest[0]
			rest = rest[1:]
			if end == '\\' && len(rest) > 0 {
				end = rest[0]
				rest = rest[1:]
			}
			if ch >= prev && ch <= end {
				matched = true
			}
			prev = 0
			first = false
			empty = false
			continue

		case c == '[' && len(rest) > 1 && rest[0] == ':':
			closeIdx := strings.Index(rest[1:], ":]")
			if closeIdx < 0 {
				if ch == c {
					matched = true
				}
				prev = c
				first = false
				empty = false
				continue
			}
			name := rest[1 : 1+closeIdx]
			classEnd := 1 + closeIdx + 2
			if classBit, ok := posixClassName[name]; ok {
				if charClassTable[ch]&classBit != 0 {
					matched = true
				}
				rest = rest[classEnd:]
				prev = 0
				first = false
				empty = false
				continue
			}
			if ch == c {
				matched = true
			}
			prev = c
			first = false
			empty = false
			continue

		default:
			if ch == c {
				matched = true
			}
			empty = false
		}
		prev = c
		first = false
	}

	// If we reach here, there's no closing ']'.
	// Special case: [!] (negated empty set) should match any character.
	// This happens when the pattern is like "!]" (negation followed by literal ]).
	// We detect this by checking if empty is still true (no real characters were matched).
	if empty && negated {
		return true, len(p)
	}

	return false, 0
}

// isGlobSpecial reports whether c starts or modifies a wildcard sub-pattern.
func isGlobSpecial(c byte) bool {
	return c == '*' || c == '?' || c == '[' || c == '\\'
}

// matchSegmentWithDoubleStar matches a pattern containing "**" against
// one or more path segments. The "**" can match across segment boundaries
// (including "/"). This is used for patterns like "foo**bar" where **
// is embedded within a segment.
func matchSegmentWithDoubleStar(pattern string, segs []string, segIdx int) bool {
	// Join remaining segments with "/" to form the text to match against.
	var text string
	if segIdx < len(segs) {
		text = strings.Join(segs[segIdx:], "/")
	}
	return matchSegmentWithDoubleStarInternal(pattern, text)
}

// matchSegmentWithDoubleStarInternal matches pattern against text where "**"
// in the pattern can match any sequence of characters including "/".
func matchSegmentWithDoubleStarInternal(pattern, text string) bool {
	p := pattern
	t := text

	for len(p) > 0 {
		if len(p) >= 2 && p[0] == '*' && p[1] == '*' {
			// Found "**" - skip all consecutive "*" characters.
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			if len(p) == 0 {
				// "**" at end matches everything remaining.
				return true
			}
			// Try matching from each position in text.
			for i := 0; i <= len(t); i++ {
				if matchSegmentWithDoubleStarInternal(p, t[i:]) {
					return true
				}
			}
			return false
		}

		if len(t) == 0 {
			return false
		}

		pc := p[0]
		p = p[1:]

		switch pc {
		case '\\':
			if len(p) == 0 {
				return false
			}
			if t[0] != p[0] {
				return false
			}
			p = p[1:]
			t = t[1:]

		case '?':
			t = t[1:]

		case '*':
			// Single * - matches any sequence except "/".
			// But since we're in double-star mode, we need to handle this carefully.
			// Actually, single * should still not match "/" even in this context.
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}
			if len(p) == 0 {
				// Check if remaining text contains "/"
				return !strings.Contains(t, "/")
			}
			// Find next non-glob char to match.
			if !isGlobSpecial(p[0]) {
				literal := p[0]
				for {
					idx := strings.IndexByte(t, literal)
					if idx < 0 {
						return false
					}
					// Check that the match doesn't cross "/" boundary
					if !strings.Contains(t[:idx], "/") {
						if matchSegmentWithDoubleStarInternal(p, t[idx:]) {
							return true
						}
					}
					t = t[idx+1:]
				}
			}
			// Try each position, but don't cross "/" boundary.
			for len(t) > 0 {
				if matchSegmentWithDoubleStarInternal(p, t) {
					return true
				}
				if t[0] == '/' {
					return false
				}
				t = t[1:]
			}
			return false

		case '[':
			if len(t) == 0 {
				return false
			}
			matched, consumed := matchBracket(p, t[0])
			if consumed == 0 || !matched {
				return false
			}
			p = p[consumed:]
			t = t[1:]

		default:
			if t[0] != pc {
				return false
			}
			t = t[1:]
		}
	}

	return len(t) == 0
}

// ---------------------------------------------------------------------------
// POSIX character class bit-masks and lookup table
// ---------------------------------------------------------------------------

const (
	classAlnum  uint16 = 1 << iota // [:alnum:]
	classAlpha                     // [:alpha:]
	classBlank                     // [:blank:]
	classCntrl                     // [:cntrl:]
	classDigit                     // [:digit:]
	classGraph                     // [:graph:]
	classLower                     // [:lower:]
	classPrint                     // [:print:]
	classPunct                     // [:punct:]
	classSpace                     // [:space:]
	classUpper                     // [:upper:]
	classXdigit                    // [:xdigit:]
)

// posixClassName maps POSIX bracket-class names to their bit in charClassTable.
var posixClassName = map[string]uint16{
	"alnum":  classAlnum,
	"alpha":  classAlpha,
	"blank":  classBlank,
	"cntrl":  classCntrl,
	"digit":  classDigit,
	"graph":  classGraph,
	"lower":  classLower,
	"print":  classPrint,
	"punct":  classPunct,
	"space":  classSpace,
	"upper":  classUpper,
	"xdigit": classXdigit,
}

// charClassTable maps each byte to a bitmask of POSIX character classes.
// Classification is ASCII-only to mirror Git's sane-ctype.h: bytes with
// the high bit set never satisfy any class.
//
// Each value is the bitwise OR of the classXxx constants for the classes
// the byte belongs to.  For example, 'A' (0x41) is alpha|upper|alnum|graph|print,
// so the entry is classAlpha|classUpper|classAlnum|classGraph|classPrint = 2275.
var charClassTable = [256]uint16{
	8, 8, 8, 8, 8, 8, 8, 8, 8, 524, 520, 520, 520, 520, 8, 8,
	8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8, 8,
	644, 416, 416, 416, 416, 416, 416, 416, 416, 416, 416, 416, 416, 416, 416, 416,
	2225, 2225, 2225, 2225, 2225, 2225, 2225, 2225, 2225, 2225, 416, 416, 416, 416, 416, 416,
	416, 3235, 3235, 3235, 3235, 3235, 3235, 1187, 1187, 1187, 1187, 1187, 1187, 1187, 1187, 1187,
	1187, 1187, 1187, 1187, 1187, 1187, 1187, 1187, 1187, 1187, 1187, 416, 416, 416, 416, 416,
	416, 2275, 2275, 2275, 2275, 2275, 2275, 227, 227, 227, 227, 227, 227, 227, 227, 227,
	227, 227, 227, 227, 227, 227, 227, 227, 227, 227, 227, 416, 416, 416, 416, 8,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
	0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
}
