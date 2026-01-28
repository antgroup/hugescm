// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/streamio"
)

const (
	maxTreeDepth      = 1024
	startingStackSize = 8
)

// New errors defined by this package.
var (
	TREE_MAGIC      = [4]byte{'Z', 'T', 0x00, 0x01}
	ErrMaxTreeDepth = errors.New("maximum tree depth exceeded")
)

type ErrDirectoryNotFound struct {
	dir string
}

func (e *ErrDirectoryNotFound) Error() string {
	return fmt.Sprintf("dir '%s' not found", e.dir)
}

func IsErrDirectoryNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrDirectoryNotFound)
	return ok
}

type ErrEntryNotFound struct {
	entry string
}

func (e *ErrEntryNotFound) Error() string {
	return fmt.Sprintf("entry '%s' not found", e.entry)
}

func IsErrEntryNotFound(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrEntryNotFound)
	return ok
}

// TreeEntry represents a file
type TreeEntry struct {
	Name    string            `json:"name"`
	Size    int64             `json:"size"`
	Mode    filemode.FileMode `json:"mode"`
	Hash    plumbing.Hash     `json:"hash"`
	Payload []byte            `json:"-"`
}

func (e *TreeEntry) Clone() *TreeEntry {
	return &TreeEntry{
		Name:    e.Name,
		Size:    e.Size,
		Mode:    e.Mode,
		Hash:    e.Hash,
		Payload: bytes.Clone(e.Payload),
	}
}

// Equal returns whether the receiving and given TreeEntry instances are
// identical in name, filemode, and OID.
func (e *TreeEntry) Equal(other *TreeEntry) bool {
	if (e == nil) != (other == nil) {
		return false
	}

	if e != nil {
		return e.Name == other.Name &&
			bytes.Equal(e.Hash[:], other.Hash[:]) &&
			e.Mode == other.Mode
	}
	return true
}

const (
	sIFMT      = filemode.FileMode(0170000)
	sIFREG     = filemode.FileMode(0100000)
	sIFDIR     = filemode.FileMode(0040000)
	sIFLNK     = filemode.FileMode(0120000)
	sIFGITLINK = filemode.FileMode(0160000)
	sIFRAGMENT = filemode.Fragments
)

func (e *TreeEntry) Type() ObjectType {
	if e.Mode&sIFRAGMENT != 0 {
		return FragmentsObject
	}
	switch e.Mode & sIFMT {
	case sIFREG:
		return BlobObject
	case sIFDIR:
		return TreeObject
	case sIFLNK:
		return BlobObject
	case sIFGITLINK:
		return CommitObject
	default:
	}
	return 0
}

// IsLink returns true if the given TreeEntry is a blob which represents a
// symbolic link (i.e., with a filemode of 0120000.
func (e *TreeEntry) IsLink() bool {
	return e.Mode&sIFMT == sIFLNK
}

func (e *TreeEntry) IsDir() bool {
	return e.Mode&sIFMT == sIFDIR
}

func (e *TreeEntry) IsRegular() bool {
	return e.Mode&sIFMT == sIFREG
}

func (e *TreeEntry) IsFragments() bool {
	return e.Mode&filemode.Fragments != 0
}

func (e *TreeEntry) OriginMode() filemode.FileMode {
	return e.Mode &^ filemode.Fragments
}

// check if entry renamed
func (e *TreeEntry) Renamed(other *TreeEntry) bool {
	return e.Mode == other.Mode && e.Hash == other.Hash
}

func (e *TreeEntry) Chmod(other *TreeEntry) bool {
	return e.Mode != other.Mode && e.Hash == other.Hash && e.Name == other.Name
}

// entry with same name
func (e *TreeEntry) Modified(other *TreeEntry) bool {
	return e.Name == other.Name
}

