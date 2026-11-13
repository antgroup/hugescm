package diferenco

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// BenchmarkUnifiedToPatch exercises the full Unified pipeline (parse +
// algorithm + ToPatch) including hunk construction - the regression gate
// for hunk-building changes such as the hunkSection scan. The
// worst-ancestry case makes every section walk hit the full scan cap by
// filling the surrounding lines with digit-prefixed data.
func BenchmarkUnifiedToPatch(b *testing.B) {
	const totalLines = 20000
	const period = 40 // one change per 40 lines -> separate hunks at context 3

	for _, tc := range []struct {
		name string
		line func(i int) string
	}{
		{"typical-ancestry", func(i int) string {
			// Unique lines keep the minimal alignment unambiguous: exactly
			// one change per period, i.e. totalLines/period hunks.
			if i%period == 0 {
				return fmt.Sprintf("func decl%d() {", i)
			}
			return fmt.Sprintf("\tbody %d", i)
		}},
		{"worst-ancestry", func(i int) string {
			// never starts with an identifier rune: each hunk's section
			// scan walks the entire hunkSectionScanLimit window.
			return "0123456789 data"
		}},
	} {
		gen := func(mutate func(string) string) string {
			var sb strings.Builder
			sb.Grow(totalLines * 16)
			for i := 0; i < totalLines; i++ {
				s := tc.line(i)
				if i%period == 20 {
					s = mutate(s)
				}
				sb.WriteString(s)
				sb.WriteByte('\n')
			}
			return sb.String()
		}
		s1 := gen(func(s string) string { return s })
		s2 := gen(func(s string) string { return s + " CHANGED" })

		b.Run(tc.name, func(b *testing.B) {
			p, err := Unified(context.Background(), &Options{S1: s1, S2: s2})
			if err != nil {
				b.Fatal(err)
			}
			if len(p.Hunks) != totalLines/period {
				b.Fatalf("hunks = %d, want %d", len(p.Hunks), totalLines/period)
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := Unified(context.Background(), &Options{S1: s1, S2: s2}); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestUnifiedHunkSection verifies that ToPatch carries the enclosing context
// line into Hunk.Section: with zero context the three changes below live in
// separate hunks, and each hunk's section is the nearest identifier line
// above its change. Blank lines never stop the scan, and a change at the
// very top of a file yields no section.
func TestUnifiedHunkSection(t *testing.T) {
	s1 := strings.Join([]string{
		"package main", "",
		"func a() {", "\told();", "}", "",
		"func b() {", "\told();", "}", "",
		"func c() {", "\told();", "}",
	}, "\n")
	s2 := strings.ReplaceAll(s1, "old()", "new()")

	p, err := Unified(context.Background(), &Options{S1: s1, S2: s2, ContextLines: 0})
	if err != nil {
		t.Fatalf("Unified() error = %v", err)
	}
	if len(p.Hunks) != 3 {
		t.Fatalf("hunks = %d, want 3", len(p.Hunks))
	}
	for i, want := range []string{"func a() {", "func b() {", "func c() {"} {
		if got := p.Hunks[i].Section; got != want {
			t.Errorf("hunk %d section = %q, want %q", i, got, want)
		}
	}

	// A change directly below a blank line keeps scanning upward past it.
	p2, err := Unified(context.Background(), &Options{
		S1:           "func a() {\n\nold()\n",
		S2:           "func a() {\n\nnew()\n",
		ContextLines: 0,
	})
	if err != nil {
		t.Fatalf("Unified() error = %v", err)
	}
	if len(p2.Hunks) != 1 || p2.Hunks[0].Section != "func a() {" {
		t.Fatalf("blank-line skip: got %d hunk(s), section %q", len(p2.Hunks), p2.Hunks[0].Section)
	}

	// A change on the first line has nothing above it: no section.
	p3, err := Unified(context.Background(), &Options{
		S1:           "old\nsame\n",
		S2:           "new\nsame\n",
		ContextLines: 0,
	})
	if err != nil {
		t.Fatalf("Unified() error = %v", err)
	}
	if len(p3.Hunks) != 1 || p3.Hunks[0].Section != "" {
		t.Fatalf("file-top change: got %d hunk(s), section %q", len(p3.Hunks), p3.Hunks[0].Section)
	}

	// Indented lines never anchor the section (git's default funcname
	// heuristic): the scan walks past them, so a go.mod-style dependency
	// list is labeled "require (" instead of the entry above the change.
	p4, err := Unified(context.Background(), &Options{
		S1:           "require (\n\tfirst v1.0\n\tsecond v2.0\n\told v3.0\n",
		S2:           "require (\n\tfirst v1.0\n\tsecond v2.0\n\tnew v3.0\n",
		ContextLines: 0,
	})
	if err != nil {
		t.Fatalf("Unified() error = %v", err)
	}
	if len(p4.Hunks) != 1 || p4.Hunks[0].Section != "require (" {
		t.Fatalf("indented skip: got %d hunk(s), section %q", len(p4.Hunks), p4.Hunks[0].Section)
	}
}

// TestUnifiedContextLines verifies that Options.ContextLines controls the
// number of context lines: negative means DefaultContextLines, zero is an
// explicit no-context diff (git's -U0), larger values expand the context.
func TestUnifiedContextLines(t *testing.T) {
	// Two 10-line inputs with a single changed line (line 5).
	lines := func(changed string) string {
		var sb strings.Builder
		for i := 1; i <= 10; i++ {
			if i == 5 {
				sb.WriteString(changed)
			} else {
				sb.WriteString(fmt.Sprintf("line %d", i))
			}
			sb.WriteString("\n")
		}
		return sb.String()
	}
	s1, s2 := lines("the old text"), lines("the new text")

	countKind := func(p *Patch, kind Operation) int {
		n := 0
		for _, h := range p.Hunks {
			for _, l := range h.Lines {
				if l.Kind == kind {
					n++
				}
			}
		}
		return n
	}

	for _, tc := range []struct {
		name         string
		contextLines int
		wantEqual    int
	}{
		{"unset uses DefaultContextLines", -1, 2 * DefaultContextLines},
		{"zero context", 0, 0},
		{"explicit three", 3, 2 * 3},
		{"larger than the file", 10, 9}, // 4 lines before + 5 lines after
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, err := Unified(context.Background(), &Options{
				S1:           s1,
				S2:           s2,
				ContextLines: tc.contextLines,
			})
			if err != nil {
				t.Fatalf("Unified() error = %v", err)
			}
			if got := countKind(p, Equal); got != tc.wantEqual {
				t.Errorf("context %d: equal lines = %d, want %d", tc.contextLines, got, tc.wantEqual)
			}
			if got := countKind(p, Insert); got != 1 {
				t.Errorf("insert lines = %d, want 1", got)
			}
			if got := countKind(p, Delete); got != 1 {
				t.Errorf("delete lines = %d, want 1", got)
			}
		})
	}
}
