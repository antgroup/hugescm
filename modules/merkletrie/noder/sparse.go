package noder

import (
	"path"
	"strings"

	"github.com/antgroup/hugescm/modules/strengthen"
)

type Matcher interface {
	Len() int
	Match(name string) (Matcher, bool)
}

type sparseTreeMatcher struct {
	entries map[string]*sparseTreeMatcher
}

func (m *sparseTreeMatcher) Len() int {
	return len(m.entries)
}

func (m *sparseTreeMatcher) Match(name string) (Matcher, bool) {
	sm, ok := m.entries[name]
	return sm, ok
}

func (m *sparseTreeMatcher) insert(p string) {
	dv := strengthen.StrSplitSkipEmpty(p, '/', 10)
	current := m
	for _, d := range dv {
		e, ok := current.entries[d]
		if !ok {
			e = &sparseTreeMatcher{entries: make(map[string]*sparseTreeMatcher)}
			current.entries[d] = e
		}
		current = e
	}
}

func NewSparseTreeMatcher(dirs []string) Matcher {
	root := &sparseTreeMatcher{entries: make(map[string]*sparseTreeMatcher)}
	for _, d := range dirs {
		root.insert(d)
	}
	return root
}

type SparseMatcher interface {
	Match(string) bool
}

type sparseMatcher struct {
	sparseEntries []string
}

const (
	dot = "."
)

// isSparseMatch: sparse match dir
// eg:
//
// sparseDir: foo/bar
// parent: foo/bar/abc --> match
// parent: foo/abc --> not macth
// parent: foo --> match
func isSparseMatch(sparseDir, parent string) bool {
	parent += "/"
	return strings.HasPrefix(parent, sparseDir) || strings.HasPrefix(sparseDir, parent)
}

func (m *sparseMatcher) Match(name string) bool {
	if len(m.sparseEntries) == 0 {
		return true
	}
	parent := path.Dir(name)
	if parent == dot {
		return true
	}
	for _, sparseDir := range m.sparseEntries {
		if isSparseMatch(sparseDir, parent) {
			return true
		}
	}
	return false
}

func NewSparseMatcher(dirs []string) SparseMatcher {
	entries := make([]string, 0, len(dirs))
	for _, d := range dirs {
		p := path.Clean(d)
		if p == dot {
			continue
		}
		entries = append(entries, p+"/")
	}
	return &sparseMatcher{sparseEntries: entries}
}
