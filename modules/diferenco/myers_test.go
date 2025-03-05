package diferenco

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestMyersDiff(t *testing.T) {
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
	changes, _ := MyersDiff(t.Context(), a, b)
	i := 0
	for _, c := range changes {
		for ; i < c.P1; i++ {
			fmt.Fprintf(os.Stderr, "  %s", sink.Lines[a[i]])
		}
		for j := c.P1; j < c.P1+c.Del; j++ {
			fmt.Fprintf(os.Stderr, "- %s", sink.Lines[a[j]])
		}
		for j := c.P2; j < c.P2+c.Ins; j++ {
			fmt.Fprintf(os.Stderr, "+ %s", sink.Lines[b[j]])
		}
		i += c.Del
	}
	for ; i < len(a); i++ {
		fmt.Fprintf(os.Stderr, "  %s", sink.Lines[a[i]])
	}
	fmt.Fprintf(os.Stderr, "\n\nEND\n\n")
}

func TestMyersDiff2(t *testing.T) {
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
	changes, _ := MyersDiff(t.Context(), a, b)
	u := sink.ToUnified(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	fmt.Fprintf(os.Stderr, "diff:\n%s\n", u.String())
}

func TestMyersDiff3(t *testing.T) {
	textA := `1
2
3
4
5`
	textB := `1
4
5
4
5`
	sink := &Sink{
		Index: make(map[string]int),
	}
	a := sink.SplitLines(textA)
	b := sink.SplitLines(textB)
	changes, _ := MyersDiff(t.Context(), a, b)
	u := sink.ToUnified(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	fmt.Fprintf(os.Stderr, "diff:\n%s\n", u.String())
}
