package bitmap

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBitmapReadWrite(t *testing.T) {
	require := require.New(t)

	b := newBitmap()
	buf := bytes.NewBuffer(nil)
	_, err := b.Write(buf, binary.BigEndian)
	require.NoError(err)

	b2, err := FromBytes(buf.Bytes(), binary.BigEndian)
	require.NoError(err)

	require.Equal(b, b2)
}

func TestBitmapGet(t *testing.T) {
	require := require.New(t)

	b := newBitmap()

	require.False(b.Get(math.MaxInt64))

	// check zeroes of the first word
	for i := int64(0); i < 5*64; i++ {
		require.False(b.Get(i), "%d", i)
	}

	// check the second word
	one := int64(5*64 + (63 - 5))
	for i := int64(5 * 64); i < 6*64; i++ {
		if i == one {
			require.True(b.Get(i), "%d -> %s", i, strconv.FormatUint(b.w[1], 2))
		} else {
			require.False(b.Get(i), "%d", i-5*64)
		}
	}

	// check third word
	one = int64(6*64 + (63 - 6))
	for i := int64(6 * 64); i < 7*64; i++ {
		if i == one {
			require.True(b.Get(i), "%i -> %s", i, strconv.FormatUint(b.w[2], 2))
		} else {
			require.False(b.Get(i), "%d", i-6*64)
		}
	}

	// check fourth word
	for i := int64(7 * 64); i < 8*64; i++ {
		require.True(b.Get(i), "%d", i-(7*64))
	}

	// check fifth word
	offset := int64(8 * 64)
	for i := offset; i < 9*64; i++ {
		if i < offset+5 {
			require.False(b.Get(i), "%d", i-offset)
		} else {
			require.True(b.Get(i), "%d", i-offset)
		}
	}

	// check sixth word
	for i := int64(9 * 64); i < 10*64; i++ {
		require.True(b.Get(i), "%d", i-9*64)
	}
}

func TestBitmapSet(t *testing.T) {
	require := require.New(t)
	b := New()

	require.NoError(b.Set(5*64 + (63 - 5)))
	require.NoError(b.Set(6*64 + (63 - 6)))

	require.Equal(ErrInvalidBitSet, b.Set(0))

	for i := int64(7 * 64); i < 8*64; i++ {
		require.NoError(b.Set(i))
	}

	for i := int64(8*64) + 5; i < 9*64; i++ {
		require.NoError(b.Set(i))
	}

	for i := int64(9 * 64); i < 10*64; i++ {
		require.NoError(b.Set(i))
	}

	require.Equal(newBitmap(), b)
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

	require := require.New(t)

	b := New()
	b.w = make([]uint64, int(maxUint31)+2)
	b.w[0] = uint64(newRlw(false, 1, uint32(maxUint31)))
	b.n = (int64(maxUint31) + 1) * 64
	b.lastrlw = 0

	require.NoError(b.Set(b.n + 63))
	require.Equal(int(maxUint31)+4, len(b.w))
	require.Equal(len(b.w)-2, b.lastrlw)
	require.Equal(uint64(newRlw(false, 1, uint32(maxUint31))), b.w[0])
	require.Equal(uint64(newRlw(false, 0, 1)), b.w[len(b.w)-2])
	require.Equal(uint64(1), b.w[len(b.w)-1])
}

func TestBitmapSetOverflowK(t *testing.T) {
	require := require.New(t)

	b := New()
	b.w = []uint64{uint64(newRlw(false, uint32(math.MaxUint32), 0))}
	b.n = int64(math.MaxUint32) * 64
	b.lastrlw = 0

	require.NoError(b.Set(b.n + 127))

	require.Equal(3, len(b.w))
	require.Equal(1, b.lastrlw)
	require.Equal(uint64(newRlw(false, uint32(math.MaxUint32), 0)), b.w[0])
	require.Equal(uint64(newRlw(false, 1, 1)), b.w[1])
	require.Equal(uint64(1), b.w[2])
}

func TestBitmapSetOverflowKAllOnes(t *testing.T) {
	require := require.New(t)

	b := New()
	b.w = []uint64{
		uint64(newRlw(true, uint32(math.MaxUint32), 1)),
		^uint64(0) >> 1 << 1,
	}
	b.n = int64(math.MaxUint32+1)*64 - 1
	b.lastrlw = 0

	require.NoError(b.Set(b.n))

	require.Equal(2, len(b.w))
	require.Equal(1, b.lastrlw)
	require.Equal(uint64(newRlw(true, uint32(math.MaxUint32), 0)), b.w[0])
	require.Equal(uint64(newRlw(true, 1, 0)), b.w[1])
}

func TestBitmapSetAllOnesPrevRlw(t *testing.T) {
	require := require.New(t)

	b := New()
	b.w = []uint64{
		uint64(newRlw(true, 1, 1)),
		^uint64(0) >> 1 << 1,
	}
	b.n = 2*64 - 1
	b.lastrlw = 0

	require.NoError(b.Set(b.n))

	require.Equal(1, len(b.w))
	require.Equal(0, b.lastrlw)
	require.Equal(uint64(newRlw(true, 2, 0)), b.w[0])
}

func TestRlwSetl(t *testing.T) {
	require := require.New(t)

	rlw := ^rlw(0)
	require.Equal(maxUint31, rlw.l())

	rlw.setl(5)
	require.Equal(uint32(5), rlw.l())
}

func TestRlwSetk(t *testing.T) {
	require := require.New(t)

	rlw := ^rlw(0)
	require.Equal(uint32(math.MaxUint32), rlw.k())

	rlw.setk(10)
	require.Equal(uint32(10), rlw.k())
}

func TestSetBit(t *testing.T) {
	var n uint64
	setbit(&n, 5)
	expected := strings.Repeat("0", 5) + "1" + strings.Repeat("0", 64-6)
	require.Equal(t,
		expected,
		fmt.Sprintf("%064s", strconv.FormatUint(n, 2)),
	)
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
	for i := 0; i < b.N; i++ {
		_ = bitmap.Get(int64(i) % bitmap.n)
	}
}

func BenchmarkBitmapGetSequential(b *testing.B) {
	bitmap, err := newBigBitmap()
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		for i := int64(0); i < bitmap.n; i++ {
			_ = bitmap.Get(i)
		}
	}
}

func BenchmarkBitmapGetNotSequential(b *testing.B) {
	bitmap, err := newBigBitmap()
	require.NoError(b, err)
	for i := 0; i < b.N; i++ {
		for i := bitmap.n; i >= 0; i-- {
			_ = bitmap.Get(i)
		}
	}
}

func BenchmarkBitmapWrite(b *testing.B) {
	bitmap := newBitmap()
	buf := bytes.NewBuffer(nil)

	for i := 0; i < b.N; i++ {
		buf.Reset()
		_, _ = bitmap.Write(buf, binary.BigEndian)
	}
}

func BenchmarkBitmapRead(b *testing.B) {
	bitmap := newBitmap()
	buf := bytes.NewBuffer(nil)
	_, err := bitmap.Write(buf, binary.BigEndian)
	require.NoError(b, err)

	bytes := buf.Bytes()

	for i := 0; i < b.N; i++ {
		_, err = FromBytes(bytes, binary.BigEndian)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}
}

func BenchmarkBitmapSet(b *testing.B) {
	bitmap := New()
	for i := 0; i < b.N; i++ {
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

	for i := int64(0); i < 100000; i++ {
		if i%2 == 0 {
			if err := b.Set(i); err != nil {
				return nil, err
			}
		}
	}

	return b, nil
}
