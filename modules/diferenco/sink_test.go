package diferenco

import (
	"fmt"
	"os"
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
	lines := s.ParseLines(text)
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
	lines := s.ParseLines(text)
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
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
	lines := s.ParseLines(text)
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
	lines := s.ParseLines(text)
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}
