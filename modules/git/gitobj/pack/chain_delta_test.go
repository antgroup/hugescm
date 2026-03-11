package pack

import (
	"bytes"
	"testing"
)

func TestChainDeltaUnpackCopiesFromBase(t *testing.T) {
	c := &ChainDelta{
		base: &ChainSimple{
			X: []byte{0x0, 0x1, 0x2, 0x3},
		},
		delta: []byte{
			0x04, // Source size: 4.
			0x03, // Destination size: 3.

			0x80 | 0x01 | 0x10, // Copy, omask=0001, smask=0001.
			0x1,                // Offset: 1.
			0x3,                // Size: 3.
		},
	}

	data, err := c.Unpack()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	expected := []byte{0x1, 0x2, 0x3}
	if !bytes.Equal(expected, data) {
		t.Errorf("Expected %v, got %v", expected, data)
	}
}

func TestChainDeltaUnpackAddsToBase(t *testing.T) {
	c := &ChainDelta{
		base: &ChainSimple{
			X: make([]byte, 0),
		},
		delta: []byte{
			0x0, // Source size: 0.
			0x3, // Destination size: 3.

			0x3, // Add, size=3.

			0x1, 0x2, 0x3, // Contents: ...
		},
	}

	data, err := c.Unpack()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	expected := []byte{0x1, 0x2, 0x3}
	if !bytes.Equal(expected, data) {
		t.Errorf("Expected %v, got %v", expected, data)
	}
}

func TestChainDeltaWithMultipleInstructions(t *testing.T) {
	c := &ChainDelta{
		base: &ChainSimple{
			X: []byte{'H', 'e', 'l', 'l', 'o', '!', '\n'},
		},
		delta: []byte{
			0x07, // Source size: 7.
			0x0e, // Destination size: 14.

			0x80 | 0x01 | 0x10, // Copy, omask=0001, smask=0001.
			0x0,                // Offset: 1.
			0x5,                // Size: 5.

			0x7,                               // Add, size=7.
			',', ' ', 'w', 'o', 'r', 'l', 'd', // Contents: ...

			0x80 | 0x01 | 0x10, // Copy, omask=0001, smask=0001.
			0x05,               // Offset: 5.
			0x02,               // Size: 2.
		},
	}

	data, err := c.Unpack()
	if err != nil {
		t.Errorf("Expected nil, got %v", err)
	}
	expected := []byte("Hello, world!\n")
	if !bytes.Equal(expected, data) {
		t.Errorf("Expected %v, got %v", expected, data)
	}
}

func TestChainDeltaWithInvalidDeltaInstruction(t *testing.T) {
	c := &ChainDelta{
		base: &ChainSimple{
			X: make([]byte, 0),
		},
		delta: []byte{
			0x0, // Source size: 0.
			0x1, // Destination size: 3.

			0x0, // Invalid instruction.
		},
	}

	data, err := c.Unpack()
	if err == nil || (err.Error() != "git/object/pack:: invalid delta data" && err.Error() != "git/object/pack: invalid delta data") {
		t.Errorf("Expected 'git/object/pack:: invalid delta data' or 'git/object/pack: invalid delta data', got %v", err)
	}
	if data != nil {
		t.Errorf("Expected nil, got %v", data)
	}
}

func TestChainDeltaWithExtraInstructions(t *testing.T) {
	c := &ChainDelta{
		base: &ChainSimple{
			X: make([]byte, 0),
		},
		delta: []byte{
			0x0, // Source size: 0.
			0x3, // Destination size: 3.

			0x4, // Add, size=4 (invalid).

			0x1, 0x2, 0x3, 0x4, // Contents: ...
		},
	}

	data, err := c.Unpack()
	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}
	if errMsg != "git/object/pack:: invalid delta data" && errMsg != "git/object/pack: invalid delta data" {
		t.Errorf("Expected 'git/object/pack:: invalid delta data' or 'git/object/pack: invalid delta data', got %v", err)
	}
	if data != nil {
		t.Errorf("Expected nil, got %v", data)
	}
}
