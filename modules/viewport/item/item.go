package item

import "regexp"

// Item defines the interface for item implementations
type Item interface {
	// Width returns the total width in terminal cells
	Width() int

	// Content returns the underlying complete string
	Content() string

	// ContentNoAnsi returns the underlying complete string without ANSI escape codes that style the string
	ContentNoAnsi() string

	// Take takes a substring (line) of the content with a specified widthToLeft and taking takeWidth.
	// continuation replaces the start and end if the content exceeds the bounds.
	// highlights is a list of highlights to apply to the taken content.
	// Returns the line and the actual width taken
	Take(
		widthToLeft,
		takeWidth int,
		continuation string,
		highlights []Highlight,
	) (string, int)

	// NumWrappedLines returns the number of wrapped lines given a wrap width
	NumWrappedLines(wrapWidth int) int

	// ExtractExactMatches extracts exact matches from the item's content without ANSI codes
	ExtractExactMatches(exactMatch string) []Match

	// ExtractRegexMatches extracts regex matches from the item's content without ANSI codes
	ExtractRegexMatches(regex *regexp.Regexp) []Match

	// ByteRangesToMatches converts byte ranges in the ANSI-stripped content to Matches
	// with both byte ranges and width ranges populated.
	ByteRangesToMatches(byteRanges []ByteRange) []Match

	// LineBrokenItems returns the sub-items of this item, each rendered on a separate line.
	// For single-line items, returns a slice containing just self.
	// For multi-line items, returns one item per content line with line breaks between them.
	LineBrokenItems() []Item

	// repr returns a representation of the object as a string for debugging
	repr() string
}
