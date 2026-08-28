package item

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/clipperhouse/displaywidth"
)

// SingleItem provides functionality to get sequential strings of a specified terminal cell width, accounting
// for the ansi escape codes styling the line.
type SingleItem struct {
	line            string     // underlying string with ansi codes. utf-8 encoded bytes
	lineNoAnsi      string     // line without ansi codes. utf-8 encoded bytes
	clusterWidths   []uint8    // display width of each grapheme cluster in lineNoAnsi
	ansiCodeIndexes [][]uint32 // slice of startByte, endByte indexes of ansi codes
	numClusters     int        // number of grapheme clusters in lineNoAnsi
	totalWidth      int        // total width in terminal cells
	fillStyle       string     // ANSI code to use when filling remaining width (emulates \x1b[K])

	sparsity                         int      // interval for which to store cumulative cell width
	sparseClusterStartByteOffset     []uint32 // cluster idx to byte offset, stored every sparsity clusters
	sparseLineNoAnsiCumClusterWidths []uint32 // cumulative terminal cell width, stored every sparsity clusters
}

// type assertion that SingleItem implements Item
var _ Item = SingleItem{}

// type assertion that *SingleItem implements Item
var _ Item = (*SingleItem)(nil)

// extractEraseInLineFillStyle finds \x1b[K or \x1b[0K in the line and returns
// the ANSI style code immediately before it (the style the terminal would use
// to fill). returns "" if no erase sequence is found or the preceding code is
// a reset (meaning fill uses default background).
func extractEraseInLineFillStyle(line string) string {
	pos := strings.Index(line, "\x1b[0K")
	if pos == -1 {
		pos = strings.Index(line, "\x1b[K")
	}
	if pos == -1 {
		return ""
	}

	// find the last \x1b[...m before the erase sequence
	prefix := line[:pos]
	lastEsc := strings.LastIndex(prefix, "\x1b[")
	if lastEsc == -1 {
		return ""
	}
	mIdx := strings.IndexByte(prefix[lastEsc:], 'm')
	if mIdx == -1 {
		return ""
	}
	code := prefix[lastEsc : lastEsc+mIdx+1]
	if isResetCode(code) {
		return ""
	}
	return code
}

// NewItem creates a new SingleItem from the given string.
func NewItem(line string) SingleItem {
	// \x1b[K and \x1b[0K tell the terminal to fill from cursor to end of line
	// with the current background color. we can't preserve them as-is because
	// the viewport's render() pads every line to a fixed width with lipgloss,
	// and those plain padding spaces overwrite the \x1b[K fill. instead, strip
	// them and record the ANSI style active at that position, then in Take()
	// append styled padding spaces to emulate the fill.
	fillStyle := extractEraseInLineFillStyle(line)
	if fillStyle != "" || strings.Contains(line, "\x1b[K") || strings.Contains(line, "\x1b[0K") {
		line = strings.ReplaceAll(line, "\x1b[0K", "")
		line = strings.ReplaceAll(line, "\x1b[K", "")
	}

	line = stripNonSGR(line)

	if len(line) <= 0 {
		return SingleItem{line: line, fillStyle: fillStyle}
	}

	// keep sparsity small for short lines
	sparsity := 4
	if len(line) > 100 {
		sparsity = 10 // tradeoff between memory usage and CPU. 10 seems to be a good balance
	}

	item := SingleItem{
		line:      line,
		sparsity:  sparsity,
		fillStyle: fillStyle,

		ansiCodeIndexes: findAnsiByteRanges(line)}

	if len(item.ansiCodeIndexes) > 0 {
		totalLen := len(line)
		for _, r := range item.ansiCodeIndexes {
			totalLen -= int(r[1] - r[0])
		}

		noAnsiBytes := make([]byte, 0, totalLen)
		lastPos := 0
		for _, r := range item.ansiCodeIndexes {
			noAnsiBytes = append(noAnsiBytes, line[lastPos:int(r[0])]...)
			lastPos = int(r[1])
		}
		noAnsiBytes = append(noAnsiBytes, line[lastPos:]...)
		item.lineNoAnsi = string(noAnsiBytes)
	} else {
		item.lineNoAnsi = line
	}

	// First pass: count grapheme clusters.
	// uax29/v2 (used by displaywidth) has a built-in ASCII hot path that
	// makes this very fast for ASCII-heavy text (e.g. source code).
	numClusters := 0
	{
		g := displaywidth.StringGraphemes(item.lineNoAnsi)
		for g.Next() {
			numClusters++
		}
	}

	// calculate size needed for sparse cumulative widths
	sparseLen := (numClusters + item.sparsity - 1) / item.sparsity
	item.sparseClusterStartByteOffset = make([]uint32, sparseLen)
	item.sparseLineNoAnsiCumClusterWidths = make([]uint32, sparseLen)

	// one byte per cluster for widths (0/1/2/3/4)
	item.clusterWidths = make([]uint8, numClusters)

	// Second pass: fill in cluster widths, byte offsets, and cumulative widths.
	{
		g := displaywidth.StringGraphemes(item.lineNoAnsi)
		var currentOffset uint32
		var cumWidth uint32
		for clusterIdx := 0; g.Next(); clusterIdx++ {
			width := clampIntToUint8(g.Width())

			item.clusterWidths[clusterIdx] = width

			cumWidth += uint32(width)
			if clusterIdx%item.sparsity == 0 {
				item.sparseClusterStartByteOffset[clusterIdx/item.sparsity] = currentOffset
				item.sparseLineNoAnsiCumClusterWidths[clusterIdx/item.sparsity] = cumWidth
			}
			if clusterIdx == numClusters-1 {
				item.totalWidth = int(cumWidth)
			}
			currentOffset += clampIntToUint32(len(g.Value()))
		}
	}
	item.numClusters = numClusters

	return item
}