// SubtreeOrder is an implementation of sort.Interface that sorts a set of
// `*TreeEntry`'s according to "subtree" order. This ordering is required to
// write trees in a correct, readable format to the Git object database.
//
// The format is as follows: entries are sorted lexicographically in byte-order,
// with subtrees (entries of Type() == object.TreeObjectType) being sorted as
// if their `Name` fields ended in a "/".
//
// See: https://github.com/git/git/blob/v2.13.0/fsck.c#L492-L525 for more
// details.
type SubtreeOrder []*TreeEntry

// Len implements sort.Interface.Len() and return the length of the underlying
// slice.
func (s SubtreeOrder) Len() int { return len(s) }

// Swap implements sort.Interface.Swap() and swaps the two elements at i and j.
func (s SubtreeOrder) Swap(i, j int) { s[i], s[j] = s[j], s[i] }

// Less implements sort.Interface.Less() and returns whether the element at "i"
// is compared as "less" than the element at "j". In other words, it returns if
// the element at "i" should be sorted ahead of that at "j".
//
// It performs this comparison in lexicographic byte-order according to the
// rules above (see SubtreeOrder).
func (s SubtreeOrder) Less(i, j int) bool {
	return s.Name(i) < s.Name(j)
}

// Name returns the name for a given entry indexed at "i", which is a C-style
// string ('\0' terminated unless it's a subtree), optionally terminated with
// '/' if it's a subtree.
//
// This is done because '/' sorts ahead of '\0', and is compatible with the
// tree order in upstream Git.
func (s SubtreeOrder) Name(i int) string {
	if i < 0 || i >= len(s) {
		return ""
	}

	entry := s[i]

	if entry.Type() == TreeObject {
		return entry.Name + "/"
	}
	return entry.Name + "\x00"
}

func (t *Tree) Append(others *TreeEntry) {
	for i, e := range t.Entries {
		if e.Name == others.Name {
			t.Entries[i] = others
			return
		}
	}
	t.Entries = append(t.Entries, others)
}

// Merge performs a merge operation against the given set of `*TreeEntry`'s by
// either replacing existing tree entries of the same name, or appending new
// entries in sub-tree order.
//
// It returns a copy of the tree, and performs the merge in O(n*log(n)) time.
func (t *Tree) Merge(others ...*TreeEntry) *Tree {
	unseen := make(map[string]*TreeEntry)

	// Build a cache of name to *TreeEntry.
	for _, other := range others {
		unseen[other.Name] = other
	}

	// Map the existing entries ("t.Entries") into a new set by either
	// copying an existing entry, or replacing it with a new one.
	entries := make([]*TreeEntry, 0, len(t.Entries))
	for _, entry := range t.Entries {
		if other, ok := unseen[entry.Name]; ok {
			entries = append(entries, other)
			delete(unseen, entry.Name)
		} else {
			entries = append(entries, &TreeEntry{
				Name:    entry.Name,
				Size:    entry.Size,
				Mode:    entry.Mode,
				Hash:    entry.Hash,
				Payload: bytes.Clone(entry.Payload),
			})
		}
	}

	// For all the items we haven't replaced into the new set, append them
	// to the entries.
	for _, remaining := range unseen {
		entries = append(entries, remaining)
	}

	// Call sort afterwords, as a tradeoff between speed and spacial
	// complexity. As a future point of optimization, adding new elements
	// (see: above) could be done as a linear pass of the "entries" set.
	//
	// In order to do that, we must have a constant-time lookup of both
	// entries in the existing and new sets. This requires building a
	// map[string]*TreeEntry for the given "others" as well as "t.Entries".
	//
	// Trees can be potentially large, so trade this spacial complexity for
	// an O(n*log(n)) sort.
	sort.Sort(SubtreeOrder(entries))

	return &Tree{Entries: entries}
}

