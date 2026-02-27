package diferenco

import (
	"context"
	"strings"
	"testing"
)

// TestNewMergeBasic tests basic merge scenarios
func TestNewMergeBasic(t *testing.T) {
	tests := []struct {
		name         string
		origin       string
		ours         string
		theirs       string
		style        int
		wantConflict bool
	}{
		{
			name:         "conflict_default",
			origin:       "line1\nline2\n",
			ours:         "line1a\nline2\n",
			theirs:       "line1b\nline2\n",
			style:        STYLE_DEFAULT,
			wantConflict: true,
		},
		{
			name:         "conflict_diff3",
			origin:       "line1\nline2\n",
			ours:         "line1a\nline2\n",
			theirs:       "line1b\nline2\n",
			style:        STYLE_DIFF3,
			wantConflict: true,
		},
		{
			name:         "no_conflict_adjacent",
			origin:       "line1\nline2\n",
			ours:         "line1a\nline2\n",
			theirs:       "line1\nline2a\n",
			style:        STYLE_DEFAULT,
			wantConflict: true, // Adjacent changes are still conflicts
		},
		{
			name:         "empty_texts",
			origin:       "",
			ours:         "",
			theirs:       "",
			style:        STYLE_DEFAULT,
			wantConflict: false,
		},
		{
			name:         "ours_empty",
			origin:       "line1\nline2\n",
			ours:         "",
			theirs:       "line1\nline2\n",
			style:        STYLE_DEFAULT,
			wantConflict: false,
		},
		{
			name:         "theirs_empty",
			origin:       "line1\nline2\n",
			ours:         "line1\nline2\n",
			theirs:       "",
			style:        STYLE_DEFAULT,
			wantConflict: false,
		},
		{
			name:         "same_change_both",
			origin:       "line1\nline2\n",
			ours:         "line1a\nline2\n",
			theirs:       "line1a\nline2\n",
			style:        STYLE_DEFAULT,
			wantConflict: false, // Same change should not conflict
		},
		{
			name:         "only_ours_changed",
			origin:       "line1\nline2\n",
			ours:         "line1a\nline2\n",
			theirs:       "line1\nline2\n",
			style:        STYLE_DEFAULT,
			wantConflict: false,
		},
		{
			name:         "only_theirs_changed",
			origin:       "line1\nline2\n",
			ours:         "line1\nline2\n",
			theirs:       "line1b\nline2\n",
			style:        STYLE_DEFAULT,
			wantConflict: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			opts := &MergeOptions{
				TextO: tt.origin,
				TextA: tt.ours,
				TextB: tt.theirs,
				Style: tt.style,
				A:     Histogram,
			}

			result, hasConflict, err := NewMerge(ctx, opts)
			if err != nil {
				t.Fatalf("NewMerge() error = %v", err)
			}

			if hasConflict != tt.wantConflict {
				t.Errorf("NewMerge() hasConflict = %v, want %v", hasConflict, tt.wantConflict)
			}

			// Verify conflict markers are present when expected
			if tt.wantConflict && !strings.Contains(result, "<<<<<<<") {
				t.Errorf("NewMerge() result should contain conflict markers when hasConflict=true")
			}

			if !tt.wantConflict && strings.Contains(result, "<<<<<<<") {
				t.Errorf("NewMerge() result should not contain conflict markers when hasConflict=false")
			}
		})
	}
}

// TestNewMergeVsMerge compares NewMerge with original Merge
func TestNewMergeVsMerge(t *testing.T) {
	tests := []struct {
		name   string
		origin string
		ours   string
		theirs string
		style  int
	}{
		{
			name:   "simple_conflict",
			origin: "line1\nline2\n",
			ours:   "line1a\nline2\n",
			theirs: "line1b\nline2\n",
			style:  STYLE_DEFAULT,
		},
		{
			name:   "adjacent_conflict",
			origin: "line1\nline2\n",
			ours:   "line1a\nline2\n",
			theirs: "line1\nline2a\n",
			style:  STYLE_DEFAULT,
		},
		{
			name:   "diff3_style",
			origin: "line1\nline2\n",
			ours:   "line1a\nline2\n",
			theirs: "line1b\nline2\n",
			style:  STYLE_DIFF3,
		},
		{
			name:   "no_change_ours",
			origin: "line1\nline2\n",
			ours:   "line1\nline2\n",
			theirs: "line1b\nline2\n",
			style:  STYLE_DEFAULT,
		},
		{
			name:   "no_change_theirs",
			origin: "line1\nline2\n",
			ours:   "line1a\nline2\n",
			theirs: "line1\nline2\n",
			style:  STYLE_DEFAULT,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()

			// Original Merge
			optsOriginal := &MergeOptions{
				TextO: tt.origin,
				TextA: tt.ours,
				TextB: tt.theirs,
				Style: tt.style,
				A:     Histogram,
			}
			resultOriginal, conflictOriginal, errOriginal := Merge(ctx, optsOriginal)
			if errOriginal != nil {
				t.Fatalf("Merge() error = %v", errOriginal)
			}

			// NewMerge
			optsNew := &MergeOptions{
				TextO: tt.origin,
				TextA: tt.ours,
				TextB: tt.theirs,
				Style: tt.style,
				A:     Histogram,
			}
			resultNew, conflictNew, errNew := NewMerge(ctx, optsNew)
			if errNew != nil {
				t.Fatalf("NewMerge() error = %v", errNew)
			}

			// Compare conflict flags
			if conflictOriginal != conflictNew {
				t.Errorf("Conflict mismatch: Merge=%v, NewMerge=%v", conflictOriginal, conflictNew)
			}

			// Compare results
			if resultOriginal != resultNew {
				t.Errorf("Results differ:\nOriginal:\n%s\n\nNewMerge:\n%s", resultOriginal, resultNew)
			}
		})
	}
}