// Width returns the total width in terminal cells.
func (l SingleItem) Width() int {
	if len(l.line) == 0 {
		return 0
	}
	return l.totalWidth
}

// Content returns the underlying string content
func (l SingleItem) Content() string {
	return l.line
}

// ContentNoAnsi returns the underlying string content without ANSI escape codes
func (l SingleItem) ContentNoAnsi() string {
	return l.lineNoAnsi
}

// Take returns a substring of the item that fits within the specified width
func (l SingleItem) Take(
	widthToLeft,
	takeWidth int,
	continuation string,
	highlights []Highlight,
) (string, int) {
	if widthToLeft < 0 {
		widthToLeft = 0
	}

	widthToLeft = min(widthToLeft, l.Width())
	startClusterIdx := l.findClusterIndexWithWidthToLeft(widthToLeft)

	if startClusterIdx >= l.numClusters || takeWidth == 0 {
		if l.fillStyle != "" && takeWidth > 0 {
			// content is empty but fill is requested — produce styled padding
			return l.fillStyle + strings.Repeat(" ", takeWidth) + RST, takeWidth
		}
		return "", 0
	}

	var result strings.Builder
	remainingWidth := takeWidth
	leftClusterIdx := startClusterIdx
	startByteOffset := l.getByteOffsetAtClusterIdx(startClusterIdx)

	clustersWritten := 0
	for ; remainingWidth > 0 && leftClusterIdx < l.numClusters; leftClusterIdx++ {
		cw := l.getClusterWidth(leftClusterIdx)
		if int(cw) > remainingWidth {
			break
		}

		result.WriteString(l.getClusterBytes(leftClusterIdx))
		clustersWritten++
		remainingWidth -= int(cw)
	}

	// if only zero-width clusters were written, return ""
	for i := 0; i < clustersWritten; i++ {
		if l.getClusterWidth(startClusterIdx+i) > 0 {
			break
		}
		if i == clustersWritten-1 {
			return "", 0
		}
	}

	res := result.String()

	// reapply original styling
	if len(l.ansiCodeIndexes) > 0 {
		res = reapplyAnsi(l.line, res, int(startByteOffset), l.ansiCodeIndexes)
	}

	// highlight the desired string
	var endByteOffset int
	if leftClusterIdx < l.numClusters {
		endByteOffset = int(l.getByteOffsetAtClusterIdx(leftClusterIdx))
	} else {
		endByteOffset = len(l.lineNoAnsi)
	}
	res = highlightString(
		res,
		highlights,
		int(startByteOffset),
		endByteOffset,
	)

	// apply left/right line continuation indicators
	if len(continuation) > 0 && (startClusterIdx > 0 || leftClusterIdx < l.numClusters) {
		continuationRunes := []rune(continuation)

		// if more clusters to the left of the result, replace start with continuation indicator
		if startClusterIdx > 0 {
			res = replaceStartWithContinuation(res, continuationRunes)
		}

		// if more clusters to the right, replace final clusters in result with continuation indicator
		if leftClusterIdx < l.numClusters {
			res = replaceEndWithContinuation(res, continuationRunes)
		}
	}

	// emulate \x1b[K: append padding spaces styled with the ANSI code that
	// was active at the \x1b[K position in the original line. we use explicit
	// styled spaces rather than re-emitting \x1b[K because render() pads
	// lines via lipgloss.Width(), and those unstyled spaces would overwrite
	// the fill.
	if l.fillStyle != "" && remainingWidth > 0 {
		res += l.fillStyle + strings.Repeat(" ", remainingWidth) + RST
		remainingWidth = 0
	}

	res = removeEmptyAnsiSequences(res)
	return res, takeWidth - remainingWidth
}

