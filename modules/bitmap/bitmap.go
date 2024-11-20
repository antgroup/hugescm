package bitmap

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

// Bitmap is an EWAH-encoded bitmap.
// See: https://github.com/lemire/javaewah
type Bitmap struct {
	// n is the number of bits in the bitmap
	n int64
	// w is the list of words in the bitmap
	w []uint64

	// stuff for writing efficiently
	lastrlw int

	// stuff for reading efficiently
	cursor  int
	lastpos int64
	acc     int64
}

// New creates a new empty bitmap.
func New() *Bitmap {
	return &Bitmap{lastrlw: -1}
}

// FromReader creates a Bitmap from the given reader.
func FromReader(r io.Reader, order binary.ByteOrder) (*Bitmap, error) {
	bits, err := readUint32(r, order)
	if err != nil {
		return nil, fmt.Errorf("bitmap: can't read uncompressed bit number: %s", err)
	}

	words, err := readUint32(r, order)
	if err != nil {
		return nil, fmt.Errorf("bitmap: can't read compressed word number: %s", err)
	}

	w := make([]uint64, int(words))
	for i := 0; i < int(words); i++ {
		w[i], err = readUint64(r, order)
		if err != nil {
			return nil, fmt.Errorf("bitmap: can't read %dth word: %s", i+1, err)
		}
	}

	lastrlw, err := readUint32(r, order)
	if err != nil {
		return nil, fmt.Errorf("bitmap: can't read position of current RLW: %s", err)
	}

	return &Bitmap{
		n:       int64(bits),
		w:       w,
		lastrlw: int(lastrlw),
	}, nil
}

// FromBytes creates a Bitmap from the given bytes.
func FromBytes(b []byte, order binary.ByteOrder) (*Bitmap, error) {
	return FromReader(bytes.NewBuffer(b), order)
}

// Write will write the Bitmap to a writer with the following format:
// https://github.com/git/git/blob/master/Documentation/technical/bitmap-format.txt#L92
func (b *Bitmap) Write(w io.Writer, order binary.ByteOrder) (n int64, err error) {
	if err := writeUint32(w, order, b.Bits()); err != nil {
		return 0, err
	}

	if err := writeUint32(w, order, uint32(len(b.w))); err != nil {
		return 0, err
	}

	for _, word := range b.w {
		if err := writeUint64(w, order, word); err != nil {
			return 0, err
		}
	}

	if err := writeUint32(w, order, uint32(b.lastrlw)); err != nil {
		return 0, err
	}

	return 4*3 + int64(len(b.w))*8, nil
}

func writeUint32(w io.Writer, bo binary.ByteOrder, num uint32) error {
	var b = make([]byte, 4)
	bo.PutUint32(b, num)
	n, err := w.Write(b)
	if err != nil {
		return err
	}

	if n != 4 {
		return fmt.Errorf("unable to write 4 bytes for uint32, wrote %d instead", n)
	}

	return nil
}

func writeUint64(w io.Writer, bo binary.ByteOrder, num uint64) error {
	var b = make([]byte, 8)
	bo.PutUint64(b, num)
	n, err := w.Write(b)
	if err != nil {
		return err
	}

	if n != 8 {
		return fmt.Errorf("unable to write 8 bytes for uint64, wrote %d instead", n)
	}

	return nil
}

func readUint32(r io.Reader, bo binary.ByteOrder) (uint32, error) {
	var buf = make([]byte, 4)
	_, err := io.ReadAtLeast(r, buf, 4)
	if err != nil {
		return 0, err
	}

	return bo.Uint32(buf), nil
}

func readUint64(r io.Reader, bo binary.ByteOrder) (uint64, error) {
	var buf = make([]byte, 8)
	_, err := io.ReadAtLeast(r, buf, 8)
	if err != nil {
		return 0, err
	}

	return bo.Uint64(buf), nil
}

// ErrInvalidBitSet is returned when there is an attempt to set a bit
// before the last written bit.
var ErrInvalidBitSet = errors.New("bitmap: attempted to set a bit before the last written bit")

const allones = ^uint64(0)
const maxUint31 = ^uint32(0) >> 1

