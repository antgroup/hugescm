//go:build !386

package bitmap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func TestBitmapReadWrite(t *testing.T) {
	b := newBitmap()
	buf := bytes.NewBuffer(nil)
	_, err := b.Write(buf, binary.BigEndian)
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	b2, err := FromBytes(buf.Bytes(), binary.BigEndian)
	if err != nil {
		t.Fatalf("FromBytes error: %v", err)
	}

	if !reflect.DeepEqual(b, b2) {
		t.Errorf("Expected %v, got %v", b, b2)
	}
}

func TestBitmapGet(t *testing.T) {
	b := newBitmap()

	if b.Get(math.MaxInt64) {
		t.Errorf("Expected false for bit %d", math.MaxInt64)
	}

	// check zeroes of the first word
	for i := range int64(5 * 64) {
		if b.Get(i) {
			t.Errorf("Expected false for bit %d", i)
		}
	}

	// check the second word
	one := int64(5*64 + (63 - 5))
	for i := int64(5 * 64); i < 6*64; i++ {
		if i == one {
			if !b.Get(i) {
				t.Errorf("Expected true for bit %d -> %s", i, strconv.FormatUint(b.w[1], 2))
			}
		} else {
			if b.Get(i) {
				t.Errorf("Expected false for bit %d", i-5*64)
			}
		}
	}

	// check third word
	one = int64(6*64 + (63 - 6))
	for i := int64(6 * 64); i < 7*64; i++ {
		if i == one {
			if !b.Get(i) {
				t.Errorf("Expected true for bit %d -> %s", i, strconv.FormatUint(b.w[2], 2))
			}
		} else {
			if b.Get(i) {
				t.Errorf("Expected false for bit %d", i-6*64)
			}
		}
	}

	// check fourth word
	for i := int64(7 * 64); i < 8*64; i++ {
		if !b.Get(i) {
			t.Errorf("Expected true for bit %d", i-(7*64))
		}
	}

	// check fifth word
	offset := int64(8 * 64)
	for i := offset; i < 9*64; i++ {
		if i < offset+5 {
			if b.Get(i) {
				t.Errorf("Expected false for bit %d", i-offset)
			}
		} else {
			if !b.Get(i) {
				t.Errorf("Expected true for bit %d", i-offset)
			}
		}
	}

	// check sixth word
	for i := int64(9 * 64); i < 10*64; i++ {
		if !b.Get(i) {
			t.Errorf("Expected true for bit %d", i-9*64)
		}
	}
}

func TestBitmapSet(t *testing.T) {
	b := New()

	if err := b.Set(5*64 + (63 - 5)); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if err := b.Set(6*64 + (63 - 6)); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	if err := b.Set(0); err != ErrInvalidBitSet {
		t.Errorf("Expected ErrInvalidBitSet, got %v", err)
	}

	for i := int64(7 * 64); i < 8*64; i++ {
		if err := b.Set(i); err != nil {
			t.Fatalf("Set error: %v", err)
		}
	}

	for i := int64(8*64) + 5; i < 9*64; i++ {
		if err := b.Set(i); err != nil {
			t.Fatalf("Set error: %v", err)
		}
	}

	for i := int64(9 * 64); i < 10*64; i++ {
		if err := b.Set(i); err != nil {
			t.Fatalf("Set error: %v", err)
		}
	}

	expected := newBitmap()
	if !reflect.DeepEqual(b, expected) {
		t.Errorf("Expected %v, got %v", expected, b)
	}
}

