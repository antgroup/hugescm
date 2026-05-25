package ignore

import (
	"slices"
	"strings"
)

// MatchResult defines outcomes of a match, no match, exclusion or inclusion.
type MatchResult int

const (
	// NoMatch defines the no match outcome of a match check
	NoMatch MatchResult = iota
	// Exclude defines an exclusion of a file as a result of a match check
	Exclude
	// Include defines an explicit inclusion of a file as a result of a match check
	Include
)

const (
	inclusionPrefix = "!"
	zeroToManyDirs  = "**"
	patternDirSep   = "/"
)

// Pattern defines a single gitignore pattern.
type Pattern interface {
	// Match matches the given path to the pattern.
	Match(path []string, isDir bool) MatchResult
}

type pattern struct {
	domain    []string
	pattern   []string
	inclusion bool
	dirOnly   bool
	isGlob    bool
}

// ParsePattern parses a gitignore pattern string into the Pattern structure.
func ParsePattern(p string, domain []string) Pattern {
	// storing domain, copy it to ensure it isn't changed externally
	domain = slices.Clone(domain)
	res := pattern{domain: domain}

	if strings.HasPrefix(p, inclusionPrefix) {
		res.inclusion = true
		p = p[1:]
	}

	if !strings.HasSuffix(p, "\\ ") {
		p = strings.TrimRight(p, " ")
	}

	if strings.HasSuffix(p, patternDirSep) {
		res.dirOnly = true
		p = p[:len(p)-1]
	}

	if strings.Contains(p, patternDirSep) {
		res.isGlob = true
	}

	res.pattern = strings.Split(p, patternDirSep)
	return &res
}

func (p *pattern) Match(path []string, isDir bool) MatchResult {
	if len(path) <= len(p.domain) {
		return NoMatch
	}
	for i, e := range p.domain {
		if path[i] != e {
			return NoMatch
		}
	}

	path = path[len(p.domain):]

	var matched bool
	if p.isGlob {
		matched = p.globMatch(path, isDir)
	} else {
		matched = p.simpleNameMatch(path, isDir)
	}
	if !matched {
		return NoMatch
	}
	if p.inclusion {
		return Include
	}
	return Exclude
}

// ---------------------------------------------------------------------------
// Wildcard pattern matching (single segment)
// ---------------------------------------------------------------------------
//
// matchPattern is a Go-idiomatic port of Git's wildmatch.c (v2.54.0).  Paths
// are pre-split on '/', so the matcher never sees '/' — only single-segment
// pattern/text pairs are matched here.  The '*' matches any sequence of
// characters (including empty); '?' matches exactly one character; bracket
// expressions "[...]" and POSIX classes "[:name:]" are supported.
//
// Unlike the C original which used integer return codes (WM_MATCH, WM_NO_MATCH,
// WM_ABORT_ALL, WM_ABORT_TO_STARSTAR) to propagate recursion pruning hints,
// this Go version returns a plain bool.  The ABORT codes were an optimisation
// for the full-path wildmatch case (to avoid retrying '*' across '/' boundaries);
// since paths are already split, those hints are unnecessary.