// Equal returns whether the receiving and given trees are equal, or in other
// words, whether they are represented by the same BLAKE3 when saved to the
// object database.
func (t *Tree) Equal(other *Tree) bool {
	if (t == nil) != (other == nil) {
		return false
	}

	if t != nil {
		if len(t.Entries) != len(other.Entries) {
			return false
		}

		for i := range t.Entries {
			e1 := t.Entries[i]
			e2 := other.Entries[i]

			if !e1.Equal(e2) {
				return false
			}
		}
	}
	return true
}

// Tree is basically like a directory - it references a bunch of other trees
// and/or blobs (i.e. files and sub-directories)
type Tree struct {
	Hash    plumbing.Hash `json:"hash"`
	Entries []*TreeEntry  `json:"entries"`

	m map[string]*TreeEntry
	t map[string]*Tree // tree path cache
	b Backend
}

func NewTree(entries []*TreeEntry) *Tree {
	return &Tree{Entries: entries}
}

// Tree returns the tree identified by the `path` argument.
// The path is interpreted as relative to the tree receiver.
func (t *Tree) Tree(ctx context.Context, path string) (*Tree, error) {
	if len(path) == 0 {
		return t, nil
	}
	e, err := t.FindEntry(ctx, path)
	if err != nil {
		return nil, &ErrDirectoryNotFound{dir: path}
	}
	return resolveTree(ctx, t.b, e.Hash)
}

func (t *Tree) Entry(name string) (*TreeEntry, error) {
	return t.entry(name)
}

// FindEntry search a TreeEntry in this tree or any subtree.
func (t *Tree) FindEntry(ctx context.Context, relativePath string) (*TreeEntry, error) {
	if t.t == nil {
		t.t = make(map[string]*Tree)
	}
	relativePath = filepath.ToSlash(relativePath) // fix on windows

	pathParts := strings.Split(relativePath, "/")
	startingTree := t
	pathCurrent := ""

	// search for the longest path in the tree path cache
	for i := len(pathParts) - 1; i >= 1; i-- {
		path := path.Join(pathParts[:i]...)

		tree, ok := t.t[path]
		if ok {
			startingTree = tree
			pathParts = pathParts[i:]
			pathCurrent = path

			break
		}
	}

	var tree *Tree
	var err error
	for tree = startingTree; len(pathParts) > 1; pathParts = pathParts[1:] {
		if tree, err = tree.dir(ctx, pathParts[0]); err != nil {
			return nil, err
		}

		pathCurrent = path.Join(pathCurrent, pathParts[0])
		t.t[pathCurrent] = tree
	}

	return tree.entry(pathParts[0])
}

func (t *Tree) dir(ctx context.Context, baseName string) (*Tree, error) {
	entry, err := t.entry(baseName)
	if err != nil {
		return nil, &ErrDirectoryNotFound{dir: baseName}
	}
	if t.b == nil {
		return nil, &ErrDirectoryNotFound{dir: baseName}
	}
	tree, err := t.b.Tree(ctx, entry.Hash)
	if err != nil {
		return nil, err
	}
	tree.b = t.b
	return tree, nil
}

func (t *Tree) entry(baseName string) (*TreeEntry, error) {
	if t.m == nil {
		t.buildMap()
	}

	entry, ok := t.m[baseName]
	if !ok {
		return nil, &ErrEntryNotFound{entry: baseName}
	}

	return entry, nil
}

// Files returns a FileIter allowing to iterate over the Tree
func (t *Tree) Files() *FileIter {
	return NewFileIter(t.b, t)
}

func (t *Tree) buildMap() {
	t.m = make(map[string]*TreeEntry)
	for i := range t.Entries {
		t.m[t.Entries[i].Name] = t.Entries[i]
	}
}

func (t *Tree) SpacePadding() int {
	var hasFragments bool
	for _, e := range t.Entries {
		if e.Type() == FragmentsObject {
			hasFragments = true
		}
	}
	if hasFragments {
		return 5
	}
	return 0
}

