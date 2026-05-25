package filesystem

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
)

func WalkNode(ctx context.Context, n noder.Noder) {
	nodes, err := n.Children(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "walk error: %s\n", err)
		return
	}
	for _, a := range nodes {
		if a.IsDir() {
			WalkNode(ctx, a)
			continue
		}
		fmt.Fprintf(os.Stderr, "%s\n", a.String())
	}
}

func TestNode(t *testing.T) {
	n := NewRootNode("/tmp/fsnode", noder.NewSparseTreeMatcher([]string{"a", "a/a", "c"}))
	WalkNode(t.Context(), n)
}

func TestNode2(t *testing.T) {
	n := NewRootNode("/tmp/fsnode", noder.NewSparseTreeMatcher([]string{}))
	WalkNode(t.Context(), n)
}

func TestNode3(t *testing.T) {
	n := NewRootNode("/tmp/xh5", noder.NewSparseTreeMatcher([]string{"dir1", "dir3"}))
	WalkNode(t.Context(), n)
}

// expectedHash mirrors what (*Node).calculateHash produces for a
// regular file: BLAKE3 of the file contents followed by the encoded
// FileMode.
func expectedHash(t *testing.T, path string, mode filemode.FileMode) []byte {
	t.Helper()
	h := plumbing.NewHasher()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if _, err := h.Write(data); err != nil {
		t.Fatalf("hash write: %v", err)
	}
	out := h.Sum()
	return append(out[:], mode.Bytes()...)
}

// resolveLeaf walks the noder tree and returns the *Node at relPath.
func resolveLeaf(t *testing.T, root noder.Noder, relPath string) *Node {
	t.Helper()
	cur, ok := root.(*Node)
	if !ok {
		t.Fatalf("root is not a *Node: %T", root)
	}
	for i, part := range strings.Split(relPath, "/") {
		children, err := cur.Children(t.Context())
		if err != nil {
			t.Fatalf("Children(%q) error: %v", cur.path, err)
		}
		var next *Node
		for _, c := range children {
			cn, ok := c.(*Node)
			if !ok {
				continue
			}
			if cn.Name() == part {
				next = cn
				break
			}
		}
		if next == nil {
			t.Fatalf("could not find %q under %q (depth %d)", part, cur.path, i)
		}
		cur = next
	}
	return cur
}