// Set sets to 1 the bit at the given position. Take into account that bits
// need to be set in ascending order. Setting the 4th bit will return an error
// if you already set the 5th bit, for example.
func (b *Bitmap) Set(pos int64) error {
	if b.n > pos {
		return ErrInvalidBitSet
	}

	if b.lastrlw < 0 {
		b.lastrlw = 0
		b.w = append(b.w, uint64(newRlw(false, 0, 0)))
	}

	last := len(b.w) - 1
	lastrlw := rlw(b.w[b.lastrlw])
	idx := uint64(pos % 64)

	bn := b.size()

	// it's inside the last word
	if bn > pos {
		setbit(&b.w[last], idx)

		// all bits in this literal are 1s, so transform it into a rlw
		if b.w[last] == allones {
			// previous rlw has 1 literal (the one being transformed), so
			// remove the literal and increase k by 1 only if k does not overflow
			if lastrlw.b() && lastrlw.l() == 1 && lastrlw.k() < math.MaxUint32 {
				lastrlw.setk(lastrlw.k() + 1)
				lastrlw.setl(0)
				b.w[b.lastrlw] = uint64(lastrlw)
				b.w = b.w[:last]
			} else {
				lastrlw.setl(lastrlw.l() - 1)
				b.w[last] = uint64(newRlw(true, 1, 0))
				b.w[b.lastrlw] = uint64(lastrlw)
				b.lastrlw = last
			}
		}
	} else {
		k := (pos - bn) / 64
		var literal uint64
		setbit(&literal, idx)

		// increment l only if l does not overflow
		if k == 0 && lastrlw.l()+1 <= maxUint31 {
			lastrlw.setl(lastrlw.l() + 1)
			b.w[b.lastrlw] = uint64(lastrlw)
		} else if k > 0 && int64(lastrlw.k())+k <= math.MaxUint32 && lastrlw.l() == 0 {
			// increment k only if k does not overflow
			lastrlw.setk(lastrlw.k() + uint32(k))
			lastrlw.setl(lastrlw.l() + 1)
			b.w[b.lastrlw] = uint64(lastrlw)
		} else {
			b.w = append(b.w, uint64(newRlw(false, uint32(k-math.MaxUint32-1), 1)))
			b.lastrlw = len(b.w) - 1
		}

		b.w = append(b.w, literal)
	}

	b.n = pos + 1

	return nil
}

// Get returns the bit at the given position, being true 1 and false 0.
func (b *Bitmap) Get(pos int64) bool {
	// quick path, if pos has never been written, it cannot be 1
	if pos >= b.n {
		return false
	}

	if b.lastpos > pos {
		b.lastpos = -1
		b.cursor = 0
		b.acc = 0
	} else if b.cursor >= len(b.w) {
		b.cursor = b.lastrlw
	}

	for ; b.cursor < len(b.w); b.cursor++ {
		acc := b.acc
		word := rlw(b.w[b.cursor])
		kb := int64(word.k()) * 64
		if pos < b.acc+kb {
			b.lastpos = pos
			return word.b()
		}

		acc += kb
		l := int64(word.l())

		if l > 0 && pos < acc+l*64 {
			for j := 1; j <= int(word.l()); j++ {
				if pos < acc+64 {
					w := b.w[b.cursor+j]
					mask := uint64(1) << (63 - uint64(pos-acc))
					return w&mask != 0
				}

				acc += 64
			}
		} else {
			acc += l * 64
		}

		b.cursor += int(l)
		b.acc = acc
	}

	return false
}

// Bits returns the number of uncompressed bits in the bitmap.
func (b *Bitmap) Bits() uint32 {
	return uint32(b.n)
}

// size returns the number of bits allocated, even if
// they are not used yet. Result of size() will always be equal
// or greater than n.
func (b *Bitmap) size() int64 {
	bn := (b.n / 64) * 64
	if b.n%64 != 0 {
		bn += 64
	}
	return bn
}

// Bytes returns the number of bytes taken by the compressed bitmap.
func (b *Bitmap) Bytes() int64 {
	return int64(len(b.w)*64) / 8
}

// Reset clears the bitmap and sets everything to unused empty zeroes.
func (b *Bitmap) Reset() {
	b.n = 0
	b.w = nil
	b.lastrlw = -1
}

// setbit sets to 1 the bit in the given idx.
func setbit(word *uint64, idx uint64) {
	*word |= (uint64(1) << (64 - idx - 1))
}

// rlw is a Running Length Word, which has 3 parts:
// - (b) 1 bit that is repeated
// - (k) 32 bits with the number of repetitions for the previous bit
// - (l) 31 bits saying how many literal words follow this rlw
type rlw uint64

// 100000000000000000000000000000000000000000000000000000000000000
const bmask = uint64(1) << 63

// 011111111111111111111111111111110000000000000000000000000000000
const kmask = ^uint64(0) >> 32 << 31

// 000000000000000000000000000000000111111111111111111111111111111
const lmask = ^uint64(0) >> 33

// newRlw creates a new rlw with the given bit, k and l.
func newRlw(b bool, k, l uint32) rlw {
	var bit uint64
	if b {
		bit = 1
	}
	return rlw(bit<<63 | uint64(k)<<31 | uint64(l))
}

// b returns the bit of this rlw, true for 1, false for 0.
func (r rlw) b() bool {
	return (uint64(r)&bmask)>>63 != 0
}

// k returns the number of word repetitions of b.
func (r rlw) k() uint32 {
	return uint32(uint64(r) & kmask >> 31)
}

// l returns the number of literal words that follow this rlw.
func (r rlw) l() uint32 {
	return uint32(uint64(r) & lmask)
}

// setk changes the k of this rlw.
func (r *rlw) setk(k uint32) {
	*r = rlw((uint64(*r) & ^kmask) | uint64(k)<<31)
}

// setl changes the l of this rlw.
func (r *rlw) setl(l uint32) {
	*r = rlw((uint64(*r) & ^lmask) | uint64(l))
}
