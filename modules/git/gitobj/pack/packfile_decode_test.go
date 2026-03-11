package pack

import (
	"bytes"
	"crypto/sha1"
	"crypto/sha256"
	"testing"
)

func TestDecodePackfileDecodesIntegerVersion(t *testing.T) {
	p, err := DecodePackfile(bytes.NewReader([]byte{
		'P', 'A', 'C', 'K', // Pack header.
		0x0, 0x0, 0x0, 0x2, // Pack version.
		0x0, 0x0, 0x0, 0x0, // Number of packed objects.
	}), sha1.New())

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if p.Version != 2 {
		t.Errorf("Expected %v, got %v", 2, p.Version)
	}
}

func TestDecodePackfileDecodesIntegerCount(t *testing.T) {
	p, err := DecodePackfile(bytes.NewReader([]byte{
		'P', 'A', 'C', 'K', // Pack header.
		0x0, 0x0, 0x0, 0x2, // Pack version.
		0x0, 0x0, 0x1, 0x2, // Number of packed objects.
	}), sha256.New())

	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	if p.Objects != 258 {
		t.Errorf("Expected %v, got %v", 258, p.Objects)
	}
}

func TestDecodePackfileReportsBadHeaders(t *testing.T) {
	p, err := DecodePackfile(bytes.NewReader([]byte{
		'W', 'R', 'O', 'N', 'G', // Malformed pack header.
		0x0, 0x0, 0x0, 0x0, // Pack version.
		0x0, 0x0, 0x0, 0x0, // Number of packed objects.
	}), sha1.New())

	if errBadPackHeader != err {
		t.Errorf("Expected %v, got %v", errBadPackHeader, err)
	}
	if p != nil {
		t.Errorf("Expected nil, got %v", p)
	}
}
