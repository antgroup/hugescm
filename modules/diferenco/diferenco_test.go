package diferenco

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDiff(t *testing.T) {
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
	a := sink.ParseLines(textA)
	b := sink.ParseLines(textB)
	changes := OnpDiff(a, b)
	i := 0
	for _, c := range changes {
		for ; i < c.P1; i++ {
			fmt.Fprintf(os.Stderr, "  %s", sink.Lines[a[i]])
		}
		for j := c.P1; j < c.P1+c.Del; j++ {
			fmt.Fprintf(os.Stderr, "\x1b[31m- %s\x1b[0m", sink.Lines[a[j]])
		}
		for j := c.P2; j < c.P2+c.Ins; j++ {
			fmt.Fprintf(os.Stderr, "\x1b[32m+ %s\x1b[0m", sink.Lines[b[j]])
		}
		i += c.Del
	}
	for ; i < len(a); i++ {
		fmt.Fprintf(os.Stderr, "  %s", sink.Lines[a[i]])
	}
	fmt.Fprintf(os.Stderr, "\n\npatience\n\n")

	diffs := PatienceDiff(a, b)
	for _, d := range diffs {
		switch d.T {
		case Delete:
			for _, i := range d.E {
				fmt.Fprintf(os.Stderr, "\x1b[31m-%s\x1b[0m", sink.Lines[i])
			}
		case Insert:
			for _, i := range d.E {
				fmt.Fprintf(os.Stderr, "\x1b[32m+%s\x1b[0m", sink.Lines[i])
			}
		default:
			for _, i := range d.E {
				fmt.Fprintf(os.Stderr, " %s", sink.Lines[i])
			}
		}
	}
}