func TestNodeHashCacheHit(t *testing.T) {
	dir := t.TempDir()
	rel := "big.bin"
	full := filepath.Join(dir, rel)
	content := bytes.Repeat([]byte{0xAB}, 4096)
	if err := os.WriteFile(full, content, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	fi, err := os.Stat(full)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	want := expectedHash(t, full, filemode.Regular)

	cache := NewCache()
	cache.Put(rel, CacheEntry{
		Size:       fi.Size(),
		ModifiedAt: fi.ModTime(),
		Hash:       want,
	})

	root := NewRootNodeWithCache(dir, nil, cache)
	leaf := resolveLeaf(t, root, rel)

	// Delete the file. If the cache is honored Hash() must not
	// touch the filesystem and must still return the cached bytes.
	if err := os.Remove(full); err != nil {
		t.Fatalf("remove: %v", err)
	}

	if got := leaf.Hash(); !bytes.Equal(got, want) {
		t.Fatalf("Hash mismatch: got %x, want %x", got, want)
	}
}

// TestNodeHashCacheReturnsCopy ensures Hash() returns a slice that
// does not alias the underlying cache storage. If a caller mutated
// the returned bytes the corruption would propagate to every other
// Node served by the same cache (and to future lookups).
func TestNodeHashCacheReturnsCopy(t *testing.T) {
	dir := t.TempDir()
	rel := "f.bin"
	full := filepath.Join(dir, rel)
	if err := os.WriteFile(full, []byte("ABC"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	fi, err := os.Stat(full)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	want := expectedHash(t, full, filemode.Regular)
	cache := NewCache()
	cache.Put(rel, CacheEntry{
		Size:       fi.Size(),
		ModifiedAt: fi.ModTime(),
		Hash:       append([]byte(nil), want...),
	})

	leaf := resolveLeaf(t, NewRootNodeWithCache(dir, nil, cache), rel)
	got := leaf.Hash()
	if !bytes.Equal(got, want) {
		t.Fatalf("Hash mismatch on first call: got %x want %x", got, want)
	}
	// Mutate the returned slice in place.
	for i := range got {
		got[i] ^= 0xFF
	}
	// A fresh Node served by the same cache must still see the
	// original value.
	leaf2 := resolveLeaf(t, NewRootNodeWithCache(dir, nil, cache), rel)
	if again := leaf2.Hash(); !bytes.Equal(again, want) {
		t.Fatalf("cache was corrupted by mutating a previous Hash() return value:\n  got  %x\n  want %x", again, want)
	}
}

func TestNodeHashCacheMissOnSize(t *testing.T) {
	dir := t.TempDir()
	rel := "f.bin"
	full := filepath.Join(dir, rel)
	if err := os.WriteFile(full, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	fi, err := os.Stat(full)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	bogus := bytes.Repeat([]byte{0xFF}, HashSize)
	cache := NewCache()
	cache.Put(rel, CacheEntry{
		Size:       fi.Size() + 10, // intentionally wrong
		ModifiedAt: fi.ModTime(),
		Hash:       bogus,
	})

	leaf := resolveLeaf(t, NewRootNodeWithCache(dir, nil, cache), rel)
	got := leaf.Hash()
	if bytes.Equal(got, bogus) {
		t.Fatalf("Hash unexpectedly returned cached bogus value: %x", got)
	}
	if want := expectedHash(t, full, filemode.Regular); !bytes.Equal(got, want) {
		t.Fatalf("Hash mismatch after miss: got %x, want %x", got, want)
	}
}

func TestNodeHashCacheMissOnMTime(t *testing.T) {
	dir := t.TempDir()
	rel := "f.bin"
	full := filepath.Join(dir, rel)
	if err := os.WriteFile(full, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	fi, err := os.Stat(full)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	bogus := bytes.Repeat([]byte{0xFE}, HashSize)
	cache := NewCache()
	cache.Put(rel, CacheEntry{
		Size:       fi.Size(),
		ModifiedAt: fi.ModTime().Add(-2 * time.Hour),
		Hash:       bogus,
	})

	leaf := resolveLeaf(t, NewRootNodeWithCache(dir, nil, cache), rel)
	got := leaf.Hash()
	if bytes.Equal(got, bogus) {
		t.Fatalf("expected cache miss on stale mtime")
	}
	if want := expectedHash(t, full, filemode.Regular); !bytes.Equal(got, want) {
		t.Fatalf("Hash mismatch: got %x, want %x", got, want)
	}
}

func TestNodeHashCacheMissOnMode(t *testing.T) {
	dir := t.TempDir()
	rel := "f.bin"
	full := filepath.Join(dir, rel)
	if err := os.WriteFile(full, []byte("payload"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}
	fi, err := os.Stat(full)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	// Cache claims this is a Regular file, but the local file is
	// actually Executable. The mode guard must reject the entry.
	bogus := bytes.Repeat([]byte{0x11}, plumbing.HASH_DIGEST_SIZE)
	cached := append([]byte{}, bogus...)
	cached = append(cached, filemode.Regular.Bytes()...)
	cache := NewCache()
	cache.Put(rel, CacheEntry{
		Size:       fi.Size(),
		ModifiedAt: fi.ModTime(),
		Hash:       cached,
	})

	leaf := resolveLeaf(t, NewRootNodeWithCache(dir, nil, cache), rel)
	got := leaf.Hash()
	if want := expectedHash(t, full, filemode.Executable); !bytes.Equal(got, want) {
		t.Fatalf("Hash mismatch on mode mismatch: got %x, want %x", got, want)
	}
}

func TestNodeHashWithoutCache(t *testing.T) {
	dir := t.TempDir()
	rel := "f.bin"
	full := filepath.Join(dir, rel)
	if err := os.WriteFile(full, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	leaf := resolveLeaf(t, NewRootNode(dir, nil), rel)
	got := leaf.Hash()
	if want := expectedHash(t, full, filemode.Regular); !bytes.Equal(got, want) {
		t.Fatalf("Hash mismatch without cache: got %x, want %x", got, want)
	}
}

// TestNodeHashCacheSkippedForSymlink verifies symlinks never consult
// the cache, even when an entry happens to be keyed under the same
// path. Symlink hashes are derived from the link target, not from file
// contents, so honoring a regular-file cache entry would silently
// corrupt the diff.
func TestNodeHashCacheSkippedForSymlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is unreliable on windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	if err := os.WriteFile(target, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	rel := "link"
	link := filepath.Join(dir, rel)
	if err := os.Symlink("target.txt", link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	fi, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}

	// Plant a bogus regular-file cache entry under the symlink's
	// path. If the cache were consulted for symlinks the returned
	// hash would equal `cached`; we assert it does not.
	bogus := bytes.Repeat([]byte{0xCC}, plumbing.HASH_DIGEST_SIZE)
	cached := append([]byte{}, bogus...)
	cached = append(cached, filemode.Regular.Bytes()...)
	cache := NewCache()
	cache.Put(rel, CacheEntry{
		Size:       fi.Size(),
		ModifiedAt: fi.ModTime(),
		Hash:       cached,
	})

	leaf := resolveLeaf(t, NewRootNodeWithCache(dir, nil, cache), rel)
	got := leaf.Hash()
	if bytes.Equal(got, cached) {
		t.Fatalf("symlink Hash unexpectedly used cached regular-file bytes: %x", got)
	}

	// The expected symlink hash is BLAKE3(target) followed by the
	// encoded symlink mode.
	h := plumbing.NewHasher()
	if _, err := h.Write([]byte("target.txt")); err != nil {
		t.Fatalf("hash write: %v", err)
	}
	sum := h.Sum()
	want := append(sum[:], filemode.Symlink.Bytes()...)
	if !bytes.Equal(got, want) {
		t.Fatalf("symlink Hash mismatch: got %x, want %x", got, want)
	}
}
