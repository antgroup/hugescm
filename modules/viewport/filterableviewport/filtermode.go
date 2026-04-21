package filterableviewport

import (
	"regexp"
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/antgroup/hugescm/modules/viewport/internal/fuzzy"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

// FilterModeName identifies a filter mode programmatically.
// Built-in names are provided as package constants.
// Define your own for custom filter modes.
type FilterModeName string

const (
	// FilterExact identifies the built-in exact substring filter mode.
	FilterExact FilterModeName = "exact"
	// FilterRegex identifies the built-in regex filter mode.
	FilterRegex FilterModeName = "regex"
	// FilterCaseInsensitive identifies the built-in case-insensitive regex filter mode.
	FilterCaseInsensitive FilterModeName = "iregex"
	// FilterFuzzy identifies the built-in fuzzy filter mode.
	FilterFuzzy FilterModeName = "fuzzy"
)

// MatchFunc extracts match byte ranges from ANSI-stripped item content.
// Called once per item during a filter scan.
type MatchFunc func(content string) []item.ByteRange

// FilterMode defines a user-configurable filter type.
type FilterMode struct {
	// Name is a stable programmatic identifier for this filter mode (e.g. FilterExact, FilterRegex).
	// Must be unique across all modes in a filterable viewport.
	Name FilterModeName
	// Key activates this filter mode
	Key key.Binding
	// Label shown in filter line, e.g. "[exact]"
	Label string
	// GetMatchFunc is called once when the filter text changes. It returns a MatchFunc
	// used for each item, or an error (e.g. invalid regex) to show no matches.
	GetMatchFunc func(filterText string) (MatchFunc, error)
}

// Matches reports whether content matches the given query according to this
// filter mode's matching logic.  It is a convenience wrapper around
// GetMatchFunc for callers that only need a boolean result.
func (fm FilterMode) Matches(query, content string) bool {
	if query == "" {
		return true
	}
	matchFn, err := fm.GetMatchFunc(query)
	if err != nil {
		return false
	}
	return len(matchFn(content)) > 0
}

// ExactFilterMode returns a FilterMode that performs exact substring matching.
func ExactFilterMode(k key.Binding) FilterMode {
	return FilterMode{
		Name:  FilterExact,
		Key:   k,
		Label: "[exact]",
		GetMatchFunc: func(filterText string) (MatchFunc, error) {
			return func(content string) []item.ByteRange {
				if filterText == "" {
					return nil
				}
				var ranges []item.ByteRange
				startIndex := 0
				for {
					foundIndex := strings.Index(content[startIndex:], filterText)
					if foundIndex == -1 {
						break
					}
					actualStart := startIndex + foundIndex
					end := actualStart + len(filterText)
					ranges = append(ranges, item.ByteRange{Start: actualStart, End: end})
					startIndex = end
				}
				return ranges
			}, nil
		},
	}
}

// RegexFilterMode returns a FilterMode that performs regex matching.
func RegexFilterMode(k key.Binding) FilterMode {
	return FilterMode{
		Name:  FilterRegex,
		Key:   k,
		Label: "[regex]",
		GetMatchFunc: func(filterText string) (MatchFunc, error) {
			re, err := regexp.Compile(filterText)
			if err != nil {
				return nil, err
			}
			return func(content string) []item.ByteRange {
				regexMatches := re.FindAllStringIndex(content, -1)
				if len(regexMatches) == 0 {
					return nil
				}
				ranges := make([]item.ByteRange, 0, len(regexMatches))
				for _, rm := range regexMatches {
					ranges = append(ranges, item.ByteRange{Start: rm[0], End: rm[1]})
				}
				return ranges
			}, nil
		},
	}
}

// CaseInsensitiveFilterMode returns a FilterMode that performs case-insensitive
// regex matching. The (?i) prefix is added internally — the user never sees it
// in the text input.
func CaseInsensitiveFilterMode(k key.Binding) FilterMode {
	return FilterMode{
		Name:  FilterCaseInsensitive,
		Key:   k,
		Label: "[iregex]",
		GetMatchFunc: func(filterText string) (MatchFunc, error) {
			re, err := regexp.Compile("(?i)" + filterText)
			if err != nil {
				return nil, err
			}
			return func(content string) []item.ByteRange {
				regexMatches := re.FindAllStringIndex(content, -1)
				if len(regexMatches) == 0 {
					return nil
				}
				ranges := make([]item.ByteRange, 0, len(regexMatches))
				for _, rm := range regexMatches {
					ranges = append(ranges, item.ByteRange{Start: rm[0], End: rm[1]})
				}
				return ranges
			}, nil
		},
	}
}

// FuzzyFilterMode returns a FilterMode that performs fuzzy matching similar to fzf.
// Characters in the query must appear in order in the content but need not be contiguous.
// Matching is case-insensitive. The highlighted range spans from the first to the last
// matched character.
func FuzzyFilterMode(k key.Binding) FilterMode {
	return FilterMode{
		Name:  FilterFuzzy,
		Key:   k,
		Label: "[fuzzy]",
		GetMatchFunc: func(filterText string) (MatchFunc, error) {
			return func(content string) []item.ByteRange {
				if filterText == "" {
					return nil
				}
				matches := fuzzy.Find([]string{content}, filterText)
				if len(matches) == 0 {
					return nil
				}
				fuzzyRanges := matches[0].MatchedByteRanges()
				if len(fuzzyRanges) == 0 {
					return nil
				}
				// Single span from first matched char to last matched char.
				return []item.ByteRange{{
					Start: fuzzyRanges[0].Start,
					End:   fuzzyRanges[len(fuzzyRanges)-1].End,
				}}
			}, nil
		},
	}
}

// DefaultFilterModes returns the default set of filter modes:
// exact (/), regex (r), case-insensitive (i).
func DefaultFilterModes() []FilterMode {
	return []FilterMode{
		ExactFilterMode(key.NewBinding(
			key.WithKeys("/"),
			key.WithHelp("/", "filter"),
		)),
		RegexFilterMode(key.NewBinding(
			key.WithKeys("r"),
			key.WithHelp("r", "regex filter"),
		)),
		CaseInsensitiveFilterMode(key.NewBinding(
			key.WithKeys("i"),
			key.WithHelp("i", "case insensitive filter"),
		)),
	}
}
