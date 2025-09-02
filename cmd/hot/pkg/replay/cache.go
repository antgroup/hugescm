package replay

import (
	"encoding/hex"
	"fmt"

	"github.com/antgroup/hugescm/modules/git/gitobj"
)

// cacheEntry caches then given "from" entry so that it is always rewritten as
// a *TreeEntry equivalent to "to".
func (r *Replayer) cacheEntry(path string, from, to *gitobj.TreeEntry) *gitobj.TreeEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.entries[r.entryKey(path, from)] = to

	return to
}

// uncacheEntry returns a *TreeEntry that is cached from the given *TreeEntry
// "from". That is to say, it returns the *TreeEntry that "from" should be
// rewritten to, or nil if none could be found.
func (r *Replayer) uncacheEntry(path string, from *gitobj.TreeEntry) *gitobj.TreeEntry {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.entries[r.entryKey(path, from)]
}

// entryKey returns a unique key for a given *TreeEntry "e".
func (r *Replayer) entryKey(path string, e *gitobj.TreeEntry) string {
	return fmt.Sprintf("%s:%x", path, e.Oid)
}

// cacheEntry caches then given "from" commit so that it is always rewritten as
// a *git/gitobj.Commit equivalent to "to".
func (r *Replayer) cacheCommit(from, to []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.commits[hex.EncodeToString(from)] = to
}

// uncacheCommit returns a *git/gitobj.Commit that is cached from the given
// *git/gitobj.Commit "from". That is to say, it returns the *git/gitobj.Commit that
// "from" should be rewritten to and true, or nil and false if none could be
// found.
func (r *Replayer) uncacheCommit(from []byte) ([]byte, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	c, ok := r.commits[hex.EncodeToString(from)]
	return c, ok
}

func copyEntry(e *gitobj.TreeEntry) *gitobj.TreeEntry {
	if e == nil {
		return nil
	}

	oid := make([]byte, len(e.Oid))
	copy(oid, e.Oid)

	return &gitobj.TreeEntry{
		Filemode: e.Filemode,
		Name:     e.Name,
		Oid:      oid,
	}
}

func copyEntryMode(e *gitobj.TreeEntry, mode int32) *gitobj.TreeEntry {
	copied := copyEntry(e)
	copied.Filemode = mode

	return copied
}
