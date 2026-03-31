package diferenco

import "testing"

func TestPatchNameHandlesNilSides(t *testing.T) {
	tests := []struct {
		name string
		p    Patch
		want string
	}{
		{
			name: "both_non_nil_prefers_to",
			p: Patch{
				From: &File{Name: "old.txt"},
				To:   &File{Name: "new.txt"},
			},
			want: "new.txt",
		},
		{
			name: "from_nil_returns_to",
			p: Patch{
				To: &File{Name: "new.txt"},
			},
			want: "new.txt",
		},
		{
			name: "to_nil_returns_from",
			p: Patch{
				From: &File{Name: "old.txt"},
			},
			want: "old.txt",
		},
		{
			name: "both_nil_returns_empty",
			p:    Patch{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.p.Name(); got != tt.want {
				t.Fatalf("Patch.Name() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateOptionsIdempotent(t *testing.T) {
	opts := &MergeOptions{
		LabelO: "base",
		LabelA: "ours",
		LabelB: "theirs",
		A:      Unspecified,
	}

	if err := opts.ValidateOptions(); err != nil {
		t.Fatalf("ValidateOptions() first call error: %v", err)
	}
	firstO, firstA, firstB := opts.LabelO, opts.LabelA, opts.LabelB

	if err := opts.ValidateOptions(); err != nil {
		t.Fatalf("ValidateOptions() second call error: %v", err)
	}

	if opts.LabelO != firstO || opts.LabelA != firstA || opts.LabelB != firstB {
		t.Fatalf("ValidateOptions() should be idempotent, got (%q, %q, %q), want (%q, %q, %q)",
			opts.LabelO, opts.LabelA, opts.LabelB, firstO, firstA, firstB)
	}
	if opts.A != Histogram {
		t.Fatalf("ValidateOptions() should default algorithm to Histogram, got %v", opts.A)
	}
}
