package item

import (
	"fmt"
	"regexp"
	"strings"
)

// MultiLineItem implements Item by wrapping multiple SingleItems, rendered with line breaks between them.
// Each individual SingleItem may span multiple terminal lines if it wraps, but the MultiLineItem itself does not
// concatenate and wrap content across items (for that, see ConcatItem).
// Take() must not be called on a MultiLineItem — callers should use Take() on individual items returned
// by LineBrokenItems() instead.
type MultiLineItem struct {
	items      []SingleItem
	totalWidth int    // sum of all item widths
	content    string // cached: item content joined with \n (with ANSI)
	noAnsi     string // cached: item content joined with \n (no ANSI)
}

// type assertion that MultiLineItem implements Item
var _ Item = MultiLineItem{}

// type assertion that *MultiLineItem implements Item
var _ Item = (*MultiLineItem)(nil)

// NewMultiLineItem creates a new MultiLineItem from the given items.
func NewMultiLineItem(items ...SingleItem) MultiLineItem {
	if len(items) == 0 {
		return MultiLineItem{}
	}

	totalWidth := 0
	for _, it := range items {
		totalWidth += it.Width()
	}

	return MultiLineItem{
		items:      items,
		totalWidth: totalWidth,
	}
}

// Width returns the total width in cells across all line-broken items.
func (m MultiLineItem) Width() int {
	return m.totalWidth
}

// Content returns the content of all items joined with newlines.
func (m MultiLineItem) Content() string {
	if m.content != "" {
		return m.content
	}
	if len(m.items) == 0 {
		return ""
	}
	if len(m.items) == 1 {
		return m.items[0].Content()
	}

	totalLen := 0
	for _, it := range m.items {
		totalLen += len(it.Content())
	}
	totalLen += len(m.items) - 1 // newline separators

	var builder strings.Builder
	builder.Grow(totalLen)
	for i, it := range m.items {
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(it.Content())
	}
	m.content = builder.String()
	return m.content
}

// ContentNoAnsi returns the content of all items joined with newlines, without ANSI codes.
func (m MultiLineItem) ContentNoAnsi() string {
	if m.noAnsi != "" {
		return m.noAnsi
	}
	if len(m.items) == 0 {
		return ""
	}
	if len(m.items) == 1 {
		return m.items[0].ContentNoAnsi()
	}

	totalLen := 0
	for _, it := range m.items {
		totalLen += len(it.ContentNoAnsi())
	}
	totalLen += len(m.items) - 1

	var builder strings.Builder
	builder.Grow(totalLen)
	for i, it := range m.items {
		if i > 0 {
			builder.WriteByte('\n')
		}
		builder.WriteString(it.ContentNoAnsi())
	}
	m.noAnsi = builder.String()
	return m.noAnsi
}

// NumWrappedLines returns the total number of terminal lines needed to render all
// line-broken items, where each item wraps independently.
func (m MultiLineItem) NumWrappedLines(wrapWidth int) int {
	if wrapWidth <= 0 {
		return 0
	}
	if len(m.items) == 0 {
		return 1
	}
	total := 0
	for _, it := range m.items {
		total += it.NumWrappedLines(wrapWidth)
	}
	return total
}

// Take must not be called on a MultiLineItem. Callers should render
// individual items returned by LineBrokenItems() instead.
func (m MultiLineItem) Take(
	_, _ int,
	_ string,
	_ []Highlight,
) (string, int) {
	panic("Take() called on MultiLineItem — use LineBrokenItems() to render individual lines")
}

// LineBrokenItems returns the individual items, each rendered on a separate line.
func (m MultiLineItem) LineBrokenItems() []Item {
	// convert MultiLineItem to Item
	items := make([]Item, len(m.items))
	for i := range m.items {
		items[i] = m.items[i]
	}
	return items
}

// repr returns a string representation of the MultiLineItem for debugging.
func (m MultiLineItem) repr() string {
	var v strings.Builder
	v.WriteString("MultiLine(")
	for i := range m.items {
		if i > 0 {
			v.WriteString(", ")
		}
		v.WriteString(m.items[i].repr())
	}
	v.WriteString(")")
	return v.String()
}

// ByteRangesToMatches converts byte ranges in the concatenated ANSI-stripped content to Matches.
func (m MultiLineItem) ByteRangesToMatches(byteRanges []ByteRange) []Match {
	if len(m.items) == 0 || len(byteRanges) == 0 {
		return nil
	}
	if len(m.items) == 1 {
		return m.items[0].ByteRangesToMatches(byteRanges)
	}

	lineByteOffsets, lineWidthOffsets := m.computeOffsets()
	matches := make([]Match, 0, len(byteRanges))
	for _, br := range byteRanges {
		startWidth, endWidth := m.byteRangeToWidthRange(br.Start, br.End, lineByteOffsets, lineWidthOffsets)
		matches = append(matches, Match{
			ByteRange:  br,
			WidthRange: WidthRange{Start: startWidth, End: endWidth},
		})
	}
	return matches
}

