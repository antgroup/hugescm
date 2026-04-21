package item

import (
	"regexp"
	"strings"
)

// ConcatItem implements Item by wrapping multiple SingleItem's without extra memory allocation
// It is useful for e.g. prefixing content on an Item without needing to recompute that entire Item.
type ConcatItem struct {
	items         []SingleItem
	totalWidth    int    // cached total width across all items
	contentNoAnsi string // cached concatenated content without ANSI escape codes
	pinnedCount   int    // number of items to pin on the left (0 = no pinning)
	pinnedWidth   int    // cached total width of pinned items
}

// type assertion that ConcatItem implements Item
var _ Item = ConcatItem{}

// type assertion that *ConcatItem implements Item
var _ Item = (*ConcatItem)(nil)

// NewConcat creates a new ConcatItem from the given items
func NewConcat(items ...SingleItem) ConcatItem {
	return NewConcatWithPinned(0, items...)
}

// NewConcatWithPinned creates a new ConcatItem with the first pinnedCount items pinned to the left.
// Pinned items are not affected by horizontal panning (widthToLeft) in Take().
func NewConcatWithPinned(pinnedCount int, items ...SingleItem) ConcatItem {
	if len(items) == 0 {
		return ConcatItem{}
	}

	if pinnedCount < 0 {
		pinnedCount = 0
	}
	if pinnedCount > len(items) {
		pinnedCount = len(items)
	}

	totalWidth := 0
	pinnedWidth := 0
	for i, item := range items {
		w := item.Width()
		totalWidth += w
		if i < pinnedCount {
			pinnedWidth += w
		}
	}

	return ConcatItem{
		items:       items,
		totalWidth:  totalWidth,
		pinnedCount: pinnedCount,
		pinnedWidth: pinnedWidth,
	}
}

// Width returns the total width across all items.
func (m ConcatItem) Width() int {
	return m.totalWidth
}

// Content returns the concatenated content of all items.
func (m ConcatItem) Content() string {
	if len(m.items) == 0 {
		return ""
	}

	if len(m.items) == 1 {
		return m.items[0].Content()
	}

	totalLen := 0
	for _, items := range m.items {
		totalLen += len(items.Content())
	}

	var builder strings.Builder
	builder.Grow(totalLen)

	for _, item := range m.items {
		builder.WriteString(item.Content())
	}

	return builder.String()
}

// ContentNoAnsi returns the concatenated content of all items without ANSI escape codes that style the string
func (m ConcatItem) ContentNoAnsi() string {
	if m.contentNoAnsi != "" {
		return m.contentNoAnsi
	}

	if len(m.items) == 0 {
		return ""
	}

	if len(m.items) == 1 {
		m.contentNoAnsi = m.items[0].ContentNoAnsi()
		return m.contentNoAnsi
	}

	// make a single allocation for the concatenated string
	totalLen := 0
	for _, items := range m.items {
		totalLen += len(items.ContentNoAnsi())
	}
	var builder strings.Builder
	builder.Grow(totalLen)
	for _, item := range m.items {
		builder.WriteString(item.ContentNoAnsi())
	}
	m.contentNoAnsi = builder.String()
	return m.contentNoAnsi
}

// Take returns a substring of the item that fits within the specified width.
// If pinnedCount > 0, the first pinnedCount items are rendered at offset 0 (ignoring widthToLeft),
// and the remaining items are rendered with widthToLeft applied in the remaining viewport width.
func (m ConcatItem) Take(
	widthToLeft,
	takeWidth int,
	continuation string,
	highlights []Highlight,
) (string, int) {
	if len(m.items) == 0 {
		return "", 0
	}

	// for single item with no pinning, delegate directly
	if len(m.items) == 1 && m.pinnedCount == 0 {
		return m.items[0].Take(widthToLeft, takeWidth, continuation, highlights)
	}

	// if no pinned items, use standard logic
	if m.pinnedCount == 0 {
		return m.takeUnpinned(widthToLeft, takeWidth, continuation, highlights)
	}

	// handle pinned items (including single item that is pinned)
	return m.takePinned(widthToLeft, takeWidth, continuation, highlights)
}

