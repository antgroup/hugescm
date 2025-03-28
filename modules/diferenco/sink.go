package diferenco

import (
	"bufio"
	"fmt"
	"io"
	"strings"
	"unicode"
)

const (
	NEWLINE_RAW = iota
	NEWLINE_LF
	NEWLINE_CRLF
)

type Sink struct {
	Lines   []string
	Index   map[string]int
	NewLine int
}

func NewSink(newLineMode int) *Sink {
	sink := &Sink{
		Lines:   make([]string, 0, 200),
		Index:   make(map[string]int),
		NewLine: newLineMode,
	}
	return sink
}

func (s *Sink) addLine(line string) int {
	if lineIndex, ok := s.Index[line]; ok {
		return lineIndex
	}
	index := len(s.Lines)
	s.Index[line] = index
	s.Lines = append(s.Lines, line)
	return index
}

func (s *Sink) ScanRawLines(r io.Reader) ([]int, error) {
	lines := make([]int, 0, 200)
	br := bufio.NewReader(r)
	for {
		line, err := br.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}
		// line including '\n' always >= 1
		if len(line) == 0 {
			break
		}
		lines = append(lines, s.addLine(line))
	}
	return lines, nil
}

func (s *Sink) ScanLines(r io.Reader) ([]int, error) {
	if s.NewLine == NEWLINE_RAW {
		return s.ScanRawLines(r)
	}
	lines := make([]int, 0, 200)
	br := bufio.NewScanner(r)
	for br.Scan() {
		lines = append(lines, s.addLine(strings.TrimSuffix(br.Text(), "\r")))
	}
	return lines, br.Err()
}

func (s *Sink) SplitRawLines(text string) []int {
	lines := make([]int, 0, 200)
	for pos := 0; pos < len(text); {
		part := text[pos:]
		newPos := strings.IndexByte(part, '\n')
		if newPos == -1 {
			lines = append(lines, s.addLine(part))
			break
		}
		lines = append(lines, s.addLine(part[:newPos+1]))
		pos += newPos + 1
	}
	return lines
}

func (s *Sink) SplitLines(text string) []int {
	if s.NewLine == NEWLINE_RAW {
		return s.SplitRawLines(text)
	}
	lines := make([]int, 0, 200)
	for pos := 0; pos < len(text); {
		part := text[pos:]
		newPos := strings.IndexByte(part, '\n')
		if newPos == -1 {
			lines = append(lines, s.addLine(strings.TrimSuffix(part, "\r")))
			break
		}
		lines = append(lines, s.addLine(strings.TrimSuffix(part[:newPos], "\r")))
		pos += newPos + 1
	}
	return lines
}

func (s *Sink) parseLines(r io.Reader, text string) ([]int, error) {
	if r != nil {
		return s.ScanLines(r)
	}
	return s.SplitLines(text), nil
}

func (s *Sink) WriteLine(w io.Writer, E ...int) {
	if s.NewLine == NEWLINE_CRLF {
		for _, e := range E {
			_, _ = fmt.Fprintf(w, "%s\r\n", s.Lines[e])
		}
		return
	}
	if s.NewLine == NEWLINE_LF {
		for _, e := range E {
			_, _ = fmt.Fprintln(w, s.Lines[e])
		}
		return
	}
	for _, e := range E {
		_, _ = io.WriteString(w, s.Lines[e])
	}
}

func (s *Sink) addEqualLines(h *Hunk, index []int, start, end int) int {
	delta := 0
	for i := start; i < end; i++ {
		if i < 0 {
			continue
		}
		if i >= len(index) {
			return delta
		}
		h.Lines = append(h.Lines, Line{Kind: Equal, Content: s.Lines[index[i]]})
		delta++
	}
	return delta
}

func (s *Sink) ToUnified(from, to *File, changes []Change, linesA, linesB []int, contextLines int) *Unified {
	gap := contextLines * 2
	u := &Unified{
		From: from,
		To:   to,
	}
	if len(changes) == 0 {
		return u
	}
	var h *Hunk
	last := 0
	toLine := 0
	for _, ch := range changes {
		start := ch.P1
		end := ch.P1 + ch.Del
		switch {
		case h != nil && start == last:
		case h != nil && start <= last+gap:
			// within range of previous lines, add the joiners
			s.addEqualLines(h, linesA, last, start)
		default:
			// need to start a new hunk
			if h != nil {
				// add the edge to the previous hunk
				s.addEqualLines(h, linesA, last, last+contextLines)
				u.Hunks = append(u.Hunks, h)
			}
			toLine += start - last
			h = &Hunk{
				FromLine: start + 1,
				ToLine:   toLine + 1,
			}
			// add the edge to the new hunk
			delta := s.addEqualLines(h, linesA, start-contextLines, start)
			h.FromLine -= delta
			h.ToLine -= delta
		}
		last = start
		for i := start; i < end; i++ {
			h.Lines = append(h.Lines, Line{Kind: Delete, Content: s.Lines[linesA[i]]})
			last++
		}
		addEnd := ch.P2 + ch.Ins
		for i := ch.P2; i < addEnd; i++ {
			h.Lines = append(h.Lines, Line{Kind: Insert, Content: s.Lines[linesB[i]]})
			toLine++
		}
	}
	if h != nil {
		// add the edge to the final hunk
		s.addEqualLines(h, linesA, last, last+contextLines)
		u.Hunks = append(u.Hunks, h)
	}
	return u
}

// Split string by non-letternumeric characters keeping delimiters
func SplitWords(text string) []string {
	words := make([]string, 0, 10)
	var hit bool
	var offset int
	for i, c := range text {
		if unicode.IsLetter(c) || unicode.IsDigit(c) {
			if !hit {
				if i > offset {
					words = append(words, text[offset:i])
				}
				offset = i
				hit = true
			}
			continue
		}
		if hit {
			if i > offset {
				words = append(words, text[offset:i])
			}
			offset = i
			hit = false
		}
	}
	if offset < len(text) {
		words = append(words, text[offset:])
	}
	return words
}
