package diferenco

import (
	"fmt"
	"io"
	"strings"
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

func (s *Sink) ProcessRawLines(text string) []int {
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

func (s *Sink) ParseLines(text string) []int {
	if s.NewLine == NEWLINE_RAW {
		return s.ProcessRawLines(text)
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

func (s *Sink) WriteLine(w io.Writer, E ...int) {
	if s.NewLine == NEWLINE_CRLF {
		for _, e := range E {
			fmt.Fprintf(w, "%s\r\n", s.Lines[e])
		}
		return
	}
	if s.NewLine == NEWLINE_LF {
		for _, e := range E {
			fmt.Fprintln(w, s.Lines[e])
		}
		return
	}
	for _, e := range E {
		_, _ = io.WriteString(w, s.Lines[e])
	}
}

func (s *Sink) AsStringDiff(o []Dfio[int]) []StringDiff {
	var newLine string
	switch s.NewLine {
	case NEWLINE_CRLF:
		newLine = "\r\n"
	case NEWLINE_LF:
		newLine = "\n"
	}
	diffs := make([]StringDiff, 0, len(o))
	for _, e := range o {
		ss := make([]string, 0, len(e.E))
		for _, i := range e.E {
			ss = append(ss, s.Lines[i])
		}
		diffs = append(diffs, StringDiff{
			Type: e.T,
			Text: strings.Join(ss, newLine),
		})
	}
	return diffs
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
