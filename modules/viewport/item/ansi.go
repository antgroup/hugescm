package item

import (
	"strings"
	"unicode/utf8"

	"charm.land/lipgloss/v2"
)

// StripAnsi removes all ANSI escape sequences from the input string.
func StripAnsi(input string) string {
	ranges := findAnsiByteRanges(input)
	if len(ranges) == 0 {
		return input
	}

	totalAnsiLen := 0
	for _, r := range ranges {
		totalAnsiLen += int(r[1] - r[0])
	}

	finalLen := len(input) - totalAnsiLen
	var builder strings.Builder
	builder.Grow(finalLen)

	lastPos := 0
	for _, r := range ranges {
		builder.WriteString(input[lastPos:int(r[0])])
		lastPos = int(r[1])
	}

	builder.WriteString(input[lastPos:])
	return builder.String()
}

// highlightRange represents a highlight with start/end positions and style
type highlightRange struct {
	startByte int
	endByte   int
	style     lipgloss.Style
}

// RST is the ansi escape sequence for resetting styles
// lipgloss v2 uses the short form "\x1b[m"
const RST = "\x1b[m"

// isResetCode checks if a code is an ANSI reset sequence
// Both "\x1b[0m" and "\x1b[m" are valid reset codes
func isResetCode(code string) bool {
	return code == "\x1b[0m" || code == "\x1b[m"
}

// reapplyAnsi reconstructs ANSI escape sequences in a truncated string based on their positions in the original.
// It ensures that any active text formatting (colors, styles) from the original string is correctly maintained
// in the truncated output, and adds proper reset codes where needed.
//
// Parameters:
//   - original: the source string containing ANSI escape sequences
//   - truncated: the truncated version of the string, without ANSI sequences
//   - truncByteOffset: byte offset in the original string where truncation started
//   - ansiCodeIndexes: pairs of start/end byte positions of ANSI codes in the original string
//
// Returns a string with ANSI escape sequences reapplied at appropriate positions,
// maintaining the original text formatting while preserving proper UTF-8 encoding.
func reapplyAnsi(original, truncated string, truncByteOffset int, ansiCodeIndexes [][]uint32) string {
	var result strings.Builder
	result.Grow(len(truncated))
	var lenAnsiAdded int
	isReset := true

	for i := 0; i < len(truncated); {
		// collect all ansi codes that should be applied immediately before the current runes
		var ansisToAdd []string
		for len(ansiCodeIndexes) > 0 {
			candidateAnsi := ansiCodeIndexes[0]
			codeStart, codeEnd := int(candidateAnsi[0]), int(candidateAnsi[1])
			originalByteIdx := truncByteOffset + i + lenAnsiAdded
			if codeStart <= originalByteIdx {
				code := original[codeStart:codeEnd]
				isReset = isResetCode(code)
				ansisToAdd = append(ansisToAdd, code)
				lenAnsiAdded += codeEnd - codeStart
				ansiCodeIndexes = ansiCodeIndexes[1:]
			} else {
				break
			}
		}

		for _, ansi := range simplifyAnsiCodes(ansisToAdd) {
			result.WriteString(ansi)
		}

		// add the bytes of the current rune
		_, size := utf8.DecodeRuneInString(truncated[i:])
		result.WriteString(truncated[i : i+size])
		i += size
	}

	if !isReset {
		result.WriteString(RST)
	}
	return result.String()
}

// getNonAnsiBytes extracts a substring of specified length from the input string, excluding ANSI escape sequences.
// It reads from the given start position until it has collected the requested number of non-ANSI bytes.
//
// Parameters:
//   - s: The input string that may contain ANSI escape sequences
//   - startIdx: The byte position in the input to start reading from
//   - numBytes: The number of non-ANSI bytes to collect
//
// Returns a string containing bytesToExtract bytes of the input with ANSI sequences removed. If the input text ends
// before collecting bytesToExtract bytes, returns all available non-ANSI bytes.
func getNonAnsiBytes(s string, startIdx, numBytes int) string {
	var result strings.Builder
	currentPos := startIdx
	bytesCollected := 0
	for currentPos < len(s) && bytesCollected < numBytes {
		if strings.HasPrefix(s[currentPos:], "\x1b[") {
			escEnd := currentPos + strings.Index(s[currentPos:], "m") + 1
			currentPos = escEnd
			continue
		}
		result.WriteByte(s[currentPos])
		bytesCollected++
		currentPos++
	}
	return result.String()
}

