package merkletrie

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
)

// fakeNoder is a minimal noder.Noder implementation for unit-testing
// doubleIter.sameHash. It can optionally implement noder.Comparators
// (mode + mtime) and noder.Sizer (size).
type fakeNoder struct {
	name       string
	hash       []byte
	mode       filemode.FileMode
	modifiedAt time.Time
	size       int64
	hasMeta    bool // emit Mode/ModifiedAt as Comparators
	hasSize    bool // emit Size as Sizer
	children   []noder.Noder
	isDir      bool
}

func (f *fakeNoder) String() string                                  { return f.name }
func (f *fakeNoder) Name() string                                    { return f.name }
func (f *fakeNoder) Hash() []byte                                    { return f.hash }
func (f *fakeNoder) IsDir() bool                                     { return f.isDir }
func (f *fakeNoder) Children(context.Context) ([]noder.Noder, error) { return f.children, nil }
func (f *fakeNoder) NumChildren(context.Context) (int, error)        { return len(f.children), nil }
func (f *fakeNoder) Skip() bool                                      { return false }

// Conditionally satisfy Comparators / Sizer via type-assertion-friendly
// wrappers below.

type compNoder struct{ *fakeNoder }

func (c compNoder) Mode() filemode.FileMode { return c.fakeNoder.mode }
func (c compNoder) ModifiedAt() time.Time   { return c.fakeNoder.modifiedAt }

type sizeNoder struct{ *fakeNoder }

func (s sizeNoder) Size() int64 { return s.fakeNoder.size }

type compSizeNoder struct{ *fakeNoder }

func (c compSizeNoder) Mode() filemode.FileMode { return c.fakeNoder.mode }
func (c compSizeNoder) ModifiedAt() time.Time   { return c.fakeNoder.modifiedAt }
func (c compSizeNoder) Size() int64             { return c.fakeNoder.size }

// makeNoder wraps a *fakeNoder so that the interface methods reflect
// the requested capabilities.
func makeNoder(n *fakeNoder) noder.Noder {
	switch {
	case n.hasMeta && n.hasSize:
		return compSizeNoder{n}
	case n.hasMeta:
		return compNoder{n}
	case n.hasSize:
		return sizeNoder{n}
	default:
		return n
	}
}

// pathOf wraps n into a noder.Path of length 1, which is what
// doubleIter consumes via Last().
func pathOf(n noder.Noder) noder.Path {
	return noder.Path{n}
}