// NumWrappedLines returns the number of wrapped lines given a wrap width
func (l SingleItem) NumWrappedLines(wrapWidth int) int {
	if wrapWidth <= 0 {
		return 0
	} else if l.totalWidth == 0 {
		return 1
	}
	return (l.totalWidth + wrapWidth - 1) / wrapWidth
}

// LineBrokenItems returns a slice containing just this item (single-line).
func (l SingleItem) LineBrokenItems() []Item {
	return []Item{l}
}

// Repr returns a string representation for debugging.
func (l SingleItem) repr() string {
	return fmt.Sprintf("Item(%q)", l.line)
}

// getClusterBytes returns the string of the grapheme cluster at the given index
func (l SingleItem) getClusterBytes(clusterIdx int) string {
	if clusterIdx < 0 || clusterIdx >= l.numClusters {
		return ""
	}
	start := l.getByteOffsetAtClusterIdx(clusterIdx)
	var end uint32
	if clusterIdx+1 >= l.numClusters {
		end = clampIntToUint32(len(l.lineNoAnsi))
	} else {
		end = l.getByteOffsetAtClusterIdx(clusterIdx + 1)
	}
	return l.lineNoAnsi[start:end]
}

func (l SingleItem) getByteOffsetAtClusterIdx(clusterIdx int) uint32 {
	if clusterIdx < 0 {
		panic("clusterIdx must be greater or equal to 0")
	}
	if clusterIdx == 0 || len(l.line) == 0 || l.sparsity == 0 {
		return 0
	}
	if clusterIdx >= l.numClusters {
		panic("cluster index greater than num clusters")
	}

	// get the last stored byte offset before this index
	sparseIdx := clusterIdx / l.sparsity
	baseClusterIdx := sparseIdx * l.sparsity

	if baseClusterIdx == clusterIdx {
		return l.sparseClusterStartByteOffset[sparseIdx]
	}

	// ASCII fast path: if all bytes between the sparse base and the target
	// are printable ASCII, each cluster is exactly 1 byte.
	startByte := l.sparseClusterStartByteOffset[sparseIdx]
	remaining := clusterIdx - baseClusterIdx
	if int(startByte)+remaining <= len(l.lineNoAnsi) {
		allASCII := true
		for i := range remaining {
			b := l.lineNoAnsi[int(startByte)+i]
			if b < 0x20 || b > 0x7E {
				allASCII = false
				break
			}
		}
		if allASCII {
			return startByte + uint32(remaining)
		}
	}

	byteOffset := startByte
	g := displaywidth.StringGraphemes(l.lineNoAnsi[startByte:])
	for range remaining {
		g.Next()
		byteOffset += clampIntToUint32(len(g.Value()))
	}
	return byteOffset
}

// getClusterIndexAtByteOffset finds the cluster index at the given byte offset
func (l SingleItem) getClusterIndexAtByteOffset(byteOffset int) int {
	if byteOffset <= 0 || len(l.lineNoAnsi) == 0 {
		return 0
	}
	if byteOffset >= len(l.lineNoAnsi) {
		return l.numClusters
	}

	// binary search to find the cluster index
	left, right := 0, l.numClusters-1
	for left <= right {
		mid := left + (right-left)/2
		midByteOffset := int(l.getByteOffsetAtClusterIdx(mid))

		if midByteOffset == byteOffset {
			return mid
		} else if midByteOffset < byteOffset {
			left = mid + 1
		} else {
			right = mid - 1
		}
	}

	// if exact match not found, return the cluster index where byteOffset would fall
	return right
}

// getClusterWidth returns the width of the grapheme cluster at the given index
func (l SingleItem) getClusterWidth(clusterIdx int) uint8 {
	if clusterIdx < 0 || clusterIdx >= l.numClusters {
		return 0
	}
	return l.clusterWidths[clusterIdx]
}