// TestNewMergeLabels tests label formatting
func TestNewMergeLabels(t *testing.T) {
	tests := []struct {
		name      string
		labelO    string
		labelA    string
		labelB    string
		wantLabel string
	}{
		{
			name:      "default_labels",
			labelO:    "o.txt",
			labelA:    "a.txt",
			labelB:    "b.txt",
			wantLabel: " a.txt", // ValidateOptions adds space
		},
		{
			name:      "empty_labels",
			labelO:    "",
			labelA:    "",
			labelB:    "",
			wantLabel: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			opts := &MergeOptions{
				TextO:  "line1\nline2\n",
				TextA:  "line1a\nline2\n",
				TextB:  "line1b\nline2\n",
				LabelO: tt.labelO,
				LabelA: tt.labelA,
				LabelB: tt.labelB,
				Style:  STYLE_DEFAULT,
				A:      Histogram,
			}

			result, _, err := NewMerge(ctx, opts)
			if err != nil {
				t.Fatalf("NewMerge() error = %v", err)
			}

			if tt.labelA != "" {
				if !strings.Contains(result, tt.wantLabel) {
					t.Errorf("NewMerge() result should contain label %q, got:\n%s", tt.wantLabel, result)
				}
			}
		})
	}
}

// TestNewMergeMultiLine tests multi-line merges
func TestNewMergeMultiLine(t *testing.T) {
	tests := []struct {
		name         string
		origin       string
		ours         string
		theirs       string
		wantConflict bool
	}{
		{
			name:         "multi_line_change",
			origin:       "line1\nline2\nline3\nline4\n",
			ours:         "line1\nline2a\nline3\nline4\n",
			theirs:       "line1\nline2\nline3b\nline4\n",
			wantConflict: true, // Adjacent modifications are treated as a conflict
		},
		{
			name:         "insert_middle",
			origin:       "line1\nline3\n",
			ours:         "line1\nline2\nline3\n",
			theirs:       "line1\nline2\nline3\n",
			wantConflict: false, // Same insert
		},
		{
			name:         "delete_middle",
			origin:       "line1\nline2\nline3\n",
			ours:         "line1\nline3\n",
			theirs:       "line1\nline3\n",
			wantConflict: false, // Same delete
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			opts := &MergeOptions{
				TextO: tt.origin,
				TextA: tt.ours,
				TextB: tt.theirs,
				Style: STYLE_DEFAULT,
				A:     Histogram,
			}

			_, hasConflict, err := NewMerge(ctx, opts)
			if err != nil {
				t.Fatalf("NewMerge() error = %v", err)
			}

			if hasConflict != tt.wantConflict {
				t.Errorf("NewMerge() hasConflict = %v, want %v", hasConflict, tt.wantConflict)
			}
		})
	}
}

// TestNewMergeContext tests context cancellation
func TestNewMergeContext(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel() // Cancel immediately

	opts := &MergeOptions{
		TextO: "line1\nline2\n",
		TextA: "line1a\nline2\n",
		TextB: "line1b\nline2\n",
		Style: STYLE_DEFAULT,
		A:     Histogram,
	}

	_, _, err := NewMerge(ctx, opts)
	if err == nil {
		t.Error("NewMerge() should return error when context is canceled")
	}
}

