package diferenco

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestProcessLine(t *testing.T) {
	text := `A
B
C
D
A`
	s := &Sink{
		Index: make(map[string]int),
	}
	lines := s.SplitLines(text)
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}
func TestProcessLineNewLine(t *testing.T) {
	text := `A
B
C
D
D
`
	s := &Sink{
		Index: make(map[string]int),
	}
	lines := s.SplitLines(text)
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}

func TestReadLines(t *testing.T) {
	text := `A
B
C
D
D
`
	s := &Sink{
		Index: make(map[string]int),
	}
	lines, err := s.ScanLines(strings.NewReader(text))
	if err != nil {
		return
	}
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}

func TestReadLinesNoNewLine(t *testing.T) {
	text := `A
B
C
D
D`
	s := &Sink{
		Index: make(map[string]int),
	}
	lines, err := s.ScanLines(strings.NewReader(text))
	if err != nil {
		return
	}
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d \"%s\"\n", line, strings.ReplaceAll(s.Lines[line], "\n", "\\n"))
	}
}

func TestReadLinesLF(t *testing.T) {
	text := `A
B
C
D
D`
	s := &Sink{
		Index:   make(map[string]int),
		NewLine: NEWLINE_LF,
	}
	lines, err := s.ScanLines(strings.NewReader(text))
	if err != nil {
		return
	}
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d \"%s\"\n", line, s.Lines[line])
	}
}

func TestProcessLineLF(t *testing.T) {
	text := `A
B
C
D
B`
	s := &Sink{
		NewLine: NEWLINE_LF,
		Index:   make(map[string]int),
	}
	lines := s.SplitLines(text)
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}

func TestProcessLineNewLineLF(t *testing.T) {
	text := `A
B
C
D
`
	s := &Sink{
		NewLine: NEWLINE_LF,
		Index:   make(map[string]int),
	}
	lines := s.SplitLines(text)
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}

func TestSplitWord(t *testing.T) {
	sss := []string{
		"  blah test2 test3  ",
		"\tblah test2 test3  ",
		"\tblah test2 test3  t",
		"\tblah test2 test3  tt",
		"The quick brown fox jumps over the lazy dog",
		"The quick brown dog leaps over the lazy cat",
		"Hello😋World",
		"😋  Hello😋World",
	}
	for _, s := range sss {
		w := SplitWords(s)
		fmt.Fprintf(os.Stderr, "[%s] -->\n", s)
		for _, e := range w {
			fmt.Fprintf(os.Stderr, "[%s] ", e)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}
}
