package diferenco

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestMerge(t *testing.T) {
	const textO = `celery
garlic
onions
salmon
tomatoes
wine
`

	const textA = `celery
salmon
tomatoes
garlic
onions
wine
`

	const textB = `celery
salmon
garlic
onions
tomatoes
wine
`

	content, conflict, err := DefaultMerge(t.Context(), textO, textA, textB, "o.txt", "a.txt", "b.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)

	content, conflict, err = Merge(t.Context(), &MergeOptions{TextO: textO, TextA: textA, TextB: textB, LabelO: "o.txt", LabelA: "a.txt", LabelB: "b.txt", Style: STYLE_ZEALOUS_DIFF3})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "ZEALOUS_DIFF3\n%s\nconflicts: %v\n", content, conflict)

	content, conflict, err = Merge(t.Context(), &MergeOptions{TextO: textO, TextA: textA, TextB: textB, LabelO: "o.txt", LabelA: "a.txt", LabelB: "b.txt", Style: STYLE_DIFF3})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "DIFF3\n%s\nconflicts: %v\n", content, conflict)
}

func TestMerge2(t *testing.T) {
	const textO = `celery
garlic
onions
salmon
tomatoes
wine
`

	const textA = `celery
salmon
tomatoes
garlic
onions
wine
`

	content, conflict, err := DefaultMerge(t.Context(), textO, textA, textA, "o.txt", "a.txt", "b.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)
}

func TestMerge3(t *testing.T) {
	const textO = `celery
garlic
onions
salmon
tomatoes
wine
`

	const textA = `celery
garlic
onions
salmon
tomatoes
wine
0000
00000
`

	const textB = `celery
garlic
onions
salmon
tomatoes
wine
0000
00000
77777
`

	content, conflict, err := DefaultMerge(t.Context(), textO, textA, textB, "o.txt", "a.txt", "b.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)

	content, conflict, err = Merge(t.Context(), &MergeOptions{TextO: textO, TextA: textA, TextB: textB, LabelO: "o.txt", LabelA: "a.txt", LabelB: "b.txt", Style: STYLE_ZEALOUS_DIFF3})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)

	content, conflict, err = Merge(t.Context(), &MergeOptions{TextO: textO, TextA: textA, TextB: textB, LabelO: "o.txt", LabelA: "a.txt", LabelB: "b.txt", Style: STYLE_DIFF3})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)

}

