package diferenco

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestPatienceDiff(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	bytesA, err := os.ReadFile(filepath.Join(dir, "testdata/a.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read a error: %v\n", err)
		return
	}
	a := string(bytesA)
	bytesB, err := os.ReadFile(filepath.Join(dir, "testdata/b.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read b error: %v\n", err)
		return
	}
	b := string(bytesB)
	sink := &Sink{
		Index: make(map[string]int),
	}
	aa := sink.ParseLines(a)
	bb := sink.ParseLines(b)
	diffs := PatienceDiff(aa, bb)
	for _, d := range diffs {
		switch d.T {
		case Delete:
			for _, i := range d.E {
				fmt.Fprintf(os.Stderr, "-%s", sink.Lines[i])
			}
		case Insert:
			for _, i := range d.E {
				fmt.Fprintf(os.Stderr, "+%s", sink.Lines[i])
			}
		default:
			for _, i := range d.E {
				fmt.Fprintf(os.Stderr, " %s", sink.Lines[i])
			}
		}
	}
}