func (t *Tree) SizePadding() int {
	var v int64
	var hasFragments bool
	for _, e := range t.Entries {
		v = max(v, e.Size)
		if e.Type() == FragmentsObject {
			hasFragments = true
		}
	}
	sizeMax := len(strconv.FormatInt(v, 10))
	if hasFragments {
		// blob/fragments 4/9 d5
		return max(5, sizeMax)
	}
	return sizeMax
}

func (t *Tree) Encode(w io.Writer) error {
	_, err := w.Write(TREE_MAGIC[:])
	if err != nil {
		return err
	}
	for _, entry := range t.Entries {
		size := entry.Size
		if len(entry.Payload) > 0 {
			if size > BlobInlineMaxBytes {
				return fmt.Errorf("tree entry '%s' inline blob '%s' too large", t.Hash, entry.Hash)
			}
			size = -entry.Size
		}
		if _, err = fmt.Fprintf(w, "%o %d %s", entry.Mode, size, entry.Name); err != nil {
			return err
		}

		if _, err = w.Write([]byte{0x00}); err != nil {
			return err
		}

		if _, err = w.Write(entry.Hash[:]); err != nil {
			return err
		}
		if len(entry.Payload) > 0 {
			if _, err = w.Write(entry.Payload); err != nil {
				return err
			}
		}
	}
	return nil
}

func (t *Tree) Decode(reader Reader) error {
	if reader.Type() != TreeObject {
		return ErrUnsupportedObject
	}
	t.Hash = reader.Hash()
	r := streamio.GetBufioReader(reader)
	defer streamio.PutBufioReader(r)

	t.Entries = nil
	for {
		str, err := r.ReadString(' ')
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}
		str = str[:len(str)-1] // strip last byte (' ')

		mode, err := filemode.New(str)
		if err != nil {
			return err
		}

		if str, err = r.ReadString(' '); err != nil {
			if err == io.EOF {
				break
			}

			return err
		}
		size, err := strconv.ParseInt(str[:len(str)-1], 10, 64)
		if err != nil {
			return err
		}

		name, err := r.ReadString(0)
		if err != nil && err != io.EOF {
			return err
		}

		var hash plumbing.Hash
		if _, err = io.ReadFull(r, hash[:]); err != nil {
			return err
		}
		var payload []byte
		if size < 0 {
			size = -size
			if size > BlobInlineMaxBytes {
				return fmt.Errorf("tree entry '%s' inline blob '%s' too large", t.Hash, hash)
			}
			payload = make([]byte, size)
			if _, err := io.ReadFull(r, payload); err != nil {
				return err
			}
		}

		baseName := name[:len(name)-1]
		t.Entries = append(t.Entries, &TreeEntry{
			Name:    baseName,
			Size:    size,
			Mode:    mode,
			Hash:    hash,
			Payload: payload,
		})

	}
	return nil
}

// resolveTree gets a tree from an object storer and decodes it.
func resolveTree(ctx context.Context, b Backend, h plumbing.Hash) (*Tree, error) {
	if b == nil {
		return nil, plumbing.NoSuchObject(h)
	}

	t, err := b.Tree(ctx, h)
	if err != nil {
		return nil, err
	}
	return t, nil
}

// File returns the hash of the file identified by the `path` argument.
// The path is interpreted as relative to the tree receiver.
func (t *Tree) File(ctx context.Context, path string) (*File, error) {
	e, err := t.FindEntry(ctx, path)
	if err != nil {
		return nil, &ErrEntryNotFound{entry: path}
	}
	return newFile(e.Name, path, e.Mode, e.Hash, e.Size, t.b), nil
}

// Diff returns a list of changes between this tree and the provided one
func (t *Tree) Diff(to *Tree, m noder.Matcher) (Changes, error) {
	return t.DiffContext(context.Background(), to, m)
}