func TestBitmapSetOverflowL(t *testing.T) {
	if testing.Short() {
		t.Skip("not running this on short mode")
		return
	}

	if os.Getenv("TRAVIS") == "true" {
		t.Skip("uses too much memory to run on travis")
		return
	}

	b := New()
	b.w = make([]uint64, int(maxUint31)+2)
	b.w[0] = uint64(newRlw(false, 1, uint32(maxUint31)))
	b.n = (int64(maxUint31) + 1) * 64
	b.lastrlw = 0

	if err := b.Set(b.n + 63); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	if len(b.w) != int(maxUint31)+4 {
		t.Errorf("Expected %d, got %d", int(maxUint31)+4, len(b.w))
	}
	if b.lastrlw != len(b.w)-2 {
		t.Errorf("Expected %d, got %d", len(b.w)-2, b.lastrlw)
	}
	if b.w[0] != uint64(newRlw(false, 1, uint32(maxUint31))) {
		t.Errorf("Expected %v, got %v", uint64(newRlw(false, 1, uint32(maxUint31))), b.w[0])
	}
	if b.w[len(b.w)-2] != uint64(newRlw(false, 0, 1)) {
		t.Errorf("Expected %v, got %v", uint64(newRlw(false, 0, 1)), b.w[len(b.w)-2])
	}
	if b.w[len(b.w)-1] != uint64(1) {
		t.Errorf("Expected %v, got %v", uint64(1), b.w[len(b.w)-1])
	}
}

func TestBitmapSetOverflowK(t *testing.T) {
	b := New()
	b.w = []uint64{uint64(newRlw(false, uint32(math.MaxUint32), 0))}
	b.n = int64(math.MaxUint32) * 64
	b.lastrlw = 0

	if err := b.Set(b.n + 127); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	if len(b.w) != 3 {
		t.Errorf("Expected 3, got %d", len(b.w))
	}
	if b.lastrlw != 1 {
		t.Errorf("Expected 1, got %d", b.lastrlw)
	}
	if b.w[0] != uint64(newRlw(false, uint32(math.MaxUint32), 0)) {
		t.Errorf("Expected %v, got %v", uint64(newRlw(false, uint32(math.MaxUint32), 0)), b.w[0])
	}
	if b.w[1] != uint64(newRlw(false, 1, 1)) {
		t.Errorf("Expected %v, got %v", uint64(newRlw(false, 1, 1)), b.w[1])
	}
	if b.w[2] != uint64(1) {
		t.Errorf("Expected %v, got %v", uint64(1), b.w[2])
	}
}

func TestBitmapSetOverflowKAllOnes(t *testing.T) {
	b := New()
	b.w = []uint64{
		uint64(newRlw(true, uint32(math.MaxUint32), 1)),
		^uint64(0) >> 1 << 1,
	}
	b.n = int64(math.MaxUint32+1)*64 - 1
	b.lastrlw = 0

	if err := b.Set(b.n); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	if len(b.w) != 2 {
		t.Errorf("Expected 2, got %d", len(b.w))
	}
	if b.lastrlw != 1 {
		t.Errorf("Expected 1, got %d", b.lastrlw)
	}
	if b.w[0] != uint64(newRlw(true, uint32(math.MaxUint32), 0)) {
		t.Errorf("Expected %v, got %v", uint64(newRlw(true, uint32(math.MaxUint32), 0)), b.w[0])
	}
	if b.w[1] != uint64(newRlw(true, 1, 0)) {
		t.Errorf("Expected %v, got %v", uint64(newRlw(true, 1, 0)), b.w[1])
	}
}

func TestBitmapSetAllOnesPrevRlw(t *testing.T) {
	b := New()
	b.w = []uint64{
		uint64(newRlw(true, 1, 1)),
		^uint64(0) >> 1 << 1,
	}
	b.n = 2*64 - 1
	b.lastrlw = 0

	if err := b.Set(b.n); err != nil {
		t.Fatalf("Set error: %v", err)
	}

	if len(b.w) != 1 {
		t.Errorf("Expected 1, got %d", len(b.w))
	}
	if b.lastrlw != 0 {
		t.Errorf("Expected 0, got %d", b.lastrlw)
	}
	if b.w[0] != uint64(newRlw(true, 2, 0)) {
		t.Errorf("Expected %v, got %v", uint64(newRlw(true, 2, 0)), b.w[0])
	}
}