// takeUnpinned is used when no items are pinned
func (m ConcatItem) takeUnpinned(
	widthToLeft,
	takeWidth int,
	continuation string,
	highlights []Highlight,
) (string, int) {
	if widthToLeft >= m.totalWidth {
		return "", 0
	}

	// find which item contains our start position
	skippedWidth := 0
	skippedBytes := 0
	firstItemIdx := 0
	firstByteIdx := 0
	startWidthFirstItem := widthToLeft

	for i := range m.items {
		itemWidth := m.items[i].Width()
		if skippedWidth+itemWidth > widthToLeft {
			firstItemIdx = i
			startWidthFirstItem = widthToLeft - skippedWidth

			runeIdx := m.items[i].findRuneIndexWithWidthToLeft(startWidthFirstItem)
			var firstItemByteIdx int
			if runeIdx < m.items[i].numNoAnsiRunes {
				firstItemByteIdx = int(m.items[i].getByteOffsetAtRuneIdx(runeIdx))
			} else {
				firstItemByteIdx = len(m.items[i].line)
			}
			firstByteIdx = skippedBytes + firstItemByteIdx
			break
		}
		skippedWidth += itemWidth
		skippedBytes += len(m.items[i].lineNoAnsi)
		startWidthFirstItem -= itemWidth
	}

	// take from first item
	res, takenWidth := m.items[firstItemIdx].Take(startWidthFirstItem, takeWidth, "", []Highlight{})
	remainingTotalWidth := takeWidth - takenWidth

	// if we have more width to take and more items available, continue
	currentItemIdx := firstItemIdx + 1
	for remainingTotalWidth > 0 && currentItemIdx < len(m.items) {
		nextPart, partWidth := m.items[currentItemIdx].Take(0, remainingTotalWidth, "", []Highlight{})
		if partWidth == 0 {
			break
		}
		res += nextPart
		remainingTotalWidth -= partWidth
		currentItemIdx++
	}

	res = highlightString(
		res,
		highlights,
		firstByteIdx,
		firstByteIdx+len(StripAnsi(res)),
	)

	// apply continuation indicators if needed
	if len(continuation) > 0 {
		contentToLeft := widthToLeft > 0
		contentToRight := m.totalWidth-widthToLeft > takeWidth-remainingTotalWidth
		if contentToLeft || contentToRight {
			continuationRunes := []rune(continuation)
			if contentToLeft {
				res = replaceStartWithContinuation(res, continuationRunes)
			}
			if contentToRight {
				res = replaceEndWithContinuation(res, continuationRunes)
			}
		}
	}

	res = removeEmptyAnsiSequences(res)
	return res, takeWidth - remainingTotalWidth
}

// takePinned handles rendering when there are pinned items
func (m ConcatItem) takePinned(
	widthToLeft,
	takeWidth int,
	continuation string,
	highlights []Highlight,
) (string, int) {
	// edge case: pinned width >= takeWidth (pinned items fill entire viewport)
	if m.pinnedWidth >= takeWidth {
		return m.takePinnedOnly(takeWidth, continuation, highlights)
	}

	// calculate available width for non-pinned content
	nonPinnedTakeWidth := takeWidth - m.pinnedWidth

	// render pinned items at offset 0
	pinnedResult, pinnedTaken := m.takePinnedItems(m.pinnedWidth, highlights)

	// render non-pinned items with the original widthToLeft
	nonPinnedResult, nonPinnedTaken := m.takeNonPinnedItems(
		widthToLeft,
		nonPinnedTakeWidth,
		continuation,
		highlights,
	)

	return pinnedResult + nonPinnedResult, pinnedTaken + nonPinnedTaken
}

// takePinnedItems renders just the pinned items at offset 0
func (m ConcatItem) takePinnedItems(takeWidth int, highlights []Highlight) (string, int) {
	if m.pinnedCount == 0 || takeWidth <= 0 {
		return "", 0
	}

	// take from pinned items
	var result strings.Builder
	remainingWidth := takeWidth

	for i := 0; i < m.pinnedCount && remainingWidth > 0; i++ {
		part, partWidth := m.items[i].Take(0, remainingWidth, "", []Highlight{})
		if partWidth == 0 {
			break
		}
		result.WriteString(part)
		remainingWidth -= partWidth
	}

	res := result.String()

	// calculate end byte for highlights (byte offset at end of pinned items)
	endByteIdx := 0
	for i := 0; i < m.pinnedCount; i++ {
		endByteIdx += len(m.items[i].lineNoAnsi)
	}

	// apply highlights to pinned section
	res = highlightString(
		res,
		highlights,
		0,
		min(endByteIdx, len(StripAnsi(res))),
	)

	return res, takeWidth - remainingWidth
}

