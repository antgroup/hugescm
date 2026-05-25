package filesystem

import (
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// HashSize is the total byte length produced by Node.Hash() and the
// peer index.Node.Hash() implementation: plumbing.Hash followed by the
// 4-byte little-endian filemode encoding.
const HashSize = plumbing.HASH_DIGEST_SIZE + 4

// CacheEntry is the precomputed hash of a worktree file together with
// the stat fields that prove it is still up-to-date. When the file at
// the same path still has the same size, mtime and mode, callers may
// reuse Hash and skip the BLAKE3 computation over the file contents.
//
// Hash is the 36-byte concatenation of a plumbing.Hash and the encoded
// FileMode. This layout matches what Node.Hash() produces, so the
// diff routine can compare cached and freshly computed values with a
// plain bytes.Equal.
type CacheEntry struct {
	Size       int64
	ModifiedAt time.Time
	Hash       []byte
}

// Cache stores CacheEntry values keyed by slash-separated worktree
// path. It is built once by the caller before a status / diff run and
// is treated as read-only afterwards. Many Node instances share the
// same Cache pointer, so it does not provide its own locking.
type Cache struct {
	m map[string]CacheEntry
}

// NewCache returns an empty Cache.
func NewCache() *Cache {
	return &Cache{m: make(map[string]CacheEntry)}
}

// Put inserts an entry. path must be the slash-separated worktree
// path that Node.path uses.
func (c *Cache) Put(path string, e CacheEntry) {
	if c == nil || c.m == nil {
		return
	}
	c.m[path] = e
}

// Lookup returns the entry for the given path. The returned Hash
// slice must not be mutated by callers.
func (c *Cache) Lookup(path string) (CacheEntry, bool) {
	if c == nil || c.m == nil {
		return CacheEntry{}, false
	}
	e, ok := c.m[path]
	return e, ok
}

// Len returns the number of cached entries; mainly useful for tests.
func (c *Cache) Len() int {
	if c == nil {
		return 0
	}
	return len(c.m)
}
