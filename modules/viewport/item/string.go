package item

import (
	"slices"
	"strings"

	"github.com/clipperhouse/displaywidth"
)

// overflowsLeft checks if a substring overflows a string on the left if the string were to start at startByteIdx inclusive.
// assumes s has no ansi codes.
// It performs a case-sensitive comparison and returns two values:
//   - A boolean indicating whether there is overflow
//   - An integer indicating the ending string index (exclusive) of the overflow (0 if none)
//
// Examples:
//
//	                   01234567890
//		overflowsLeft("my str here", 3, "my str") returns (true, 6)
//		overflowsLeft("my str here", 3, "your str") returns (false, 0)
//		overflowsLeft("my str here", 6, "my str") returns (false, 0)
func overflowsLeft(s string, startByteIdx int, substr string) (bool, int) {
	if len(s) == 0 || len(substr) == 0 || len(substr) > len(s) {
		return false, 0
	}
	end := len(substr) + startByteIdx
	for offset := 1; offset < len(substr); offset++ {
		if startByteIdx-offset < 0 || end-offset > len(s) {
			continue
		}
		if s[startByteIdx-offset:end-offset] == substr {
			return true, end - offset
		}
	}
	return false, 0
}

// overflowsRight checks if a substring overflows a string on the right if the string were to end at endByteIdx exclusive.
// assumes s has no ansi codes.
// It performs a case-sensitive comparison and returns two values:
//   - A boolean indicating whether there is overflow
//   - An integer indicating the starting string startByteIdx of the overflow (0 if none)
//
// Examples:
//
//	                    01234567890
//		overflowsRight("my str here", 3, "y str") returns (true, 1)
//		overflowsRight("my str here", 3, "y strong") returns (false, 0)
//		overflowsRight("my str here", 6, "tr here") returns (true, 4)
func overflowsRight(s string, endByteIdx int, substr string) (bool, int) {
	if len(s) == 0 || len(substr) == 0 || len(substr) > len(s) {
		return false, 0
	}

	leftmostIdx := endByteIdx - len(substr) + 1
	for offset := 0; offset < len(substr); offset++ {
		startIdx := leftmostIdx + offset
		if startIdx < 0 || startIdx+len(substr) > len(s) {
			continue
		}
		sl := s[startIdx : startIdx+len(substr)]
		if sl == substr {
			return true, leftmostIdx + offset
		}
	}
	return false, 0
}

// runeDisplayWidth returns the monospace display width of a single rune.
// For user-visible text, grapheme clusters should be used instead;
// this helper is only for continuation indicator runes which are always
// standalone characters.
func runeDisplayWidth(r rune) int {
	return displaywidth.Rune(r)
}

func replaceStartWithContinuation(s string, continuationRunes []rune) string {
	if len(s) == 0 || len(continuationRunes) == 0 {
		return s
	}

	var sb strings.Builder
	// ControlSequences: true makes uax29 treat ANSI escape sequences as
	// zero-width grapheme clusters, so they pass through automatically.
	opts := displaywidth.Options{ControlSequences: true}
	g := opts.StringGraphemes(s)

	for g.Next() {
		cluster := g.Value()
		width := g.Width()

		// Zero-width clusters (ANSI codes, standalone combining marks) pass through
		if width == 0 {
			sb.WriteString(cluster)
			continue
		}

		if len(continuationRunes) > 0 {
			// Calculate total remaining continuation width
			remainingContinuationWidth := 0
			for _, cr := range continuationRunes {
				remainingContinuationWidth += runeDisplayWidth(cr)
			}

			if width > remainingContinuationWidth {
				// Cluster wider than remaining continuation — write cluster as-is, stop
				sb.WriteString(cluster)
				continuationRunes = nil
			} else {
				// Replace cluster with continuation runes
				clusterWidth := width
				for clusterWidth > 0 && len(continuationRunes) > 0 {
					currContinuationRune := continuationRunes[0]
					crWidth := runeDisplayWidth(currContinuationRune)
					sb.WriteRune(currContinuationRune)
					continuationRunes = continuationRunes[1:]
					clusterWidth -= crWidth
				}
			}
		} else {
			sb.WriteString(cluster)
		}
	}

	return sb.String()
}

