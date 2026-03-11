package pack

import (
	"testing"
)

func TestBoundsLeft(t *testing.T) {
	if newBounds(1, 2).Left() != 1 {
		t.Errorf("Expected %v, got %v", 1, newBounds(1, 2).Left())
	}
}

func TestBoundsRight(t *testing.T) {
	if newBounds(1, 2).Right() != 2 {
		t.Errorf("Expected %v, got %v", 2, newBounds(1, 2).Right())
	}
}

func TestBoundsWithLeftReturnsNewBounds(t *testing.T) {
	b1 := newBounds(1, 2)
	b2 := b1.WithLeft(3)

	if b1.Left() != 1 {
		t.Errorf("Expected %v, got %v", 1, b1.Left())
	}
	if b1.Right() != 2 {
		t.Errorf("Expected %v, got %v", 2, b1.Right())
	}

	if b2.Left() != 3 {
		t.Errorf("Expected %v, got %v", 3, b2.Left())
	}
	if b2.Right() != 2 {
		t.Errorf("Expected %v, got %v", 2, b2.Right())
	}
}

func TestBoundsWithRightReturnsNewBounds(t *testing.T) {
	b1 := newBounds(1, 2)
	b2 := b1.WithRight(3)

	if b1.Left() != 1 {
		t.Errorf("Expected %v, got %v", 1, b1.Left())
	}
	if b1.Right() != 2 {
		t.Errorf("Expected %v, got %v", 2, b1.Right())
	}

	if b2.Left() != 1 {
		t.Errorf("Expected %v, got %v", 1, b2.Left())
	}
	if b2.Right() != 3 {
		t.Errorf("Expected %v, got %v", 3, b2.Right())
	}
}

func TestBoundsEqualWithIdenticalBounds(t *testing.T) {
	b1 := newBounds(1, 2)
	b2 := newBounds(1, 2)

	if !b1.Equal(b2) {
		t.Errorf("Expected true")
	}
}

func TestBoundsEqualWithDifferentBounds(t *testing.T) {
	b1 := newBounds(1, 2)
	b2 := newBounds(3, 4)

	if b1.Equal(b2) {
		t.Errorf("Expected false")
	}
}

func TestBoundsEqualWithNilReceiver(t *testing.T) {
	bnil := (*bounds)(nil)
	b2 := newBounds(1, 2)

	if bnil.Equal(b2) {
		t.Errorf("Expected false")
	}
}

func TestBoundsEqualWithNilArgument(t *testing.T) {
	b1 := newBounds(1, 2)
	bnil := (*bounds)(nil)

	if b1.Equal(bnil) {
		t.Errorf("Expected false")
	}
}

func TestBoundsEqualWithNilArgumentAndReceiver(t *testing.T) {
	b1 := (*bounds)(nil)
	b2 := (*bounds)(nil)

	if !b1.Equal(b2) {
		t.Errorf("Expected true")
	}
}

func TestBoundsString(t *testing.T) {
	b1 := newBounds(1, 2)

	if b1.String() != "[1,2]" {
		t.Errorf("Expected [1,2], got %v", b1.String())
	}
}
