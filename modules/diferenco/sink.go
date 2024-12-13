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