func (l SingleItem) getCumulativeWidthAtClusterIdx(clusterIdx int) uint32 {
	if clusterIdx < 0 {
		return 0
	}
	if clusterIdx >= l.numClusters {
		panic("clusterIdx greater than num clusters")
	}

	// get the last stored cumulative width before this index
	sparseIdx := clusterIdx / l.sparsity
	baseClusterIdx := sparseIdx * l.sparsity

	if baseClusterIdx == clusterIdx {
		return l.sparseLineNoAnsiCumClusterWidths[sparseIdx]
	}

	// sum the widths from the last stored point to our target index
	var additionalWidth uint32
	for i := baseClusterIdx + 1; i <= clusterIdx; i++ {
		additionalWidth += uint32(l.getClusterWidth(i))
	}

	return l.sparseLineNoAnsiCumClusterWidths[sparseIdx] + additionalWidth
}

// findClusterIndexWithWidthToLeft returns the index of the cluster that has the input width to the left of it
func (l SingleItem) findClusterIndexWithWidthToLeft(widthToLeft int) int {
	if widthToLeft < 0 {
		panic("widthToLeft less than 0")
	}
	if widthToLeft == 0 || l.numClusters == 0 {
		return 0
	}
	if widthToLeft > l.Width() {
		panic("widthToLeft greater than total width")
	}

	left, right := 0, l.numClusters-1
	widthToLeftUint32 := clampIntToUint32(widthToLeft)
	if l.getCumulativeWidthAtClusterIdx(right) < widthToLeftUint32 {
		return l.numClusters
	}

	for left < right {
		mid := left + (right-left)/2
		if l.getCumulativeWidthAtClusterIdx(mid) >= widthToLeftUint32 {
			right = mid
		} else {
			left = mid + 1
		}
	}

	// skip over zero-width clusters
	w := l.getCumulativeWidthAtClusterIdx(left)
	nextLeft := left + 1
	for nextLeft < l.numClusters && l.getCumulativeWidthAtClusterIdx(nextLeft) == w {
		left = nextLeft
		nextLeft++
	}

	return left + 1
}

// ByteRangesToMatches converts byte ranges in the ANSI-stripped content to Matches.
func (l SingleItem) ByteRangesToMatches(byteRanges []ByteRange) []Match {
	if len(byteRanges) == 0 {
		return nil
	}
	matches := make([]Match, 0, len(byteRanges))
	for _, br := range byteRanges {
		startWidth, endWidth := l.byteRangeToWidthRange(br.Start, br.End)
		matches = append(matches, Match{
			ByteRange:  br,
			WidthRange: WidthRange{Start: startWidth, End: endWidth},
		})
	}
	return matches
}

// byteRangeToWidthRange converts a byte range to a width range for a SingleItem.
func (l SingleItem) byteRangeToWidthRange(startByte, endByte int) (startWidth, endWidth int) {
	startClusterIdx := l.getClusterIndexAtByteOffset(startByte)
	endClusterIdx := l.getClusterIndexAtByteOffset(endByte)

	if startClusterIdx > 0 {
		startWidth = int(l.getCumulativeWidthAtClusterIdx(startClusterIdx - 1))
	}
	if endClusterIdx > 0 {
		endWidth = int(l.getCumulativeWidthAtClusterIdx(endClusterIdx - 1))
	}
	return
}

// ExtractExactMatches extracts exact matches from the item's content without ANSI codes
func (l SingleItem) ExtractExactMatches(exactMatch string) []Match {
	if exactMatch == "" {
		return nil
	}

	unstyled := l.lineNoAnsi
	var byteRanges []ByteRange
	startIndex := 0
	for {
		foundIndex := strings.Index(unstyled[startIndex:], exactMatch)
		if foundIndex == -1 {
			break
		}
		actualStartIndex := startIndex + foundIndex
		endIndex := actualStartIndex + len(exactMatch)
		byteRanges = append(byteRanges, ByteRange{Start: actualStartIndex, End: endIndex})
		startIndex = endIndex // overlapping matches are not considered
	}
	return l.ByteRangesToMatches(byteRanges)
}

// ExtractRegexMatches extracts regex matches from the item's content without ANSI codes
func (l SingleItem) ExtractRegexMatches(regex *regexp.Regexp) []Match {
	regexMatches := regex.FindAllStringIndex(l.lineNoAnsi, -1)
	if len(regexMatches) == 0 {
		return nil
	}
	byteRanges := make([]ByteRange, 0, len(regexMatches))
	for _, rm := range regexMatches {
		byteRanges = append(byteRanges, ByteRange{Start: rm[0], End: rm[1]})
	}
	return l.ByteRangesToMatches(byteRanges)
}
