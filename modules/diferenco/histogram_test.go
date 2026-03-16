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
	changes, _ := DiffSlices(t.Context(), a, b, Histogram)
	u := sink.ToPatch(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Patch{u})
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
	changes, _ := DiffSlices(t.Context(), a, b, Histogram)
	u := sink.ToPatch(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Patch{u})
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
	changes, _ := DiffSlices(t.Context(), a, b, Histogram)
	u := sink.ToPatch(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Patch{u})
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
	changes, _ := DiffSlices(t.Context(), a, b, Histogram)
	u := sink.ToPatch(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Patch{u})
}

// TestHistogramHeuristic demonstrates the improved heuristic effect
func TestHistogramHeuristic(t *testing.T) {
	// Case 1: Multiple potential anchors - should pick the most unique one
	t.Log("\n=== Case 1: Prefer unique anchor over common lines ===")
	t.Log("Before optimization: might pick any matching line")
	t.Log("After optimization: picks the most unique (lowest occurrences) line")
	{
		text1 := `start
unique_anchor
middle
common
common
end`
		text2 := `start
unique_anchor
middle
common
end`
		sink := &Sink{Index: make(map[string]int)}
		a := sink.SplitLines(text1)
		b := sink.SplitLines(text2)
		changes, _ := DiffSlices(t.Context(), a, b, Histogram)

		u := sink.ToPatch(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
		e := NewUnifiedEncoder(os.Stderr)
		e.SetColor(color.NewColorConfig())
		_ = e.Encode([]*Patch{u})

		// Verify: should have 1 delete
		totalDel := 0
		for _, c := range changes {
			totalDel += c.Del
		}
		t.Logf("Result: %d changes, %d deletions (expected: 1 deletion)", len(changes), totalDel)
	}

	// Case 2: Longer match vs more unique match - prefer longer
	t.Log("\n=== Case 2: Prefer longer match over more unique ===")
	t.Log("Before optimization: might pick shorter unique match")
	t.Log("After optimization: picks the longest common substring")
	{
		text1 := `header
block_start
line1
line2
line3
block_end
trailer`
		text2 := `header
block_start
line1
line2
line3
block_end
new_trailer`
		sink := &Sink{Index: make(map[string]int)}
		a := sink.SplitLines(text1)
		b := sink.SplitLines(text2)
		changes, _ := DiffSlices(t.Context(), a, b, Histogram)

		u := sink.ToPatch(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
		e := NewUnifiedEncoder(os.Stderr)
		e.SetColor(color.NewColorConfig())
		_ = e.Encode([]*Patch{u})

		totalDel, totalIns := 0, 0
		for _, c := range changes {
			totalDel += c.Del
			totalIns += c.Ins
		}
		t.Logf("Result: %d changes, %d deletions, %d insertions", len(changes), totalDel, totalIns)
		t.Logf("Expected: 1 delete (trailer) + 1 insert (new_trailer)")
	}

	// Case 3: Cross-match scenario - classic diff problem
	t.Log("\n=== Case 3: Cross-match avoidance ===")
	t.Log("Without heuristic: might match wrong braces")
	t.Log("With heuristic: matches unique function signatures correctly")
	{
		text1 := `func foo() {
    return 1;
}
func bar() {
    return 2;
}`
		text2 := `func foo() {
    return 1;
}
func bar() {
    return 99;
}`
		sink := &Sink{Index: make(map[string]int)}
		a := sink.SplitLines(text1)
		b := sink.SplitLines(text2)
		changes, _ := DiffSlices(t.Context(), a, b, Histogram)

		u := sink.ToPatch(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
		e := NewUnifiedEncoder(os.Stderr)
		e.SetColor(color.NewColorConfig())
		_ = e.Encode([]*Patch{u})

		totalDel, totalIns := 0, 0
		for _, c := range changes {
			totalDel += c.Del
			totalIns += c.Ins
		}
		t.Logf("Result: %d deletions, %d insertions (expected: 1 del + 1 ins)", totalDel, totalIns)
	}

	// Case 4: Identical repeated blocks - stability test
	t.Log("\n=== Case 4: Repeated blocks stability ===")
	t.Log("Multiple identical blocks should be matched correctly")
	{
		text1 := `block {
    a
    b
}
block {
    a
    b
}`
		text2 := `block {
    a
    X
}
block {
    a
    Y
}`
		sink := &Sink{Index: make(map[string]int)}
		a := sink.SplitLines(text1)
		b := sink.SplitLines(text2)
		changes, _ := DiffSlices(t.Context(), a, b, Histogram)

		u := sink.ToPatch(&File{Name: "a.txt"}, &File{Name: "b.txt"}, changes, a, b, DefaultContextLines)
		e := NewUnifiedEncoder(os.Stderr)
		e.SetColor(color.NewColorConfig())
		_ = e.Encode([]*Patch{u})

		t.Logf("Result: %d changes (expected: 2 changes - one per block)", len(changes))
	}
}
