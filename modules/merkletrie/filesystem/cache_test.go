package filesystem

import (
	"testing"
	"time"
)

func TestCachePutLookup(t *testing.T) {
	c := NewCache()
	if c.Len() != 0 {
		t.Fatalf("new cache should be empty, got %d", c.Len())
	}

	now := time.Now()
	hash := make([]byte, HashSize)
	for i := range hash {
		hash[i] = byte(i)
	}
	c.Put("a/b", CacheEntry{Size: 123, ModifiedAt: now, Hash: hash})

	got, ok := c.Lookup("a/b")
	if !ok {
		t.Fatalf("expected cache hit on a/b")
	}
	if got.Size != 123 || !got.ModifiedAt.Equal(now) || len(got.Hash) != HashSize {
		t.Fatalf("unexpected cached entry: %+v", got)
	}

	if _, ok := c.Lookup("missing"); ok {
		t.Fatalf("expected miss on missing key")
	}

	if c.Len() != 1 {
		t.Fatalf("Len mismatch, want 1 got %d", c.Len())
	}
}

func TestCacheNilSafe(t *testing.T) {
	var c *Cache
	// Put / Lookup / Len on nil receiver must not panic.
	c.Put("x", CacheEntry{})
	if _, ok := c.Lookup("x"); ok {
		t.Fatalf("nil cache should never report a hit")
	}
	if c.Len() != 0 {
		t.Fatalf("nil cache Len should be 0")
	}
}

func TestSameModTime(t *testing.T) {
	base := time.Unix(1_700_000_000, 123_456_000) // 123456 microseconds
	cases := []struct {
		name string
		a, b time.Time
		want bool
	}{
		{"identical", base, base, true},
		{"different second", base, base.Add(time.Second), false},
		{"sub-microsecond drift", base, base.Add(500 * time.Nanosecond), true},
		{"different microsecond", base, base.Add(2 * time.Microsecond), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameModTime(tc.a, tc.b); got != tc.want {
				t.Fatalf("sameModTime(%v,%v) = %v, want %v",
					tc.a, tc.b, got, tc.want)
			}
		})
	}
}