// highlightString applies highlighting to a segment of text while handling cases where the highlight
// might overflow the segment boundaries. It preserves any existing ANSI styling in the segment.
//
// Parameters:
//   - styledSegment: the text segment to highlight, which may contain ANSI codes
//   - highlights: a list of Highlight structs defining the styledLine byte offsets and styles to apply
//     NOTE: highlight byte ranges should not overlap
//   - plainLineSegmentStartByte: byte offset where styledSegment starts in full line without ansi codes
//   - plainLineSegmentEndByte: byte offset where styledSegment ends in full line without ansi codes
//
// Returns the segment with highlighting applied, preserving original ANSI codes.
func highlightString(
	styledSegment string,
	highlights []Highlight,
	plainLineSegmentStartByte int,
	plainLineSegmentEndByte int,
) string {
	if len(highlights) == 0 {
		return styledSegment
	}

	var applicableHighlights []highlightRange
	for _, highlight := range highlights {
		unstyledByteRange := highlight.ByteRangeUnstyledContent
		if unstyledByteRange.Start < plainLineSegmentEndByte && unstyledByteRange.End > plainLineSegmentStartByte {
			startByte := max(unstyledByteRange.Start, plainLineSegmentStartByte) - plainLineSegmentStartByte
			endByte := min(unstyledByteRange.End, plainLineSegmentEndByte) - plainLineSegmentStartByte
			applicableHighlights = append(applicableHighlights, highlightRange{
				startByte: startByte,
				endByte:   endByte,
				style:     highlight.Style,
			})
		}
	}

	if len(applicableHighlights) == 0 {
		return styledSegment
	}

	// sort highlights by start position
	for i := 0; i < len(applicableHighlights); i++ {
		for j := i + 1; j < len(applicableHighlights); j++ {
			if applicableHighlights[j].startByte < applicableHighlights[i].startByte {
				applicableHighlights[i], applicableHighlights[j] = applicableHighlights[j], applicableHighlights[i]
			}
		}
	}

	var result strings.Builder
	// pre-allocation based on highlight density (~50 bytes per highlight for styling)
	estimatedSize := len(styledSegment) + len(applicableHighlights)*50
	result.Grow(estimatedSize)

	var activeStyles []string
	nonAnsiBytes := 0
	highlightIdx := 0
	inAnsi := false

	i := 0
	for i < len(styledSegment) {
		// handle ansi sequences
		if strings.HasPrefix(styledSegment[i:], "\x1b[") {
			inAnsi = true
			ansiLen := strings.Index(styledSegment[i:], "m")
			if ansiLen != -1 {
				escEnd := i + ansiLen + 1
				ansi := styledSegment[i:escEnd]
				if isResetCode(ansi) {
					activeStyles = []string{} // reset
				} else {
					activeStyles = append(activeStyles, ansi) // add new active style
				}
				result.WriteString(ansi)
				i = escEnd
				inAnsi = false
				continue
			}
		}

		if !inAnsi {
			// check if need to start a highlight at this position
			highlighted := false
			for highlightIdx < len(applicableHighlights) &&
				applicableHighlights[highlightIdx].startByte == nonAnsiBytes {
				highlight := applicableHighlights[highlightIdx]

				// reset current styles if any
				if len(activeStyles) > 0 {
					result.WriteString(RST)
				}

				// extract and apply highlight text
				plainText := getNonAnsiBytes(styledSegment, i, highlight.endByte-highlight.startByte)
				result.WriteString(highlight.style.Render(plainText))

				// restore previous styles if any
				if len(activeStyles) > 0 {
					for _, style := range activeStyles {
						result.WriteString(style)
					}
				}

				// skip highlighted text
				count := 0
				for count < len(plainText) && i < len(styledSegment) {
					if strings.HasPrefix(styledSegment[i:], "\x1b[") {
						escEnd := i + strings.Index(styledSegment[i:], "m") + 1
						result.WriteString(styledSegment[i:escEnd])
						i = escEnd
						continue
					}
					i++
					count++
				}
				nonAnsiBytes += len(plainText)
				highlightIdx++

				// skip to next highlight that doesn't overlap
				for highlightIdx < len(applicableHighlights) &&
					applicableHighlights[highlightIdx].startByte < nonAnsiBytes {
					highlightIdx++
				}

				highlighted = true
				continue
			}
			if highlighted {
				continue
			}
		}

		// regular character
		if i < len(styledSegment) {
			result.WriteByte(styledSegment[i])
			if !inAnsi {
				nonAnsiBytes++
			}
		}
		i++
	}

	return removeEmptyAnsiSequences(result.String())
}

