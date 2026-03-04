// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestSizeUnmarshalText(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"100", 100},
		{"1k", 1024},
		{"1K", 1024},
		{"1m", 1024 * 1024},
		{"1M", 1024 * 1024},
		{"1g", 1024 * 1024 * 1024},
		{"1G", 1024 * 1024 * 1024},
		{"10m", 10 * 1024 * 1024},
		{"10M", 10 * 1024 * 1024},
		{"10mb", 10 * 1024 * 1024},
		{"10MB", 10 * 1024 * 1024},
		{"512k", 512 * 1024},
		{"512kb", 512 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			var s Size
			if err := s.UnmarshalText([]byte(tt.input)); err != nil {
				t.Fatalf("UnmarshalText(%q) error: %v", tt.input, err)
			}
			if int64(s) != tt.expected {
				t.Errorf("UnmarshalText(%q) = %d, want %d", tt.input, s, tt.expected)
			}
		})
	}
}

func TestSizeTOMLDecode(t *testing.T) {
	type Config struct {
		Threshold Size `toml:"threshold"`
		Size      Size `toml:"size"`
	}

	tests := []struct {
		name     string
		input    string
		expected Config
	}{
		{
			name: "basic sizes",
			input: `
threshold = "10m"
size = "100m"
`,
			expected: Config{
				Threshold: 10 * 1024 * 1024,
				Size:      100 * 1024 * 1024,
			},
		},
		{
			name: "with B suffix",
			input: `
threshold = "10MB"
size = "100GB"
`,
			expected: Config{
				Threshold: 10 * 1024 * 1024,
				Size:      100 * 1024 * 1024 * 1024,
			},
		},
		{
			name: "numeric values",
			input: `
threshold = "1024"
size = "2048"
`,
			expected: Config{
				Threshold: 1024,
				Size:      2048,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Config
			if _, err := toml.Decode(tt.input, &c); err != nil {
				t.Fatalf("Decode error: %v", err)
			}
			if c.Threshold != tt.expected.Threshold {
				t.Errorf("Threshold = %d, want %d", c.Threshold, tt.expected.Threshold)
			}
			if c.Size != tt.expected.Size {
				t.Errorf("Size = %d, want %d", c.Size, tt.expected.Size)
			}
		})
	}
}

func TestSizeInFragment(t *testing.T) {
	input := `
[fragment]
threshold = "2g"
size = "5g"
`
	var cfg Config
	if _, err := toml.Decode(input, &cfg); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	// Check raw values
	if cfg.Fragment.ThresholdRaw != 2*1024*1024*1024 {
		t.Errorf("ThresholdRaw = %d, want %d", cfg.Fragment.ThresholdRaw, 2*1024*1024*1024)
	}
	if cfg.Fragment.SizeRaw != 5*1024*1024*1024 {
		t.Errorf("SizeRaw = %d, want %d", cfg.Fragment.SizeRaw, 5*1024*1024*1024)
	}

	// Check computed values
	if expected := int64(2 * 1024 * 1024 * 1024); cfg.Fragment.Threshold() != expected {
		t.Errorf("Threshold() = %d, want %d", cfg.Fragment.Threshold(), expected)
	}
	if expected := int64(5 * 1024 * 1024 * 1024); cfg.Fragment.Size() != expected {
		t.Errorf("Size() = %d, want %d", cfg.Fragment.Size(), expected)
	}
}

func TestSizeDefault(t *testing.T) {
	// When not set, should use defaults
	input := `
[fragment]
`
	var cfg Config
	if _, err := toml.Decode(input, &cfg); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	// Check defaults are used when values are 0
	if cfg.Fragment.Threshold() != FragmentThreshold {
		t.Errorf("Threshold() = %d, want default %d", cfg.Fragment.Threshold(), FragmentThreshold)
	}
	if cfg.Fragment.Size() != FragmentSize {
		t.Errorf("Size() = %d, want default %d", cfg.Fragment.Size(), FragmentSize)
	}
}

func TestSizeInTransport(t *testing.T) {
	input := `
[transport]
largeSize = "10m"
maxEntries = 8
`
	var cfg Config
	if _, err := toml.Decode(input, &cfg); err != nil {
		t.Fatalf("Decode error: %v", err)
	}

	if cfg.Transport.LargeSizeRaw != 10*1024*1024 {
		t.Errorf("LargeSizeRaw = %d, want %d", cfg.Transport.LargeSizeRaw, 10*1024*1024)
	}
	if cfg.Transport.MaxEntries != 8 {
		t.Errorf("MaxEntries = %d, want 8", cfg.Transport.MaxEntries)
	}

	// Check computed value
	if expected := int64(10 * 1024 * 1024); cfg.Transport.LargeSize() != expected {
		t.Errorf("LargeSize() = %d, want %d", cfg.Transport.LargeSize(), expected)
	}
}