// matchPattern reports whether text matches the wildcard pattern.
func matchPattern(pattern, text string) bool {
	p := pattern
	for len(p) > 0 {
		pc := p[0]
		p = p[1:]

		// When text is exhausted, only a trailing '*' can still match.
		if len(text) == 0 && pc != '*' {
			return false
		}

		switch pc {
		case '\\':
			// Literal match with the following character.  A trailing '\' has
			// no character to escape — the compare fails.
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
			// Collapse consecutive '*' into a single token.  Within a
			// pre-split segment both '*' and '**' match any characters.
			for len(p) > 0 && p[0] == '*' {
				p = p[1:]
			}

			// Trailing star matches everything remaining.
			if len(p) == 0 {
				return true
			}

			// Optimisation: when the star is followed by a literal character,
			// skip ahead in text to the next occurrence of that character
			// instead of trying every suffix position.
			if !isGlobSpecial(p[0]) {
				literal := p[0]
				for {
					idx := strings.IndexByte(text, literal)
					if idx < 0 {
						return false
					}
					if matchPattern(p, text[idx:]) {
						return true
					}
					text = text[idx+1:]
				}
			}

			// General case: try matching the remaining pattern at every
			// position in text.
			for {
				if len(text) == 0 {
					return false
				}
				if matchPattern(p, text) {
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

	// Check for negation.
	negated := false
	rest := p
	if rest[0] == '!' || rest[0] == '^' {
		negated = true
		rest = rest[1:]
	}

	matched = false
	prev := byte(0)

	for len(rest) > 0 {
		c := rest[0]
		rest = rest[1:]

		switch {
		case c == '\\' && len(rest) > 0:
			c = rest[0]
			rest = rest[1:]
			if ch == c {
				matched = true
			}

		case c == '-' && prev != 0 && len(rest) > 0 && rest[0] != ']':
			// Character range: prev - end.
			end := rest[0]
			rest = rest[1:]
			if end == '\\' && len(rest) > 0 {
				end = rest[0]
				rest = rest[1:]
			}
			if ch >= prev && ch <= end {
				matched = true
			}
			// Reset prev so that a following range doesn't chain from end.
			prev = 0
			continue

		case c == '[' && len(rest) > 1 && rest[0] == ':':
			// POSIX character class [:name:].
			closeIdx := strings.Index(rest[1:], ":]")
			if closeIdx < 0 {
				// Not a valid class; treat '[' as a literal.
				if ch == c {
					matched = true
				}
				prev = c
				continue
			}
			name := rest[1 : 1+closeIdx]
			classEnd := 1 + closeIdx + 2 // skip ":]"
			if classBit, ok := posixClassName[name]; ok {
				if charClassTable[ch]&classBit != 0 {
					matched = true
				}
				rest = rest[classEnd:]
				prev = 0
				continue
			}
			// Unrecognised class name: treat as literal.
			if ch == c {
				matched = true
			}
			prev = c
			continue

		case c == ']':
			// End of bracket expression.
			return matched != negated, len(p) - len(rest)

		default:
			if ch == c {
				matched = true
			}
		}
		prev = c
	}

	// No closing ']' found: malformed bracket expression.
	return false, 0
}

// isGlobSpecial reports whether c starts or modifies a wildcard sub-pattern
// ('*', '?', '[', '\'); everything else is literal text.
func isGlobSpecial(c byte) bool {
	return c == '*' || c == '?' || c == '[' || c == '\\'
}

// POSIX character class bit-masks for charClassTable.
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

// ---------------------------------------------------------------------------
// Pattern matching methods
// ---------------------------------------------------------------------------

func (p *pattern) simpleNameMatch(path []string, isDir bool) bool {
	for i, name := range path {
		if matchPattern(p.pattern[0], name) {
			if p.dirOnly && !isDir && i == len(path)-1 {
				return false
			}
			return true
		}
	}
	return false
}

func (p *pattern) globMatch(path []string, isDir bool) bool {
	var matched bool
	var canTraverse bool
	var trailingStar bool

	for i, seg := range p.pattern {
		if seg == "" {
			canTraverse = false
			continue
		}

		if seg == zeroToManyDirs {
			if i == len(p.pattern)-1 {
				// Trailing ** matches everything remaining (if there's
				// something left or it's a directory).
				if len(path) > 0 || isDir {
					matched = true
					trailingStar = true
				} else {
					matched = false
				}
				break
			}
			canTraverse = true
			continue
		}

		// Patterns like "foo**bar" or "**bar" contain '**' but are not
		// exactly "**", so they are treated as regular wildcard patterns
		// and matchPattern will handle the embedded '**' within a
		// single segment.
		if len(path) == 0 {
			return false
		}

		if canTraverse {
			canTraverse = false
			if path, matched = tryTraverse(seg, path); !matched {
				return false
			}
			continue
		}

		if !matchPattern(seg, path[0]) {
			return false
		}
		matched = true
		path = path[1:]

		// All path segments consumed but there are still pattern
		// segments to satisfy.  A subsequent "**" may still rescue
		// the match; otherwise fail immediately.
		if len(path) == 0 && i < len(p.pattern)-1 && !slices.Contains(p.pattern[i+1:], zeroToManyDirs) {
			return false
		}
	}

	if matched && p.dirOnly && !isDir && (len(path) == 0 || trailingStar) {
		matched = false
	}
	return matched
}

// tryTraverse skips path elements until one matches seg, consuming it.
// It returns the remaining path and whether a match was found.
func tryTraverse(seg string, path []string) ([]string, bool) {
	for i, s := range path {
		if matchPattern(seg, s) {
			return path[i+1:], true
		}
	}
	return nil, false
}