func TestRlwSetl(t *testing.T) {
	rlw := ^rlw(0)
	if rlw.l() != maxUint31 {
		t.Errorf("Expected %d, got %d", maxUint31, rlw.l())
	}

	rlw.setl(5)
	if rlw.l() != uint32(5) {
		t.Errorf("Expected %d, got %d", uint32(5), rlw.l())
	}
}

func TestRlwSetk(t *testing.T) {
	rlw := ^rlw(0)
	if rlw.k() != uint32(math.MaxUint32) {
		t.Errorf("Expected %d, got %d", uint32(math.MaxUint32), rlw.k())
	}

	rlw.setk(10)
	if rlw.k() != uint32(10) {
		t.Errorf("Expected %d, got %d", uint32(10), rlw.k())
	}
}

func TestSetBit(t *testing.T) {
	var n uint64
	setbit(&n, 5)
	expected := strings.Repeat("0", 5) + "1" + strings.Repeat("0", 64-6)
	result := fmt.Sprintf("%064s", strconv.FormatUint(n, 2))
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

// see: https://github.com/erizocosmico/go-ewah/issues/1
func TestBug1(t *testing.T) {
	b := New()
	arr := []int64{1, 5, 8, 11, 15, 19, 23, 30, 128}
	for _, e := range arr {
		_ = b.Set(e)
	}

	for _, e := range arr {
		if !b.Get(e) {
			t.Errorf("expecting %d to be in bitmap", e)
		}
	}
}

func BenchmarkBitmapGet(b *testing.B) {
	bitmap := newBitmap()
	for i := 0; b.Loop(); i++ {
		_ = bitmap.Get(int64(i) % bitmap.n)
	}
}

func BenchmarkBitmapGetSequential(b *testing.B) {
	bitmap, err := newBigBitmap()
	if err != nil {
		b.Fatalf("newBigBitmap error: %v", err)
	}
	for b.Loop() {
		for i := int64(0); i < bitmap.n; i++ {
			_ = bitmap.Get(i)
		}
	}
}

func BenchmarkBitmapGetNotSequential(b *testing.B) {
	bitmap, err := newBigBitmap()
	if err != nil {
		b.Fatalf("newBigBitmap error: %v", err)
	}
	for b.Loop() {
		for i := bitmap.n; i >= 0; i-- {
			_ = bitmap.Get(i)
		}
	}
}

func BenchmarkBitmapWrite(b *testing.B) {
	bitmap := newBitmap()
	buf := bytes.NewBuffer(nil)

	for b.Loop() {
		buf.Reset()
		_, _ = bitmap.Write(buf, binary.BigEndian)
	}
}

func BenchmarkBitmapRead(b *testing.B) {
	bitmap := newBitmap()
	buf := bytes.NewBuffer(nil)
	_, err := bitmap.Write(buf, binary.BigEndian)
	if err != nil {
		b.Fatalf("Write error: %v", err)
	}

	bytes := buf.Bytes()

	for b.Loop() {
		_, err = FromBytes(bytes, binary.BigEndian)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}

func BenchmarkBitmapSet(b *testing.B) {
	bitmap := New()
	for i := 0; b.Loop(); i++ {
		_ = bitmap.Set(int64(i))
	}
}

func newBitmap() *Bitmap {
	b := New()
	b.w = []uint64{
		uint64(newRlw(false, 5, 2)),
		uint64(1) << 5,
		uint64(1) << 6,
		uint64(newRlw(true, 1, 1)),
		^uint64(0) >> 5,
		uint64(newRlw(true, 1, 0)),
	}
	b.n = 10 * 64
	b.lastrlw = 5
	return b
}

func newBigBitmap() (*Bitmap, error) {
	b := New()

	for i := range int64(100000) {
		if i%2 == 0 {
			if err := b.Set(i); err != nil {
				return nil, err
			}
		}
	}

	return b, nil
}
