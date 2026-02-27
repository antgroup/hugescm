package diferenco

import (
	"context"
	"testing"
)

// TestNewHasConflict tests the NewHasConflict function
func TestNewHasConflict(t *testing.T) {
	tests := []struct {
		name      string
		textO     string
		textA     string
		textB     string
		wantTrue  bool
		expectErr bool
	}{
		{
			name:      "no_conflict_only_a_changed",
			textO:     "line1\nline2\nline3\n",
			textA:     "line1a\nline2\nline3\n",
			textB:     "line1\nline2\nline3\n",
			wantTrue:  false,
			expectErr: false,
		},
		{
			name:      "no_conflict_only_b_changed",
			textO:     "line1\nline2\nline3\n",
			textA:     "line1\nline2\nline3\n",
			textB:     "line1\nline2b\nline3\n",
			wantTrue:  false,
			expectErr: false,
		},
		{
			name:      "no_conflict_both_same_change",
			textO:     "line1\nline2\nline3\n",
			textA:     "line1a\nline2\nline3\n",
			textB:     "line1a\nline2\nline3\n",
			wantTrue:  false,
			expectErr: false,
		},
		{
			name:      "conflict_same_line_different_content",
			textO:     "line1\nline2\nline3\n",
			textA:     "line1\nline2a\nline3\n",
			textB:     "line1\nline2b\nline3\n",
			wantTrue:  true,
			expectErr: false,
		},
		{
			name:      "conflict_different_lines_adjacent",
			textO:     "line1\nline2\nline3\n",
			textA:     "line1a\nline2\nline3\n",
			textB:     "line1\nline2b\nline3\n",
			wantTrue:  true,
			expectErr: false,
		},
		{
			name:      "conflict_adjacent_changes",
			textO:     "line1\nline2\nline3\n",
			textA:     "line1a\nline2\nline3\n",
			textB:     "line1b\nline2\nline3\n",
			wantTrue:  true,
			expectErr: false,
		},
		{
			name:      "no_conflict_all_same",
			textO:     "line1\nline2\nline3\n",
			textA:     "line1\nline2\nline3\n",
			textB:     "line1\nline2\nline3\n",
			wantTrue:  false,
			expectErr: false,
		},
		{
			name:      "no_conflict_empty_texts",
			textO:     "",
			textA:     "",
			textB:     "",
			wantTrue:  false,
			expectErr: false,
		},
		{
			name:      "conflict_empty_origin",
			textO:     "",
			textA:     "line1\n",
			textB:     "line2\n",
			wantTrue:  true,
			expectErr: false,
		},
		{
			name:      "conflict_insert_at_same_position_different_content",
			textO:     "line1\nline3\n",
			textA:     "line1\nline2a\nline3\n",
			textB:     "line1\nline2b\nline3\n",
			wantTrue:  true,
			expectErr: false,
		},
		{
			name:      "conflict_insert_at_same_position",
			textO:     "line1\nline3\n",
			textA:     "line1\nline2a\nline3\n",
			textB:     "line1\nline2b\nline3\n",
			wantTrue:  true,
			expectErr: false,
		},
		{
			name:      "no_conflict_delete_same_line",
			textO:     "line1\nline2\nline3\n",
			textA:     "line1\nline3\n",
			textB:     "line1\nline3\n",
			wantTrue:  false,
			expectErr: false,
		},
		{
			name:      "conflict_delete_different_lines",
			textO:     "line1\nline2\nline3\n",
			textA:     "line1\nline3\n",
			textB:     "line1\nline2\n",
			wantTrue:  true,
			expectErr: false,
		},
		{
			name:      "no_conflict_single_line",
			textO:     "line1\n",
			textA:     "line1\n",
			textB:     "line1\n",
			wantTrue:  false,
			expectErr: false,
		},
		{
			name:      "conflict_single_line",
			textO:     "line1\n",
			textA:     "line1a\n",
			textB:     "line1b\n",
			wantTrue:  true,
			expectErr: false,
		},
		{
			name:      "no_conflict_multiple_changes_separated",
			textO:     "line1\nline2\nline3\nline4\nline5\n",
			textA:     "line1a\nline2\nline3\nline4\nline5\n",
			textB:     "line1\nline2\nline3\nline4b\nline5\n",
			wantTrue:  false,
			expectErr: false,
		},
		{
			name:      "conflict_multiple_overlapping_changes",
			textO:     "line1\nline2\nline3\nline4\nline5\n",
			textA:     "line1\nline2a\nline3a\nline4\nline5\n",
			textB:     "line1\nline2b\nline3b\nline4\nline5\n",
			wantTrue:  true,
			expectErr: false,
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewHasConflict(ctx, tt.textO, tt.textA, tt.textB)

			if (err != nil) != tt.expectErr {
				t.Errorf("NewHasConflict() error = %v, expectErr %v", err, tt.expectErr)
				return
			}

			if got != tt.wantTrue {
				t.Errorf("NewHasConflict() = %v, want %v", got, tt.wantTrue)
			}
		})
	}
}

// TestNewHasConflictVsMerge tests that NewHasConflict is consistent with NewMerge
func TestNewHasConflictVsMerge(t *testing.T) {
	tests := []struct {
		name  string
		textO string
		textA string
		textB string
	}{
		{
			name:  "simple_conflict",
			textO: "line1\nline2\nline3\n",
			textA: "line1\nline2a\nline3\n",
			textB: "line1\nline2b\nline3\n",
		},
		{
			name:  "no_conflict",
			textO: "line1\nline2\nline3\n",
			textA: "line1\nline2a\nline3\n",
			textB: "line1\nline2\nline3\n",
		},
		{
			name:  "adjacent_changes",
			textO: "line1\nline2\nline3\n",
			textA: "line1a\nline2\nline3\n",
			textB: "line1b\nline2\nline3\n",
		},
		{
			name:  "same_change",
			textO: "line1\nline2\nline3\n",
			textA: "line1\nline2a\nline3\n",
			textB: "line1\nline2a\nline3\n",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Check with NewHasConflict
			hasConflict, err := NewHasConflict(ctx, tt.textO, tt.textA, tt.textB)
			if err != nil {
				t.Fatalf("NewHasConflict() error = %v", err)
			}

			// Check with NewMerge
			opts := &MergeOptions{
				TextO: tt.textO,
				TextA: tt.textA,
				TextB: tt.textB,
				Style: STYLE_DEFAULT,
				A:     Histogram,
			}
			_, mergeHasConflict, err := NewMerge(ctx, opts)
			if err != nil {
				t.Fatalf("NewMerge() error = %v", err)
			}

			// They should match
			if hasConflict != mergeHasConflict {
				t.Errorf("NewHasConflict() = %v, NewMerge() = %v, should match",
					hasConflict, mergeHasConflict)
			}
		})
	}
}

// TestNewHasConflictContextCancellation tests context cancellation
func TestNewHasConflictContextCancellation(t *testing.T) {
	tests := []struct {
		name         string
		cancelBefore bool
		cancelDuring bool
		expectError  bool
	}{
		{
			name:         "cancel_before_merge",
			cancelBefore: true,
			expectError:  true,
		},
		{
			name:         "no_cancellation",
			cancelBefore: false,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())

			if tt.cancelBefore {
				cancel()
			}

			textO := "line1\nline2\nline3\n"
			textA := "line1\nline2a\nline3\n"
			textB := "line1\nline2\nline3\n"

			_, err := NewHasConflict(ctx, textO, textA, textB)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}

			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			cancel()
		})
	}
}