// ExtractExactMatches extracts exact matches from the concatenated content.
// Byte ranges are relative to ContentNoAnsi(). Width ranges are cumulative across items.
func (m MultiLineItem) ExtractExactMatches(exactMatch string) []Match {
	if len(m.items) == 0 || exactMatch == "" {
		return nil
	}
	if len(m.items) == 1 {
		return m.items[0].ExtractExactMatches(exactMatch)
	}

	concatenated := m.ContentNoAnsi()
	var byteRanges []ByteRange
	startIndex := 0
	for {
		foundIndex := strings.Index(concatenated[startIndex:], exactMatch)
		if foundIndex == -1 {
			break
		}
		actualStartIndex := startIndex + foundIndex
		endIndex := actualStartIndex + len(exactMatch)
		byteRanges = append(byteRanges, ByteRange{Start: actualStartIndex, End: endIndex})
		startIndex = endIndex
	}
	return m.ByteRangesToMatches(byteRanges)
}

// ExtractRegexMatches extracts regex matches from the concatenated content.
func (m MultiLineItem) ExtractRegexMatches(regex *regexp.Regexp) []Match {
	if len(m.items) == 0 {
		return nil
	}
	if len(m.items) == 1 {
		return m.items[0].ExtractRegexMatches(regex)
	}

	concatenated := m.ContentNoAnsi()
	regexMatches := regex.FindAllStringIndex(concatenated, -1)
	if len(regexMatches) == 0 {
		return nil
	}

	byteRanges := make([]ByteRange, 0, len(regexMatches))
	for _, rm := range regexMatches {
		byteRanges = append(byteRanges, ByteRange{Start: rm[0], End: rm[1]})
	}
	return m.ByteRangesToMatches(byteRanges)
}

// computeOffsets returns cumulative byte offsets and width offsets for each line-broken item.
// Byte offsets account for the \n separators between items in the concatenated content.
func (m MultiLineItem) computeOffsets() (lineByteOffsets, lineWidthOffsets []int) {
	lineByteOffsets = make([]int, len(m.items)+1)
	lineWidthOffsets = make([]int, len(m.items)+1)
	for i, it := range m.items {
		lineByteOffsets[i+1] = lineByteOffsets[i] + len(it.ContentNoAnsi())
		if i < len(m.items)-1 {
			lineByteOffsets[i+1]++ // \n separator
		}
		lineWidthOffsets[i+1] = lineWidthOffsets[i] + it.Width()
	}
	return
}

// findLineForByteOffset finds which line-broken item contains the given byte offset
// in the concatenated content. Returns (lineIndex, localByteOffset).
func (m MultiLineItem) findLineForByteOffset(byteOffset int, lineByteOffsets []int) (int, int) {
	for i := 0; i < len(m.items); i++ {
		lineStart := lineByteOffsets[i]
		lineEnd := lineByteOffsets[i] + len(m.items[i].ContentNoAnsi())
		if byteOffset >= lineStart && byteOffset < lineEnd {
			return i, byteOffset - lineStart
		}
		// byteOffset falls on the \n separator — attribute to the next line
		if i < len(m.items)-1 && byteOffset == lineEnd {
			return i + 1, 0
		}
	}
	// past the end
	lastIdx := len(m.items) - 1
	return lastIdx, len(m.items[lastIdx].ContentNoAnsi())
}

// byteRangeToWidthRange converts a byte range in the concatenated content to a
// cumulative width range across line-broken items.
func (m MultiLineItem) byteRangeToWidthRange(
	startByte, endByte int,
	lineByteOffsets, lineWidthOffsets []int,
) (startWidth, endWidth int) {
	startLineIdx, startLocalByte := m.findLineForByteOffset(startByte, lineByteOffsets)
	endLineIdx, endLocalByte := m.findLineForByteOffset(endByte, lineByteOffsets)

	if startLineIdx >= 0 && startLineIdx < len(m.items) {
		startRuneIdx := m.items[startLineIdx].getRuneIndexAtByteOffset(startLocalByte)
		if startRuneIdx > 0 {
			startWidth = int(m.items[startLineIdx].getCumulativeWidthAtRuneIdx(startRuneIdx - 1))
		}
		startWidth += lineWidthOffsets[startLineIdx]
	}

	if endLineIdx >= 0 && endLineIdx < len(m.items) {
		endRuneIdx := m.items[endLineIdx].getRuneIndexAtByteOffset(endLocalByte)
		if endRuneIdx > 0 {
			endWidth = int(m.items[endLineIdx].getCumulativeWidthAtRuneIdx(endRuneIdx - 1))
		}
		endWidth += lineWidthOffsets[endLineIdx]
	}

	return
}

// NumLineBrokenItems returns the number of line-broken items.
func (m MultiLineItem) NumLineBrokenItems() int {
	return len(m.items)
}

// LineBrokenItem returns the line-broken item at the given index.
func (m MultiLineItem) LineBrokenItem(idx int) SingleItem {
	return m.items[idx]
}

// String returns the content for fmt.Stringer compatibility.
func (m MultiLineItem) String() string {
	return fmt.Sprintf("MultiLineItem{lines=%d, width=%d}", len(m.items), m.totalWidth)
}