func TestSameModTime(t *testing.T) {
	base := time.Unix(1_700_000_000, 123_456_000)
	cases := []struct {
		name string
		a, b time.Time
		want bool
	}{
		{"identical", base, base, true},
		{"sub-microsecond drift", base, base.Add(700 * time.Nanosecond), true},
		{"different microsecond", base, base.Add(2 * time.Microsecond), false},
		{"different second", base, base.Add(time.Second), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameModTime(tc.a, tc.b); got != tc.want {
				t.Fatalf("sameModTime(%v,%v)=%v, want %v", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

// hashEqualMustNotBeCalled returns a noder.Equal that fails the test if
// invoked. Used to assert the fast-path is hit.
func hashEqualMustNotBeCalled(t *testing.T) noder.Equal {
	t.Helper()
	return func(a, b noder.Hasher) bool {
		t.Fatalf("hashEqual should not be invoked when fast-path applies")
		return false
	}
}

func TestSameHashFastPathHitOnMatchingMetadata(t *testing.T) {
	mt := time.Unix(1_700_000_000, 0)
	from := makeNoder(&fakeNoder{
		name: "f", mode: filemode.Regular, modifiedAt: mt,
		size: 100, hasMeta: true, hasSize: true,
		hash: []byte("from-hash-should-not-be-read"),
	})
	to := makeNoder(&fakeNoder{
		name: "f", mode: filemode.Regular, modifiedAt: mt,
		size: 100, hasMeta: true, hasSize: true,
		hash: []byte("to-hash-DIFFERENT-but-fast-path-matches"),
	})

	d := &doubleIter{hashEqual: hashEqualMustNotBeCalled(t)}
	d.from.current = pathOf(from)
	d.to.current = pathOf(to)

	if !d.sameHash() {
		t.Fatalf("expected fast-path hit when mode/size/mtime match")
	}
}

func TestSameHashRejectsDifferentSize(t *testing.T) {
	mt := time.Unix(1_700_000_000, 0)
	from := makeNoder(&fakeNoder{
		name: "f", mode: filemode.Regular, modifiedAt: mt,
		size: 100, hasMeta: true, hasSize: true,
	})
	to := makeNoder(&fakeNoder{
		name: "f", mode: filemode.Regular, modifiedAt: mt,
		size: 200, hasMeta: true, hasSize: true,
	})

	d := &doubleIter{
		hashEqual: func(a, b noder.Hasher) bool {
			t.Fatalf("hashEqual should not be invoked when sizes differ")
			return false
		},
	}
	d.from.current = pathOf(from)
	d.to.current = pathOf(to)

	if d.sameHash() {
		t.Fatalf("expected sameHash=false when sizes differ")
	}
}

func TestSameHashRejectsDifferentMode(t *testing.T) {
	mt := time.Unix(1_700_000_000, 0)
	from := makeNoder(&fakeNoder{
		name: "f", mode: filemode.Regular, modifiedAt: mt,
		size: 100, hasMeta: true, hasSize: true,
	})
	to := makeNoder(&fakeNoder{
		name: "f", mode: filemode.Executable, modifiedAt: mt,
		size: 100, hasMeta: true, hasSize: true,
	})

	d := &doubleIter{
		hashEqual: func(a, b noder.Hasher) bool {
			t.Fatalf("hashEqual should not be invoked when modes differ")
			return false
		},
	}
	d.from.current = pathOf(from)
	d.to.current = pathOf(to)

	if d.sameHash() {
		t.Fatalf("expected sameHash=false when modes differ")
	}
}

func TestSameHashFallsBackToHashEqualOnMTimeDrift(t *testing.T) {
	from := makeNoder(&fakeNoder{
		name: "f", mode: filemode.Regular,
		modifiedAt: time.Unix(1_700_000_000, 0),
		size:       100, hasMeta: true, hasSize: true,
	})
	to := makeNoder(&fakeNoder{
		name: "f", mode: filemode.Regular,
		modifiedAt: time.Unix(1_700_000_005, 0), // 5s drift
		size:       100, hasMeta: true, hasSize: true,
	})

	called := false
	d := &doubleIter{
		hashEqual: func(a, b noder.Hasher) bool {
			called = true
			return true // pretend the actual hashes are equal
		},
	}
	d.from.current = pathOf(from)
	d.to.current = pathOf(to)

	if !d.sameHash() {
		t.Fatalf("hashEqual returned true but sameHash returned false")
	}
	if !called {
		t.Fatalf("expected hashEqual to be invoked when mtimes do not match within microsecond")
	}
}

func TestSameHashSkipsSizeCheckWhenInterfaceMissing(t *testing.T) {
	mt := time.Unix(1_700_000_000, 0)
	// from implements Comparators but not Sizer; to does the same.
	// Sizes are not part of the interface, so the Size guard is
	// inactive and we should still hit the fast path on matching mtime.
	from := makeNoder(&fakeNoder{
		name: "f", mode: filemode.Regular, modifiedAt: mt,
		hasMeta: true, hasSize: false,
	})
	to := makeNoder(&fakeNoder{
		name: "f", mode: filemode.Regular, modifiedAt: mt,
		hasMeta: true, hasSize: false,
	})

	d := &doubleIter{hashEqual: hashEqualMustNotBeCalled(t)}
	d.from.current = pathOf(from)
	d.to.current = pathOf(to)

	if !d.sameHash() {
		t.Fatalf("expected fast-path hit when Sizer is absent on both sides")
	}
}

// Sanity: ensure noder.Sizer is satisfied by our compSizeNoder so the
// production type assertion is exercised the same way real noders are.
func TestSizerInterfaceShape(t *testing.T) {
	var _ noder.Sizer = compSizeNoder{}
	var _ noder.Comparators = compSizeNoder{}
	var _ noder.Comparators = compNoder{}
	var _ noder.Sizer = sizeNoder{}
	// Compile-time assertion only; no runtime assertion needed.
	if errors.Is(nil, nil) != true {
		t.Fatal("unreachable")
	}
}