func simplifyAnsiCodes(ansis []string) []string {
	if len(ansis) == 0 {
		return []string{}
	}

	// if there's just a bunch of reset sequences, compress it to one
	allReset := true
	for _, ansi := range ansis {
		if !isResetCode(ansi) {
			allReset = false
			break
		}
	}
	if allReset {
		return []string{RST}
	}

	// return all ansis to the right of the rightmost reset seq
	for i := len(ansis) - 1; i >= 0; i-- {
		if isResetCode(ansis[i]) {
			result := ansis[i+1:]
			// keep reset at the start if present
			if isResetCode(ansis[0]) {
				return append([]string{RST}, result...)
			}
			return result
		}
	}
	return ansis
}

func runesHaveAnsiPrefix(runes []rune) bool {
	return len(runes) >= 2 && runes[0] == '\x1b' && runes[1] == '['
}

func findAnsiByteRanges(s string) [][]uint32 {
	// pre-count to allocate exact size
	count := strings.Count(s, "\x1b[")
	if count == 0 {
		return nil
	}

	allRanges := make([]uint32, count*2)
	ranges := make([][]uint32, count)

	for i := range count {
		ranges[i] = allRanges[i*2 : i*2+2]
	}

	rangeIdx := 0
	for i := 0; i < len(s); {
		if i+1 < len(s) && s[i] == '\x1b' && s[i+1] == '[' {
			start := i
			i += 2 // skip \x1b[

			// find the 'm' that ends this sequence
			for i < len(s) && s[i] != 'm' {
				i++
			}

			if i < len(s) && s[i] == 'm' {
				allRanges[rangeIdx*2] = clampIntToUint32(start)
				allRanges[rangeIdx*2+1] = clampIntToUint32(i + 1)
				rangeIdx++
				i++
				continue
			}
		}
		i++
	}
	return ranges[:rangeIdx]
}

func findAnsiRuneRanges(s string) [][]uint32 {
	// pre-count to allocate exact size
	count := strings.Count(s, "\x1b[")
	if count == 0 {
		return nil
	}

	allRanges := make([]uint32, count*2)
	ranges := make([][]uint32, count)

	for i := range count {
		ranges[i] = allRanges[i*2 : i*2+2]
	}

	rangeIdx := 0
	runes := []rune(s)
	for i := 0; i < len(runes); {
		if i+1 < len(runes) && runes[i] == '\x1b' && runes[i+1] == '[' {
			start := i
			i += 2 // skip \x1b[

			// find the 'm' that ends this sequence
			for i < len(runes) && runes[i] != 'm' {
				i++
			}

			if i < len(runes) && runes[i] == 'm' {
				allRanges[rangeIdx*2] = clampIntToUint32(start)
				allRanges[rangeIdx*2+1] = clampIntToUint32(i + 1)
				rangeIdx++
				i++
				continue
			}
		}
		i++
	}
	return ranges[:rangeIdx]
}