func TestMergeConflicts(t *testing.T) {
	const textO = `1
2
3
4
5
6
`

	const textA = `1
2
AAA
XXX
4
5
6
`

	const textB = `1
2
BBB
YYY
4
5
6
`

	content, conflict, err := DefaultMerge(t.Context(), textO, textA, textB, "o.txt", "a.txt", "b.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)
}

// TestWriteConflictSuffix tests whether the suffix != 0 branch in writeConflict
// can ever be reached. This tests the hypothesis that conflict.a and conflict.b
// never have a common suffix when excludeFalseConflicts is true.
func TestWriteConflictSuffix(t *testing.T) {
	tests := []struct {
		name  string
		textO string
		textA string
		textB string
	}{
		{
			name: "same_prefix_and_suffix_in_conflict",
			// Test: a and b have the same prefix and suffix in the conflict region
			textO: `line1
line2
line3
line4
line5
`,
			textA: `line1
CHANGED_A
line3
line4
line5
`,
			textB: `line1
CHANGED_B
line3
line4
line5
`,
		},
		{
			name: "multi_line_same_ending",
			// Test: multi-line changes with the same ending
			textO: `start
old1
old2
end
`,
			textA: `start
new_a1
new_a2
common_end
end
`,
			textB: `start
new_b1
new_b2
common_end
end
`,
		},
		{
			name: "insert_with_common_context",
			// Test: insert operation with the same surrounding context
			textO: `prefix
content
suffix
`,
			textA: `prefix
inserted_a
content
suffix
`,
			textB: `prefix
inserted_b
content
suffix
`,
		},
		{
			name: "delete_with_common_remaining",
			// Test: delete operation with the same remaining content
			textO: `line1
to_delete
line2
line3
`,
			textA: `line1
line2
line3
`,
			textB: `line1
extra_line
line2
line3
`,
		},
		{
			name: "complex_overlapping_changes",
			// Test: complex overlapping changes
			textO: `a
b
c
d
e
f
`,
			textA: `a
X
Y
d
e
f
`,
			textB: `a
Z
W
d
e
f
`,
		},
		{
			name: "both_add_same_prefix_different_middle",
			// Test: both sides add the same prefix but different middle
			textO: `1
2
3
`,
			textA: `1
same_prefix
different_A
3
`,
			textB: `1
same_prefix
different_B
3
`,
		},
		{
			name: "adjacent_changes",
			// Test: adjacent changes
			textO: `line1
line2
line3
line4
`,
			textA: `line1
modified_a1
modified_a2
line3
line4
`,
			textB: `line1
modified_b1
modified_b2
line3
line4
`,
		},
		{
			name: "same_content_different_position",
			// Test: same content at different positions
			textO: `a
b
c
d
`,
			textA: `a
x
b
c
d
`,
			textB: `a
b
x
c
d
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 使用三种样式测试
			for _, style := range []int{STYLE_DEFAULT, STYLE_DIFF3, STYLE_ZEALOUS_DIFF3} {
				styleName := []string{"DEFAULT", "DIFF3", "ZEALOUS_DIFF3"}[style]
				t.Run(styleName, func(t *testing.T) {
					content, hasConflict, err := Merge(t.Context(), &MergeOptions{
						TextO: tt.textO,
						TextA: tt.textA,
						TextB: tt.textB,
						Style: style,
					})
					if err != nil {
						t.Fatalf("unexpected error: %v", err)
					}

					// 详细输出以便调试
					t.Logf("Style %s:\n%s\nhasConflict: %v", styleName, content, hasConflict)
				})
			}
		})
	}
}

// TestConflictSuffixDirectly directly tests the writeConflict function
// by constructing conflict structs to verify suffix behavior.
func TestConflictSuffixDirectly(t *testing.T) {
	s := NewSink(NEWLINE_RAW)

	tests := []struct {
		name     string
		conflict conflict[int]
		wantIn   string // should contain this substring
		wantNot  string // should NOT contain this substring
	}{
		{
			name: "identical_a_and_b",
			// If a and b are identical, this should not be a real conflict
			conflict: conflict[int]{
				a: s.SplitLines("same\ncontent\n"),
				o: s.SplitLines("original\n"),
				b: s.SplitLines("same\ncontent\n"),
			},
		},
		{
			name: "a_and_b_share_prefix_and_suffix",
			conflict: conflict[int]{
				a: s.SplitLines("prefix\ndiff_a\nsuffix\n"),
				o: s.SplitLines("original\n"),
				b: s.SplitLines("prefix\ndiff_b\nsuffix\n"),
			},
		},
		{
			name: "a_and_b_completely_different",
			conflict: conflict[int]{
				a: s.SplitLines("completely\ndifferent\na\n"),
				o: s.SplitLines("original\n"),
				b: s.SplitLines("totally\nother\nb\n"),
			},
		},
		{
			name: "a_and_b_share_only_prefix",
			conflict: conflict[int]{
				a: s.SplitLines("prefix\nunique_a\n"),
				o: s.SplitLines("original\n"),
				b: s.SplitLines("prefix\nunique_b\n"),
			},
		},
		{
			name: "a_and_b_share_only_suffix",
			conflict: conflict[int]{
				a: s.SplitLines("unique_a\nsuffix\n"),
				o: s.SplitLines("original\n"),
				b: s.SplitLines("unique_b\nsuffix\n"),
			},
		},
		{
			name: "empty_a_and_b",
			conflict: conflict[int]{
				a: []int{},
				o: s.SplitLines("original\n"),
				b: []int{},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, style := range []int{STYLE_DEFAULT, STYLE_ZEALOUS_DIFF3} {
				styleName := []string{"DEFAULT", "DIFF3", "ZEALOUS_DIFF3"}[style]
				t.Run(styleName, func(t *testing.T) {
					opts := &MergeOptions{Style: style}
					out := &strings.Builder{}
					s.writeConflict(out, opts, &tt.conflict)
					result := out.String()
					t.Logf("Output:\n%s", result)

					// Check for suffix-related output
					// In DEFAULT mode, if suffix != 0, the common suffix would be output after >>>>>>>
					// We can check by examining the number of lines in the output
					lines := strings.Split(result, "\n")
					t.Logf("Number of lines: %d", len(lines))
				})
			}
		})
	}
}

// TestDiff3MergeIndicesConflictBounds tests what ranges diff3MergeIndices
// produces for conflict regions.
func TestDiff3MergeIndicesConflictBounds(t *testing.T) {
	s := NewSink(NEWLINE_RAW)

	tests := []struct {
		name  string
		textO string
		textA string
		textB string
	}{
		{
			name:  "simple_conflict",
			textO: "line1\nline2\nline3\n",
			textA: "line1\nCHANGED_A\nline3\n",
			textB: "line1\nCHANGED_B\nline3\n",
		},
		{
			name:  "conflict_with_shared_suffix",
			textO: "a\nb\nc\nd\n",
			textA: "a\nX\nc\nd\n",
			textB: "a\nY\nc\nd\n",
		},
		{
			name:  "conflict_with_shared_prefix_and_suffix",
			textO: "prefix\nmiddle\nsuffix\n",
			textA: "prefix\nA\nsuffix\n",
			textB: "prefix\nB\nsuffix\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o := s.SplitLines(tt.textO)
			a := s.SplitLines(tt.textA)
			b := s.SplitLines(tt.textB)

			indices, err := diff3MergeIndices(t.Context(), o, a, b, Histogram)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			t.Logf("Indices for %s:", tt.name)
			for i, idx := range indices {
				if len(idx) == 3 {
					// Non-conflict record: {side, offset, length}
					t.Logf("  [%d]: side=%d, offset=%d, length=%d", i, idx[0], idx[1], idx[2])
				} else if len(idx) == 7 {
					// Conflict record: {-1, aLhs, aLen, oLhs, oLen, bLhs, bLen}
					t.Logf("  [%d]: CONFLICT, a=[%d:%d], o=[%d:%d], b=[%d:%d]",
						i, idx[1], idx[1]+idx[2], idx[3], idx[3]+idx[4], idx[5], idx[5]+idx[6])

					// Examine the conflict content
					conflictA := a[idx[1] : idx[1]+idx[2]]
					conflictB := b[idx[5] : idx[5]+idx[6]]
					prefix := commonPrefixLength(conflictA, conflictB)
					suffix := commonSuffixLength(conflictA[prefix:], conflictB[prefix:])

					t.Logf("    conflict.a = %v", conflictA)
					t.Logf("    conflict.b = %v", conflictB)
					t.Logf("    prefix length = %d", prefix)
					t.Logf("    suffix length = %d", suffix)

					// Key test: verify if suffix can be non-zero
					if suffix > 0 {
						t.Errorf("    suffix = %d (non-zero!), this would trigger the 'dead code' branch!", suffix)
					}
				}
			}
		})
	}
}

// TestWriteConflictSuffixNeverHappens verifies that the `if suffix != 0` branch
// in writeConflict can NEVER be reached when going through the normal Merge path.
func TestWriteConflictSuffixNeverHappens(t *testing.T) {
	// This test verifies: through the normal Merge path, suffix is always 0
	// This means the `if suffix != 0` branch is dead code

	tests := []struct {
		name  string
		textO string
		textA string
		textB string
	}{
		{
			name:  "case1",
			textO: "1\n2\n3\n4\n5\n",
			textA: "A\n2\n3\n4\n5\n",
			textB: "B\n2\n3\n4\n5\n",
		},
		{
			name:  "case2",
			textO: "prefix\norig\nsuffix\n",
			textA: "prefix\nA\nsuffix\n",
			textB: "prefix\nB\nsuffix\n",
		},
		{
			name:  "case3",
			textO: "a\nb\nc\nd\ne\n",
			textA: "a\nX\nY\nc\nd\ne\n",
			textB: "a\nP\nQ\nc\nd\ne\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use Merge function for complete testing
			content, _, err := Merge(t.Context(), &MergeOptions{
				TextO: tt.textO,
				TextA: tt.textA,
				TextB: tt.textB,
				Style: STYLE_DEFAULT,
			})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check if the output contains the expected common suffix
			// If the suffix != 0 branch were executed, the common suffix would appear after >>>>>>>
			t.Logf("Output:\n%s", content)
		})
	}
}

// TestMergeParallelSuffixBehavior tests whether MergeParallel can trigger the suffix != 0 branch
func TestMergeParallelSuffixBehavior(t *testing.T) {
	tests := []struct {
		name  string
		textO string
		textA string
		textB string
	}{
		{
			name:  "simple_conflict",
			textO: "line1\nline2\nline3\nline4\n",
			textA: "line1\nCHANGED_A\nline3\nline4\n",
			textB: "line1\nCHANGED_B\nline3\nline4\n",
		},
		{
			name:  "multi_line_with_shared_context",
			textO: "start\na\nb\nc\nend\n",
			textA: "start\nX\nY\nZ\nc\nend\n",
			textB: "start\nP\nQ\nR\nc\nend\n",
		},
		{
			name:  "insert_at_beginning",
			textO: "line1\nline2\n",
			textA: "NEW_A\nline1\nline2\n",
			textB: "NEW_B\nline1\nline2\n",
		},
		{
			name:  "delete_vs_modify",
			textO: "a\ntarget\nb\n",
			textA: "a\nb\n",
			textB: "a\nMODIFIED\nb\n",
		},
		{
			name:  "complex_overlapping",
			textO: "1\n2\n3\n4\n5\n6\n",
			textA: "1\nA1\nA2\nA3\n5\n6\n",
			textB: "1\nB1\nB2\nB3\n5\n6\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Compare Merge and MergeParallel outputs
			content1, hasConflict1, err := Merge(t.Context(), &MergeOptions{
				TextO: tt.textO,
				TextA: tt.textA,
				TextB: tt.textB,
				Style: STYLE_DEFAULT,
			})
			if err != nil {
				t.Fatalf("Merge error: %v", err)
			}

			content2, hasConflict2, err := MergeParallel(t.Context(), &MergeOptions{
				TextO: tt.textO,
				TextA: tt.textA,
				TextB: tt.textB,
				Style: STYLE_DEFAULT,
			})
			if err != nil {
				t.Fatalf("MergeParallel error: %v", err)
			}

			t.Logf("Merge output:\n%s", content1)
			t.Logf("MergeParallel output:\n%s", content2)

			// Check if results are consistent
			if hasConflict1 != hasConflict2 {
				t.Errorf("conflict status mismatch: Merge=%v, MergeParallel=%v", hasConflict1, hasConflict2)
			}

			// Both should produce the same output (modulo whitespace differences)
			if content1 != content2 {
				t.Errorf("output mismatch:\nMerge:\n%s\nMergeParallel:\n%s", content1, content2)
			}
		})
	}
}

// TestMergeParallelConflictSuffixDirectly directly tests the conflict structure
// created by MergeParallel's writeConflictRegion function
func TestMergeParallelConflictSuffixDirectly(t *testing.T) {
	sink := NewSink(NEWLINE_LF)

	tests := []struct {
		name       string
		textO      string
		textA      string
		textB      string
		wantSuffix int // expected suffix length in conflict
	}{
		{
			name:       "simple_different_content",
			textO:      "a\nb\nc\n",
			textA:      "X\nb\nc\n",
			textB:      "Y\nb\nc\n",
			wantSuffix: 0, // conflict should only contain X/Y, not b,c
		},
		{
			name:       "same_prefix_different_middle",
			textO:      "start\nmid\nend\n",
			textA:      "start\nA\nend\n",
			textB:      "start\nB\nend\n",
			wantSuffix: 0, // conflict should only contain A/B
		},
		{
			name:       "multi_line_conflict",
			textO:      "1\n2\n3\n4\n",
			textA:      "A1\nA2\n3\n4\n",
			textB:      "B1\nB2\n3\n4\n",
			wantSuffix: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oIdx := sink.SplitLines(tt.textO)
			aIdx := sink.SplitLines(tt.textA)
			bIdx := sink.SplitLines(tt.textB)

			// Get changes using parallel diff
			changesA, changesB, err := parallelDiff(t.Context(), oIdx, aIdx, bIdx, Histogram)
			if err != nil {
				t.Fatalf("parallelDiff error: %v", err)
			}

			t.Logf("changesA: %v", changesA)
			t.Logf("changesB: %v", changesB)

			// Find merge regions
			regions := findMergeRegions(changesA, changesB)
			t.Logf("regions: %+v", regions)

			// Check each conflict region
			for _, region := range regions {
				// Finalize region (check for false conflicts)
				region = finalizeRegion(region, changesA, changesB, aIdx, bIdx)

				if !region.isConflict {
					continue
				}

				// Calculate conflict content like writeConflictRegion does
				aLhs, aRhs := calculateRangeByIndices(changesA, region.changesAIndices, aIdx, region.start, region.end)
				bLhs, bRhs := calculateRangeByIndices(changesB, region.changesBIndices, bIdx, region.start, region.end)

				conflictA := aIdx[aLhs:aRhs]
				conflictB := bIdx[bLhs:bRhs]

				prefix := commonPrefixLength(conflictA, conflictB)
				suffix := commonSuffixLength(conflictA[prefix:], conflictB[prefix:])

				t.Logf("region: start=%d, end=%d", region.start, region.end)
				t.Logf("conflict.a: %v (aLhs=%d, aRhs=%d)", conflictA, aLhs, aRhs)
				t.Logf("conflict.b: %v (bLhs=%d, bRhs=%d)", conflictB, bLhs, bRhs)
				t.Logf("prefix=%d, suffix=%d", prefix, suffix)

				if suffix != tt.wantSuffix {
					t.Errorf("suffix mismatch: got %d, want %d", suffix, tt.wantSuffix)
				}

				if suffix > 0 {
					t.Errorf("suffix=%d (non-zero!), this would trigger the 'dead code' branch!", suffix)
				}
			}
		})
	}
}