// DiffContext returns a list of changes between this tree and the provided one
// Error will be returned if context expires. Provided context must be non nil.
//
// NOTE: Since version 5.1.0 the renames are correctly handled, the settings
// used are the recommended options DefaultDiffTreeOptions.
func (t *Tree) DiffContext(ctx context.Context, to *Tree, m noder.Matcher) (Changes, error) {
	return DiffTreeWithOptions(ctx, t, to, DefaultDiffTreeOptions, m)
}

// StatsContext: stats
func (t *Tree) StatsContext(ctx context.Context, to *Tree, m noder.Matcher, opts *PatchOptions) (FileStats, error) {
	changes, err := t.DiffContext(ctx, to, m)
	if err != nil {
		return nil, err
	}
	return changes.Stats(ctx, opts)
}

// treeEntryIter facilitates iterating through the TreeEntry objects in a Tree.
type treeEntryIter struct {
	t   *Tree
	pos int
}

func (iter *treeEntryIter) Next() (*TreeEntry, error) {
	if iter.pos >= len(iter.t.Entries) {
		return &TreeEntry{}, io.EOF
	}
	iter.pos++
	return iter.t.Entries[iter.pos-1], nil
}

// TreeWalker provides a means of walking through all of the entries in a Tree.
type TreeWalker struct {
	stack     []*treeEntryIter
	base      string
	recursive bool
	seen      map[plumbing.Hash]bool

	b Backend
	t *Tree
}

// NewTreeWalker returns a new TreeWalker for the given tree.
//
// It is the caller's responsibility to call Close() when finished with the
// tree walker.
func NewTreeWalker(t *Tree, recursive bool, seen map[plumbing.Hash]bool) *TreeWalker {
	stack := make([]*treeEntryIter, 0, startingStackSize)
	stack = append(stack, &treeEntryIter{t, 0})

	return &TreeWalker{
		stack:     stack,
		recursive: recursive,
		seen:      seen,

		b: t.b,
		t: t,
	}
}

// Next returns the next object from the tree. Objects are returned in order
// and subtrees are included. After the last object has been returned further
// calls to Next() will return io.EOF.
//
// In the current implementation any objects which cannot be found in the
// underlying repository will be skipped automatically. It is possible that this
// may change in future versions.
func (w *TreeWalker) Next(ctx context.Context) (name string, entry *TreeEntry, err error) {
	var obj *Tree
	for {
		current := len(w.stack) - 1
		if current < 0 {
			// Nothing left on the stack so we're finished
			err = io.EOF
			return
		}

		if current > maxTreeDepth {
			// We're probably following bad data or some self-referencing tree
			err = ErrMaxTreeDepth
			return
		}

		entry, err = w.stack[current].Next()
		if err == io.EOF {
			// Finished with the current tree, move back up to the parent
			w.stack = w.stack[:current]
			w.base, _ = path.Split(w.base)
			w.base = strings.TrimSuffix(w.base, "/")
			continue
		}

		if err != nil {
			return
		}

		if w.seen[entry.Hash] {
			continue
		}

		if entry.Mode == filemode.Dir {
			obj, err = resolveTree(ctx, w.b, entry.Hash)
		}
		if plumbing.IsNoSuchObject(err) {
			continue
		}
		name = simpleJoin(w.base, entry.Name)

		if err != nil {
			err = io.EOF
			return
		}

		break
	}

	if !w.recursive {
		return
	}

	if obj != nil {
		w.stack = append(w.stack, &treeEntryIter{obj, 0})
		w.base = simpleJoin(w.base, entry.Name)
	}

	return
}

// Tree returns the tree that the tree walker most recently operated on.
func (w *TreeWalker) Tree() *Tree {
	current := len(w.stack) - 1
	if w.stack[current].pos == 0 {
		current--
	}

	if current < 0 {
		return nil
	}

	return w.stack[current].t
}

// Close releases any resources used by the TreeWalker.
func (w *TreeWalker) Close() {
	w.stack = nil
}

func simpleJoin(parent, child string) string {
	if len(parent) > 0 {
		return parent + "/" + child
	}
	return child
}
