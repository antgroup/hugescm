// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/antgroup/hugescm/modules/merkletrie/filesystem"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/plumbing/format/index"
)

func mustHash(b byte) plumbing.Hash {
	var h plumbing.Hash
	for i := range h {
		h[i] = b
	}
	return h
}

func expectedCacheHash(h plumbing.Hash, mode filemode.FileMode) []byte {
	buf := make([]byte, 0, filesystem.HashSize)
	buf = append(buf, h[:]...)
	buf = append(buf, mode.Bytes()...)
	return buf
}

// fakeResolver records every entry it is asked to resolve and returns
// the provided origin (size, hash, mode).
type fakeResolver struct {
	calls   []*index.Entry
	resolve func(e *index.Entry) *index.Entry
}

func (f *fakeResolver) Fn() fragmentsResolver {
	return func(_ context.Context, e *index.Entry) *index.Entry {
		f.calls = append(f.calls, e)
		return f.resolve(e)
	}
}

func TestBuildCacheFromIndexNilIndex(t *testing.T) {
	if c := buildCacheFromIndex(context.Background(), nil, nil); c != nil {
		t.Fatalf("expected nil cache for nil index, got %v", c)
	}
}

func TestBuildCacheFromIndexRegularOnly(t *testing.T) {
	mtime := time.Unix(1_700_000_000, 0)
	idx := &index.Index{Entries: []*index.Entry{
		{
			Name:       "a.bin",
			Hash:       mustHash(0x01),
			Size:       12,
			Mode:       filemode.Regular,
			Stage:      index.Merged,
			ModifiedAt: mtime,
		},
		{
			Name:       "b.sh",
			Hash:       mustHash(0x02),
			Size:       7,
			Mode:       filemode.Executable,
			Stage:      index.Merged,
			ModifiedAt: mtime,
		},
	}}

	c := buildCacheFromIndex(context.Background(), idx, nil)
	if c == nil {
		t.Fatalf("expected non-nil cache")
	}
	if c.Len() != 2 {
		t.Fatalf("expected 2 entries, got %d", c.Len())
	}

	e, ok := c.Lookup("a.bin")
	if !ok {
		t.Fatalf("a.bin not found")
	}
	if e.Size != 12 || !e.ModifiedAt.Equal(mtime) {
		t.Fatalf("a.bin meta mismatch: %+v", e)
	}
	if want := expectedCacheHash(mustHash(0x01), filemode.Regular); !bytes.Equal(e.Hash, want) {
		t.Fatalf("a.bin hash mismatch: got %x want %x", e.Hash, want)
	}

	e, ok = c.Lookup("b.sh")
	if !ok {
		t.Fatalf("b.sh not found")
	}
	if want := expectedCacheHash(mustHash(0x02), filemode.Executable); !bytes.Equal(e.Hash, want) {
		t.Fatalf("b.sh hash mismatch: got %x want %x", e.Hash, want)
	}
}

func TestBuildCacheFromIndexSkipsNonRegular(t *testing.T) {
	mtime := time.Unix(1_700_000_100, 0)
	idx := &index.Index{Entries: []*index.Entry{
		{Name: "sym", Mode: filemode.Symlink, Stage: index.Merged, ModifiedAt: mtime},
		{Name: "sub", Mode: filemode.Submodule, Stage: index.Merged, ModifiedAt: mtime},
		{Name: "dir", Mode: filemode.Dir, Stage: index.Merged, ModifiedAt: mtime},
		{Name: "skipped", Mode: filemode.Regular, Stage: index.Merged, SkipWorktree: true, ModifiedAt: mtime},
		{Name: "unmerged", Mode: filemode.Regular, Stage: index.OurMode, ModifiedAt: mtime},
		// `git add -N` placeholder: zero hash. Caching it would let
		// a worktree match silently report Unmodified.
		{Name: "ita", Mode: filemode.Regular, Stage: index.Merged, IntentToAdd: true, ModifiedAt: mtime},
		// One real regular file so the cache itself is non-empty.
		{Name: "keep", Mode: filemode.Regular, Stage: index.Merged, ModifiedAt: mtime, Hash: mustHash(0x33), Size: 1},
	}}

	c := buildCacheFromIndex(context.Background(), idx, nil)
	if c == nil {
		t.Fatalf("expected non-nil cache")
	}
	if c.Len() != 1 {
		t.Fatalf("expected only the regular file to be cached, got %d entries", c.Len())
	}
	for _, name := range []string{"sym", "sub", "dir", "skipped", "unmerged", "ita"} {
		if _, ok := c.Lookup(name); ok {
			t.Fatalf("%q should not be cached", name)
		}
	}
	if _, ok := c.Lookup("keep"); !ok {
		t.Fatalf("keep should be cached")
	}
}

