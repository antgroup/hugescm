package diferenco

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/antgroup/hugescm/modules/diferenco/color"
)

func TestHistogram(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	bytesA, err := os.ReadFile(filepath.Join(dir, "testdata/a.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read a error: %v\n", err)
		return
	}
	textA := string(bytesA)
	bytesB, err := os.ReadFile(filepath.Join(dir, "testdata/b.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read b error: %v\n", err)
		return
	}
	textB := string(bytesB)
	sink := &Sink{
		Index: make(map[string]int),
	}
	a := sink.SplitLines(textA)
	b := sink.SplitLines(textB)
	changes, _ := HistogramDiff(t.Context(), a, b)
	u := sink.ToUnified(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}

func TestHistogram2(t *testing.T) {
	lines1 := `A
x
A
A
A
x
A
A
A`
	lines2 := `A
x
A
Z
A
x
A
A
A`
	sink := &Sink{
		Index: make(map[string]int),
	}
	a := sink.SplitLines(lines1)
	b := sink.SplitLines(lines2)
	changes, _ := HistogramDiff(t.Context(), a, b)
	u := sink.ToUnified(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}

func TestHistogram3(t *testing.T) {
	lines1 := `a
b
c
a
b
c`
	lines2 := `x
 b
z
a
b
c`
	sink := &Sink{
		Index: make(map[string]int),
	}
	a := sink.SplitLines(lines1)
	b := sink.SplitLines(lines2)
	changes, _ := HistogramDiff(t.Context(), a, b)
	u := sink.ToUnified(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}

func TestHistogram4(t *testing.T) {
	lines1 := `a
b
c
a
b
c
a
b
c`
	lines2 := `a
b
c
a1
a2
a3
b
c1
a
b
c`
	sink := &Sink{
		Index: make(map[string]int),
	}
	a := sink.SplitLines(lines1)
	b := sink.SplitLines(lines2)
	changes, _ := HistogramDiff(t.Context(), a, b)
	u := sink.ToUnified(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}