// TestNewMergeValidateOptions tests option validation
func TestNewMergeValidateOptions(t *testing.T) {
	tests := []struct {
		name    string
		opts    *MergeOptions
		wantErr bool
	}{
		{
			name:    "nil_options",
			opts:    nil,
			wantErr: true,
		},
		{
			name: "valid_options",
			opts: &MergeOptions{
				TextO: "line1\n",
				TextA: "line1a\n",
				TextB: "line1b\n",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, _, err := NewMerge(ctx, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewMerge() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestNewMergeAlgorithms tests different diff algorithms
func TestNewMergeAlgorithms(t *testing.T) {
	algorithms := []Algorithm{
		Histogram,
		Myers,
		ONP,
		Patience,
		Minimal,
	}

	for _, algo := range algorithms {
		t.Run(algo.String(), func(t *testing.T) {
			ctx := context.Background()
			opts := &MergeOptions{
				TextO: "line1\nline2\nline3\n",
				TextA: "line1a\nline2\nline3\n",
				TextB: "line1b\nline2\nline3\n",
				Style: STYLE_DEFAULT,
				A:     algo,
			}

			result, hasConflict, err := NewMerge(ctx, opts)
			if err != nil {
				t.Fatalf("NewMerge() with algorithm %s error = %v", algo, err)
			}

			if !hasConflict {
				t.Errorf("NewMerge() with algorithm %s should detect conflict", algo)
			}

			if !strings.Contains(result, "<<<<<<<") {
				t.Errorf("NewMerge() result should contain conflict markers")
			}
		})
	}
}

// TestNewMergeComplexConflicts tests complex conflict scenarios
func TestNewMergeComplexConflicts(t *testing.T) {
	tests := []struct {
		name         string
		origin       string
		ours         string
		theirs       string
		wantConflict bool
	}{
		{
			name:         "both_delete_same",
			origin:       "line1\nline2\nline3\n",
			ours:         "line1\nline3\n",
			theirs:       "line1\nline3\n",
			wantConflict: false, // Same delete, no conflict
		},
		{
			name:         "both_delete_different",
			origin:       "line1\nline2\nline3\n",
			ours:         "line1\nline3\n",
			theirs:       "line1\nline2\n",
			wantConflict: true, // Different deletions create conflict
		},
		{
			name:         "both_insert_same_place",
			origin:       "line1\nline3\n",
			ours:         "line1\nline2\nline3\n",
			theirs:       "line1\nline2a\nline3\n",
			wantConflict: true, // Different insert at same place
		},
		{
			name:         "replace_same_content",
			origin:       "line1\nline2\nline3\n",
			ours:         "line1\nline2a\nline3\n",
			theirs:       "line1\nline2a\nline3\n",
			wantConflict: false, // Same replacement, no conflict
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			opts := &MergeOptions{
				TextO: tt.origin,
				TextA: tt.ours,
				TextB: tt.theirs,
				Style: STYLE_DEFAULT,
				A:     Histogram,
			}

			_, hasConflict, err := NewMerge(ctx, opts)
			if err != nil {
				t.Fatalf("NewMerge() error = %v", err)
			}

			if hasConflict != tt.wantConflict {
				t.Errorf("NewMerge() hasConflict = %v, want %v", hasConflict, tt.wantConflict)
			}
		})
	}
}

// TestNewMergeEmptyRegion tests edge cases with empty regions
func TestNewMergeEmptyRegion(t *testing.T) {
	tests := []struct {
		name         string
		origin       string
		ours         string
		theirs       string
		wantConflict bool
	}{
		{
			name:         "ours_insert_at_beginning",
			origin:       "line1\nline2\n",
			ours:         "line0\nline1\nline2\n",
			theirs:       "line1\nline2\n",
			wantConflict: false,
		},
		{
			name:         "theirs_insert_at_end",
			origin:       "line1\nline2\n",
			ours:         "line1\nline2\n",
			theirs:       "line1\nline2\nline3\n",
			wantConflict: false,
		},
		{
			name:         "both_insert_at_beginning_different",
			origin:       "line1\nline2\n",
			ours:         "line0a\nline1\nline2\n",
			theirs:       "line0b\nline1\nline2\n",
			wantConflict: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			opts := &MergeOptions{
				TextO: tt.origin,
				TextA: tt.ours,
				TextB: tt.theirs,
				Style: STYLE_DEFAULT,
				A:     Histogram,
			}

			_, hasConflict, err := NewMerge(ctx, opts)
			if err != nil {
				t.Fatalf("NewMerge() error = %v", err)
			}

			if hasConflict != tt.wantConflict {
				t.Errorf("NewMerge() hasConflict = %v, want %v", hasConflict, tt.wantConflict)
			}
		})
	}
}