func replaceEndWithContinuation(s string, continuationRunes []rune) string {
	if len(s) == 0 || len(continuationRunes) == 0 {
		return s
	}

	// Segment the entire string into grapheme clusters using displaywidth
	// with ControlSequences: true so ANSI escape sequences are returned as
	// zero-width clusters automatically.
	type segment struct {
		data  string
		width int // 0 for ANSI codes, >0 for visible clusters
	}
	var segs []segment

	opts := displaywidth.Options{ControlSequences: true}
	g := opts.StringGraphemes(s)
	for g.Next() {
		segs = append(segs, segment{data: g.Value(), width: g.Width()})
	}

	// Process segments from right to left, collecting output in reverse.
	var parts []string
	remainingCont := continuationRunes
	for _, seg := range slices.Backward(segs) {

		if seg.width == 0 {
			// Zero-width cluster (ANSI code, standalone combining mark) — pass through
			parts = append(parts, seg.data)
			continue
		}

		if len(remainingCont) > 0 {
			// Calculate total remaining continuation width
			remainingContWidth := 0
			for _, cr := range remainingCont {
				remainingContWidth += runeDisplayWidth(cr)
			}

			if seg.width > remainingContWidth {
				// Cluster wider than remaining continuation — write cluster as-is, stop
				parts = append(parts, seg.data)
				remainingCont = nil
			} else {
				// Replace cluster with continuation runes (from the end)
				clusterWidthLeft := seg.width
				for clusterWidthLeft > 0 && len(remainingCont) > 0 {
					cr := remainingCont[len(remainingCont)-1]
					crWidth := runeDisplayWidth(cr)
					parts = append(parts, string(cr))
					remainingCont = remainingCont[:len(remainingCont)-1]
					clusterWidthLeft -= crWidth
				}
			}
		} else {
			parts = append(parts, seg.data)
		}
	}

	// Build result by concatenating parts in reverse order.
	var result strings.Builder
	result.Grow(len(s))
	for _, part := range slices.Backward(parts) {
		result.WriteString(part)
	}

	return result.String()
}

// getBytesLeftOfWidth returns nBytes of content to the left of startItemIdx while excluding ANSI codes
func getBytesLeftOfWidth(nBytes int, items []SingleItem, startItemIdx int, widthToLeft int) string {
	if nBytes < 0 {
		panic("nBytes must be greater than 0")
	}
	if nBytes == 0 || len(items) == 0 || startItemIdx >= len(items) {
		return ""
	}

	// first try to get bytes from the current item
	var result string
	currentItem := items[startItemIdx]
	clusterIdx := currentItem.findClusterIndexWithWidthToLeft(widthToLeft)
	if clusterIdx > 0 {
		var startByteOffset uint32
		if clusterIdx >= currentItem.numClusters {
			startByteOffset = clampIntToUint32(len(currentItem.lineNoAnsi))
		} else {
			startByteOffset = currentItem.getByteOffsetAtClusterIdx(clusterIdx)
		}
		noAnsiContent := currentItem.lineNoAnsi[:startByteOffset]
		if len(noAnsiContent) >= nBytes {
			return noAnsiContent[len(noAnsiContent)-nBytes:]
		}
		result = noAnsiContent
		nBytes -= len(noAnsiContent)
	}

	// if we need more bytes, look in previous items
	for i := startItemIdx - 1; i >= 0 && nBytes > 0; i-- {
		prevItem := items[i]
		noAnsiContent := prevItem.lineNoAnsi
		if len(noAnsiContent) >= nBytes {
			result = noAnsiContent[len(noAnsiContent)-nBytes:] + result
			break
		}
		result = noAnsiContent + result
		nBytes -= len(noAnsiContent)
	}

	return result
}

// getBytesRightOfWidth returns nBytes of content to the right of endItemIdx while excluding ANSI codes
func getBytesRightOfWidth(nBytes int, items []SingleItem, endItemIdx int, widthToRight int) string {
	if nBytes < 0 {
		panic("nBytes must be greater than 0")
	}
	if nBytes == 0 || len(items) == 0 || endItemIdx >= len(items) {
		return ""
	}

	// first try to get bytes from the current item
	var result string
	currentItem := items[endItemIdx]
	if widthToRight > 0 {
		currentItemWidth := currentItem.Width()
		widthToLeft := currentItemWidth - widthToRight
		startClusterIdx := currentItem.findClusterIndexWithWidthToLeft(widthToLeft)
		if startClusterIdx < currentItem.numClusters {
			startByteOffset := currentItem.getByteOffsetAtClusterIdx(startClusterIdx)
			noAnsiContent := currentItem.lineNoAnsi[startByteOffset:]
			if len(noAnsiContent) >= nBytes {
				return noAnsiContent[:nBytes]
			}
			result = noAnsiContent
			nBytes -= len(noAnsiContent)
		}
	}

	// if we need more bytes, look in subsequent items
	for i := endItemIdx + 1; i < len(items) && nBytes > 0; i++ {
		nextItem := items[i]
		noAnsiContent := nextItem.lineNoAnsi
		if len(noAnsiContent) >= nBytes {
			result += noAnsiContent[:nBytes]
			break
		}
		result += noAnsiContent
		nBytes -= len(noAnsiContent)
	}

	return result
}