func TestBuildCacheFromIndexFragmentsResolved(t *testing.T) {
	mtime := time.Unix(1_700_000_200, 0)
	fragHash := mustHash(0x10) // stored fragments-meta hash
	origin := mustHash(0xAA)   // resolved underlying blob hash
	resolver := &fakeResolver{
		resolve: func(e *index.Entry) *index.Entry {
			return &index.Entry{
				Name:       e.Name,
				Hash:       origin,
				Size:       4 * 1024 * 1024 * 1024, // 4 GB
				Mode:       filemode.Regular,
				Stage:      e.Stage,
				ModifiedAt: e.ModifiedAt,
			}
		},
	}

	idx := &index.Index{Entries: []*index.Entry{
		{
			Name:       "big.bin",
			Hash:       fragHash,
			Size:       42, // intentionally tiny in the fragments meta
			Mode:       filemode.Regular | filemode.Fragments,
			Stage:      index.Merged,
			ModifiedAt: mtime,
		},
	}}

	c := buildCacheFromIndex(context.Background(), idx, resolver.Fn())
	if c == nil || c.Len() != 1 {
		t.Fatalf("expected 1 cached entry, got %v", c)
	}
	if len(resolver.calls) != 1 || resolver.calls[0].Name != "big.bin" {
		t.Fatalf("resolver should have been invoked once for big.bin, got %+v", resolver.calls)
	}

	e, ok := c.Lookup("big.bin")
	if !ok {
		t.Fatalf("big.bin not cached")
	}
	if e.Size != 4*1024*1024*1024 {
		t.Fatalf("size should come from resolver, got %d", e.Size)
	}
	if !e.ModifiedAt.Equal(mtime) {
		t.Fatalf("mtime mismatch")
	}
	// Hash bytes must be (origin || Regular.Bytes()). In particular
	// the fragments mode bit must be stripped by Mode.Origin() so the
	// 36-byte layout matches what mindex.Node.Hash() produces for the
	// staging side.
	want := expectedCacheHash(origin, filemode.Regular)
	if !bytes.Equal(e.Hash, want) {
		t.Fatalf("hash mismatch:\n  got  %x\n  want %x", e.Hash, want)
	}
}

func TestBuildCacheFromIndexFragmentsResolverReturnsNil(t *testing.T) {
	mtime := time.Unix(1_700_000_300, 0)
	resolver := &fakeResolver{
		resolve: func(*index.Entry) *index.Entry { return nil },
	}

	idx := &index.Index{Entries: []*index.Entry{
		{
			Name:       "broken.bin",
			Hash:       mustHash(0x77),
			Size:       1,
			Mode:       filemode.Regular | filemode.Fragments,
			Stage:      index.Merged,
			ModifiedAt: mtime,
		},
		{
			Name:       "ok.bin",
			Hash:       mustHash(0x88),
			Size:       1,
			Mode:       filemode.Regular,
			Stage:      index.Merged,
			ModifiedAt: mtime,
		},
	}}

	c := buildCacheFromIndex(context.Background(), idx, resolver.Fn())
	if c == nil {
		t.Fatalf("expected non-nil cache")
	}
	if _, ok := c.Lookup("broken.bin"); ok {
		t.Fatalf("unresolved fragments entry must not be cached")
	}
	if _, ok := c.Lookup("ok.bin"); !ok {
		t.Fatalf("non-fragments entry should still be cached")
	}
}

func TestBuildCacheFromIndexFragmentsNoResolver(t *testing.T) {
	// If a fragments entry is present but no resolver is wired up,
	// the entry must be skipped silently instead of using the raw
	// fragments-meta hash (which would be wrong).
	mtime := time.Unix(1_700_000_400, 0)
	idx := &index.Index{Entries: []*index.Entry{
		{
			Name:       "frag.bin",
			Hash:       mustHash(0x55),
			Size:       1,
			Mode:       filemode.Regular | filemode.Fragments,
			Stage:      index.Merged,
			ModifiedAt: mtime,
		},
	}}

	c := buildCacheFromIndex(context.Background(), idx, nil)
	if c == nil {
		t.Fatalf("expected non-nil cache")
	}
	if c.Len() != 0 {
		t.Fatalf("expected fragments entry to be skipped without resolver, got %d entries", c.Len())
	}
}

// TestBuildCacheFromIndexFragmentsResolverPassThrough mirrors the
// production resolver's "could not load fragments meta" path: it
// returns the input entry unchanged. The cache must then NOT store
// the fragments-meta hash/size, because the worktree side would never
// match it and we would also risk leaking meta values out as if they
// were the real blob.
func TestBuildCacheFromIndexFragmentsResolverPassThrough(t *testing.T) {
	mtime := time.Unix(1_700_000_500, 0)
	resolver := &fakeResolver{
		// Verbatim pass-through is the "failure" signal in
		// (*Worktree).resolveFragmentsIndex.
		resolve: func(e *index.Entry) *index.Entry { return e },
	}

	idx := &index.Index{Entries: []*index.Entry{
		{
			Name:       "frag.bin",
			Hash:       mustHash(0x99),
			Size:       1,
			Mode:       filemode.Regular | filemode.Fragments,
			Stage:      index.Merged,
			ModifiedAt: mtime,
		},
	}}

	c := buildCacheFromIndex(context.Background(), idx, resolver.Fn())
	if c == nil {
		t.Fatalf("expected non-nil cache")
	}
	if _, ok := c.Lookup("frag.bin"); ok {
		t.Fatalf("fragments entry with pass-through resolver must not be cached")
	}
}
