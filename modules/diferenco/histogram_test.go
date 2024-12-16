package diferenco

import (
	"context"
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
	changes, _ := HistogramDiff(context.Background(), a, b)
	u := sink.ToUnified(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}
