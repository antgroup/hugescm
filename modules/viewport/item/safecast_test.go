package item

import (
	"math"
	"testing"
)

func TestClampIntToUint8(t *testing.T) {
	tests := []struct {
		name string
		val  int
		want uint8
	}{
		{"zero", 0, 0},
		{"positive in range", 100, 100},
		{"max uint8", math.MaxUint8, math.MaxUint8},
		{"above max uint8", math.MaxUint8 + 1, math.MaxUint8},
		{"large positive", math.MaxInt, math.MaxUint8},
		{"negative", -1, 0},
		{"large negative", math.MinInt, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampIntToUint8(tt.val); got != tt.want {
				t.Errorf("clampIntToUint8(%d) = %d, want %d", tt.val, got, tt.want)
			}
		})
	}
}

func TestClampIntToUint32(t *testing.T) {
	tests := []struct {
		name string
		val  int
		want uint32
	}{
		{"zero", 0, 0},
		{"positive in range", 100, 100},
		{"negative", -1, 0},
		{"large negative", math.MinInt, 0},
	}

	// On 64-bit platforms, test that large values are clamped to MaxUint32
	if math.MaxInt > math.MaxUint32 {
		tests = append(tests, struct {
			name string
			val  int
			want uint32
		}{"max int clamped", math.MaxInt, math.MaxUint32})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clampIntToUint32(tt.val); got != tt.want {
				t.Errorf("clampIntToUint32(%d) = %d, want %d", tt.val, got, tt.want)
			}
		})
	}
}
