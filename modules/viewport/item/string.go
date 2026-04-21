package item

import (
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

func replaceStartWithContinuation(s string, continuationRunes []rune) string {
	if len(s) == 0 || len(continuationRunes) == 0 {
		return s
	}

	var sb strings.Builder
	ansiCodeIndexes := findAnsiRuneRanges(s)
	runes := []rune(s)

	for runeIdx := 0; runeIdx < len(runes); {
		if len(ansiCodeIndexes) > 0 {
			codeStart, codeEnd := int(ansiCodeIndexes[0][0]), int(ansiCodeIndexes[0][1])
			if runeIdx == codeStart {
				for j := codeStart; j < codeEnd; j++ {
					sb.WriteRune(runes[j])
				}
				// skip ansi
				runeIdx = codeEnd
				ansiCodeIndexes = ansiCodeIndexes[1:]
				continue
			}
		}
		if len(continuationRunes) > 0 {
			rWidth := displaywidth.Rune(runes[runeIdx])

			// if rune is wider than remaining continuation width, cut off the continuation
			remainingContinuationWidth := 0
			for _, cr := range continuationRunes {
				remainingContinuationWidth += displaywidth.Rune(cr)
			}
			if rWidth > remainingContinuationWidth {
				sb.WriteRune(runes[runeIdx])
				continuationRunes = nil
			}

			// replace current rune with continuation runes
			for rWidth > 0 && len(continuationRunes) > 0 {
				currContinuationRune := continuationRunes[0]
				sb.WriteRune(currContinuationRune)
				continuationRunes = continuationRunes[1:]
				rWidth -= displaywidth.Rune(currContinuationRune)
			}

			// skip subsequent zero-width runes that are not ansi sequences
			nextIdx := runeIdx + 1
			for nextIdx < len(runes) {
				nextRWidth := displaywidth.Rune(runes[nextIdx])
				if nextRWidth == 0 && nextIdx < len(runes) && !runesHaveAnsiPrefix(runes[nextIdx:]) {
					runeIdx++
					nextIdx = runeIdx + 1
				} else {
					break
				}
			}
		} else {
			sb.WriteRune(runes[runeIdx])
		}
		runeIdx++
	}

	return sb.String()
}

func replaceEndWithContinuation(s string, continuationRunes []rune) string {
	if len(s) == 0 || len(continuationRunes) == 0 {
		return s
	}

	// collect runes to prepend (we're iterating backwards)
	var runesToPrepend []rune
	ansiCodeIndexes := findAnsiRuneRanges(s)
	runes := []rune(s)

	for runeIdx := len(runes) - 1; runeIdx >= 0; {
		if len(ansiCodeIndexes) > 0 {
			lastAnsiCodeIndexes := ansiCodeIndexes[len(ansiCodeIndexes)-1]
			codeStart, codeEnd := int(lastAnsiCodeIndexes[0]), int(lastAnsiCodeIndexes[1])
			if runeIdx == codeEnd-1 {
				for j := codeEnd - 1; j >= codeStart; j-- {
					runesToPrepend = append(runesToPrepend, runes[j])
				}
				// skip ansi
				runeIdx = codeStart - 1
				ansiCodeIndexes = ansiCodeIndexes[:len(ansiCodeIndexes)-1]
				continue
			}
		}
		if len(continuationRunes) > 0 {
			rWidth := displaywidth.Rune(runes[runeIdx])

			// if rune is wider than remaining continuation width, cut off the continuation
			remainingContinuationWidth := 0
			for _, cr := range continuationRunes {
				remainingContinuationWidth += displaywidth.Rune(cr)
			}
			if rWidth > remainingContinuationWidth {
				runesToPrepend = append(runesToPrepend, runes[runeIdx])
				continuationRunes = nil
			}

			// replace current rune with continuation runes
			for rWidth > 0 && len(continuationRunes) > 0 {
				currContinuationRune := continuationRunes[len(continuationRunes)-1]
				runesToPrepend = append(runesToPrepend, currContinuationRune)
				continuationRunes = continuationRunes[:len(continuationRunes)-1]
				rWidth -= displaywidth.Rune(currContinuationRune)
			}
		} else {
			runesToPrepend = append(runesToPrepend, runes[runeIdx])
		}
		runeIdx--
	}

	// build result string efficiently
	var result strings.Builder
	result.Grow(len(runesToPrepend) * 4) // estimate 4 bytes per rune on average
	for i := len(runesToPrepend) - 1; i >= 0; i-- {
		result.WriteRune(runesToPrepend[i])
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
	runeIdx := currentItem.findRuneIndexWithWidthToLeft(widthToLeft)
	if runeIdx > 0 {
		var startByteOffset uint32
		if runeIdx >= currentItem.numNoAnsiRunes {
			startByteOffset = clampIntToUint32(len(currentItem.lineNoAnsi))
		} else {
			startByteOffset = currentItem.getByteOffsetAtRuneIdx(runeIdx)
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
		startRuneIdx := currentItem.findRuneIndexWithWidthToLeft(widthToLeft)
		if startRuneIdx < currentItem.numNoAnsiRunes {
			startByteOffset := currentItem.getByteOffsetAtRuneIdx(startRuneIdx)
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