// takeNonPinnedItems renders items after the pinned ones with the given offset
func (m ConcatItem) takeNonPinnedItems(
	widthToLeft,
	takeWidth int,
	continuation string,
	highlights []Highlight,
) (string, int) {
	if m.pinnedCount >= len(m.items) || takeWidth <= 0 {
		return "", 0
	}

	// calculate the byte offset where non-pinned content starts
	pinnedByteOffset := 0
	for i := 0; i < m.pinnedCount; i++ {
		pinnedByteOffset += len(m.items[i].lineNoAnsi)
	}

	// calculate total width of non-pinned items
	nonPinnedTotalWidth := m.totalWidth - m.pinnedWidth

	// if widthToLeft exceeds non-pinned content, return empty
	if widthToLeft >= nonPinnedTotalWidth {
		return "", 0
	}

	// find starting item and position within non-pinned items
	skippedWidth := 0
	skippedBytes := pinnedByteOffset
	firstItemIdx := m.pinnedCount
	startWidthFirstItem := widthToLeft

	for i := m.pinnedCount; i < len(m.items); i++ {
		itemWidth := m.items[i].Width()
		if skippedWidth+itemWidth > widthToLeft {
			firstItemIdx = i
			startWidthFirstItem = widthToLeft - skippedWidth

			runeIdx := m.items[i].findRuneIndexWithWidthToLeft(startWidthFirstItem)
			var firstItemByteIdx int
			if runeIdx < m.items[i].numNoAnsiRunes {
				firstItemByteIdx = int(m.items[i].getByteOffsetAtRuneIdx(runeIdx))
			} else {
				firstItemByteIdx = len(m.items[i].line)
			}
			skippedBytes += firstItemByteIdx
			break
		}
		skippedWidth += itemWidth
		skippedBytes += len(m.items[i].lineNoAnsi)
		startWidthFirstItem -= itemWidth
	}

	firstByteIdx := skippedBytes

	// take from first non-pinned item
	res, takenWidth := m.items[firstItemIdx].Take(startWidthFirstItem, takeWidth, "", []Highlight{})
	remainingTotalWidth := takeWidth - takenWidth

	// continue with subsequent items
	currentItemIdx := firstItemIdx + 1
	for remainingTotalWidth > 0 && currentItemIdx < len(m.items) {
		nextPart, partWidth := m.items[currentItemIdx].Take(0, remainingTotalWidth, "", []Highlight{})
		if partWidth == 0 {
			break
		}
		res += nextPart
		remainingTotalWidth -= partWidth
		currentItemIdx++
	}

	// apply highlights
	res = highlightString(
		res,
		highlights,
		firstByteIdx,
		firstByteIdx+len(StripAnsi(res)),
	)

	// apply continuation indicators for non-pinned section
	if len(continuation) > 0 {
		contentToLeft := widthToLeft > 0
		contentToRight := nonPinnedTotalWidth-widthToLeft > takeWidth-remainingTotalWidth
		if contentToLeft || contentToRight {
			continuationRunes := []rune(continuation)
			if contentToLeft {
				res = replaceStartWithContinuation(res, continuationRunes)
			}
			if contentToRight {
				res = replaceEndWithContinuation(res, continuationRunes)
			}
		}
	}

	return res, takeWidth - remainingTotalWidth
}

// takePinnedOnly handles case where pinned width >= viewport width
func (m ConcatItem) takePinnedOnly(takeWidth int, continuation string, highlights []Highlight) (string, int) {
	// render only pinned items, applying continuation if they overflow
	var result strings.Builder
	remainingWidth := takeWidth

	for i := 0; i < m.pinnedCount && remainingWidth > 0; i++ {
		part, partWidth := m.items[i].Take(0, remainingWidth, "", []Highlight{})
		if partWidth == 0 {
			break
		}
		result.WriteString(part)
		remainingWidth -= partWidth
	}

	res := result.String()

	// calculate byte range for highlights
	endByteIdx := 0
	for i := 0; i < m.pinnedCount; i++ {
		endByteIdx += len(m.items[i].lineNoAnsi)
	}

	res = highlightString(res, highlights, 0, min(endByteIdx, len(StripAnsi(res))))

	// apply continuation if pinned items overflow viewport
	if len(continuation) > 0 && m.pinnedWidth > takeWidth {
		res = replaceEndWithContinuation(res, []rune(continuation))
	}

	return res, takeWidth - remainingWidth
}

// NumWrappedLines returns the number of wrapped lines given a wrap width
func (m ConcatItem) NumWrappedLines(wrapWidth int) int {
	if wrapWidth <= 0 {
		return 0
	} else if m.totalWidth == 0 {
		return 1
	}
	return (m.totalWidth + wrapWidth - 1) / wrapWidth
}

// LineBrokenItems returns a slice containing just this item (single-line).
func (m ConcatItem) LineBrokenItems() []Item {
	return []Item{m}
}