// stripNonSGR removes all non-SGR ANSI escape sequences from the input string.
// SGR sequences (\x1b[...m) are preserved. all other escape sequences (CSI non-SGR,
// OSC, Fe, Fp, nF, SS2, SS3) are stripped. uses lazy allocation so lines containing
// only SGR sequences (the common case) incur zero allocations.
func stripNonSGR(line string) string {
	if !strings.Contains(line, "\x1b") {
		return line
	}

	var b strings.Builder
	allocated := false
	lastCopied := 0

	i := 0
	for i < len(line) {
		if line[i] != '\x1b' {
			i++
			continue
		}

		seqStart := i
		i++ // past \x1b

		if i >= len(line) {
			// bare \x1b at end of string — keep it
			break
		}

		next := line[i]
		switch {
		case next == '[':
			// CSI sequence: \x1b[ + params (0x30-0x3F) + intermediates (0x20-0x2F) + final (0x40-0x7E)
			i++ // past [

			// consume parameter bytes
			for i < len(line) && line[i] >= 0x30 && line[i] <= 0x3F {
				i++
			}
			// consume intermediate bytes
			for i < len(line) && line[i] >= 0x20 && line[i] <= 0x2F {
				i++
			}
			// final byte
			if i >= len(line) {
				// truncated CSI — keep as literal text
				i = seqStart + 1
				continue
			}
			finalByte := line[i]
			i++ // past final byte

			if finalByte < 0x40 || finalByte > 0x7E {
				// malformed — keep as literal text
				i = seqStart + 1
				continue
			}

			if finalByte == 'm' {
				// SGR — keep
				continue
			}

			// non-SGR CSI — strip
			if !allocated {
				b.Grow(len(line))
				allocated = true
			}
			b.WriteString(line[lastCopied:seqStart])
			lastCopied = i

		case next == ']':
			// OSC sequence: \x1b] ... terminated by BEL (\x07) or ST (\x1b\\)
			i++ // past ]
			for i < len(line) {
				if line[i] == '\x07' {
					i++ // past BEL
					break
				}
				if line[i] == '\x1b' && i+1 < len(line) && line[i+1] == '\\' {
					i += 2 // past ST
					break
				}
				i++
			}
			// strip (including unterminated OSC at end of string)
			if !allocated {
				b.Grow(len(line))
				allocated = true
			}
			b.WriteString(line[lastCopied:seqStart])
			lastCopied = i

		case next == 'N' || next == 'O':
			// SS2 or SS3: \x1b + N/O + one designated character
			i++ // past N or O
			if i < len(line) {
				i++ // past designated character
			}
			if !allocated {
				b.Grow(len(line))
				allocated = true
			}
			b.WriteString(line[lastCopied:seqStart])
			lastCopied = i

		case next >= 0x40 && next <= 0x5F:
			// Fe sequence (excluding [, ], N, O handled above): \x1b + Fe byte
			i++ // past Fe byte
			if !allocated {
				b.Grow(len(line))
				allocated = true
			}
			b.WriteString(line[lastCopied:seqStart])
			lastCopied = i

		case next >= 0x30 && next <= 0x3F:
			// Fp (DEC private): \x1b + byte in 0x30-0x3F (ESC-7, ESC-8, ESC-=, ESC->)
			i++ // past Fp byte
			if !allocated {
				b.Grow(len(line))
				allocated = true
			}
			b.WriteString(line[lastCopied:seqStart])
			lastCopied = i

		case next >= 0x20 && next <= 0x2F:
			// nF sequence: \x1b + intermediates (0x20-0x2F) + final (0x30-0x7E)
			i++ // past first intermediate
			for i < len(line) && line[i] >= 0x20 && line[i] <= 0x2F {
				i++
			}
			if i < len(line) && line[i] >= 0x30 && line[i] <= 0x7E {
				i++ // past final byte
			}
			if !allocated {
				b.Grow(len(line))
				allocated = true
			}
			b.WriteString(line[lastCopied:seqStart])
			lastCopied = i

		default:
			// bare \x1b followed by unrecognized byte — keep as literal
			continue
		}
	}

	if !allocated {
		return line
	}
	b.WriteString(line[lastCopied:])
	return b.String()
}

func removeEmptyAnsiSequences(s string) string {
	if len(s) == 0 {
		return s
	}

	var result strings.Builder
	result.Grow(len(s))

	i := 0
	for i < len(s) {
		if i < len(s)-4 && s[i:i+2] == "\x1b[" {
			// find the end of this ansi sequence
			end := i + 2
			for end < len(s) && s[end] != 'm' {
				end++
			}
			if end < len(s) {
				end++ // include the 'm'
				ansiSeq := s[i:end]

				// check if this is followed immediately by a reset sequence
				if end < len(s)-2 && s[end:end+2] == "\x1b[" {
					resetEnd := end + 2
					for resetEnd < len(s) && s[resetEnd] != 'm' {
						resetEnd++
					}
					if resetEnd < len(s) {
						resetEnd++ // include the 'm'
						resetSeq := s[end:resetEnd]

						// if this is a reset sequence (\x1b[0m or \x1b[m), skip both sequences
						if resetSeq == "\x1b[0m" || resetSeq == "\x1b[m" {
							i = resetEnd
							continue
						}
					}
				}

				// not followed by reset, keep the sequence
				result.WriteString(ansiSeq)
				i = end
				continue
			}
		}

		result.WriteByte(s[i])
		i++
	}

	return result.String()
}