// Repr returns a string representation of the ConcatItem for debugging.
func (m ConcatItem) repr() string {
	var v strings.Builder
	v.WriteString("Concat(")
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
func (m ConcatItem) ByteRangesToMatches(byteRanges []ByteRange) []Match {
	if len(m.items) == 0 || len(byteRanges) == 0 {
		return nil
	}
	if len(m.items) == 1 {
		return m.items[0].ByteRangesToMatches(byteRanges)
	}

	itemByteOffsets, itemWidthOffsets := m.computeItemOffsets()

	matches := make([]Match, 0, len(byteRanges))
	for _, br := range byteRanges {
		startWidth, endWidth := m.concatByteRangeToWidthRange(br.Start, br.End, itemByteOffsets, itemWidthOffsets)
		matches = append(matches, Match{
			ByteRange:  br,
			WidthRange: WidthRange{Start: startWidth, End: endWidth},
		})
	}
	return matches
}

// concatByteRangeToWidthRange converts a byte range in the concatenated content to a
// width range using precomputed item offsets.
func (m ConcatItem) concatByteRangeToWidthRange(
	startByte, endByte int,
	itemByteOffsets, itemWidthOffsets []int,
) (startWidth, endWidth int) {
	startItemIdx, startLocalByteOffset := m.findItemForByteOffset(startByte, itemByteOffsets)
	endItemIdx, endLocalByteOffset := m.findItemForByteOffset(endByte, itemByteOffsets)

	if startItemIdx >= 0 && startItemIdx < len(m.items) {
		startRuneIdx := m.items[startItemIdx].getRuneIndexAtByteOffset(startLocalByteOffset)
		if startRuneIdx > 0 {
			startWidth = int(m.items[startItemIdx].getCumulativeWidthAtRuneIdx(startRuneIdx - 1))
		}
		startWidth += itemWidthOffsets[startItemIdx]
	}

	if endItemIdx >= 0 && endItemIdx < len(m.items) {
		endRuneIdx := m.items[endItemIdx].getRuneIndexAtByteOffset(endLocalByteOffset)
		if endRuneIdx > 0 {
			endWidth = int(m.items[endItemIdx].getCumulativeWidthAtRuneIdx(endRuneIdx - 1))
		}
		endWidth += itemWidthOffsets[endItemIdx]
	}
	return
}

// computeItemOffsets precomputes cumulative byte and width offsets for each item.
func (m ConcatItem) computeItemOffsets() (itemByteOffsets, itemWidthOffsets []int) {
	itemByteOffsets = make([]int, len(m.items)+1)
	itemWidthOffsets = make([]int, len(m.items)+1)
	for i, it := range m.items {
		itemByteOffsets[i+1] = itemByteOffsets[i] + len(it.ContentNoAnsi())
		itemWidthOffsets[i+1] = itemWidthOffsets[i] + it.Width()
	}
	return
}

// ExtractExactMatches extracts exact matches from the item's content without ANSI codes
func (m ConcatItem) ExtractExactMatches(exactMatch string) []Match {
	if len(m.items) == 0 || exactMatch == "" {
		return []Match{}
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

// findItemForByteOffset finds which item contains the given byte offset in concatenated content
// Returns (itemIndex, localByteOffset) where localByteOffset is the offset within that item
func (m ConcatItem) findItemForByteOffset(byteOffset int, itemByteOffsets []int) (int, int) {
	// binary search to find the item containing this byte offset
	left, right := 0, len(m.items)-1

	for left <= right {
		mid := left + (right-left)/2
		if byteOffset >= itemByteOffsets[mid] && byteOffset < itemByteOffsets[mid+1] {
			return mid, byteOffset - itemByteOffsets[mid]
		} else if byteOffset < itemByteOffsets[mid] {
			right = mid - 1
		} else {
			left = mid + 1
		}
	}

	// if not found within items, handle edge cases
	if byteOffset >= itemByteOffsets[len(m.items)] {
		// past the end - return last item with offset at end
		lastItemIdx := len(m.items) - 1
		return lastItemIdx, len(m.items[lastItemIdx].ContentNoAnsi())
	}

	// before the beginning
	return 0, 0
}

// ExtractRegexMatches extracts regex matches from the item's content without ANSI codes
func (m ConcatItem) ExtractRegexMatches(regex *regexp.Regexp) []Match {
	if len(m.items) == 0 {
		return []Match{}
	}
	if len(m.items) == 1 {
		return m.items[0].ExtractRegexMatches(regex)
	}

	concatenated := m.ContentNoAnsi()
	regexMatches := regex.FindAllStringIndex(concatenated, -1)
	if len(regexMatches) == 0 {
		return []Match{}
	}

	byteRanges := make([]ByteRange, 0, len(regexMatches))
	for _, rm := range regexMatches {
		byteRanges = append(byteRanges, ByteRange{Start: rm[0], End: rm[1]})
	}
	return m.ByteRangesToMatches(byteRanges)
}
