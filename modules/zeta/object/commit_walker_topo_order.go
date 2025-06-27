package object

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/emirpasic/gods/trees/binaryheap"
)

type commitStacker interface {
	Push(c *Commit)
	Pop() (*Commit, bool)
	Peek() (*Commit, bool)
	Size() int
}

type commitStack struct {
	stack []*Commit
}

func (cs *commitStack) Push(c *Commit) {
	cs.stack = append(cs.stack, c)
}

// Pop pops the most recently added CommitNode from the stack
func (cs *commitStack) Pop() (*Commit, bool) {
	if len(cs.stack) == 0 {
		return nil, false
	}
	c := cs.stack[len(cs.stack)-1]
	cs.stack = cs.stack[:len(cs.stack)-1]
	return c, true
}

// Peek returns the most recently added CommitNode from the stack without removing it
func (cs *commitStack) Peek() (*Commit, bool) {
	if len(cs.stack) == 0 {
		return nil, false
	}
	return cs.stack[len(cs.stack)-1], true
}

// Size returns the number of CommitNodes in the stack
func (cs *commitStack) Size() int {
	return len(cs.stack)
}

// commitHeap is a stack implementation using an underlying binary heap
type commitHeap struct {
	*binaryheap.Heap
}

// Push pushes a new CommitNode to the heap
func (h *commitHeap) Push(c *Commit) {
	h.Heap.Push(c)
}

// Pop removes top element on heap and returns it, or nil if heap is empty.
// Second return parameter is true, unless the heap was empty and there was nothing to pop.
func (h *commitHeap) Pop() (*Commit, bool) {
	c, ok := h.Heap.Pop()
	if !ok {
		return nil, false
	}
	return c.(*Commit), true
}

// Peek returns top element on the heap without removing it, or nil if heap is empty.
// Second return parameter is true, unless the heap was empty and there was nothing to peek.
func (h *commitHeap) Peek() (*Commit, bool) {
	c, ok := h.Heap.Peek()
	if !ok {
		return nil, false
	}
	return c.(*Commit), true
}

// Size returns number of elements within the heap.
func (h *commitHeap) Size() int {
	return h.Heap.Size()
}

// composeIgnores composes the ignore list with the provided seenExternal list
func composeIgnores(ignore []plumbing.Hash, seenExternal map[plumbing.Hash]bool) map[plumbing.Hash]bool {
	seen := make(map[plumbing.Hash]bool)
	for _, h := range ignore {
		seen[h] = true
	}
	for h := range seenExternal {
		seen[h] = true
	}
	return seen
}

type commitTopoOrderIterator struct {
	explorerStack commitStacker
	visitStack    commitStacker
	inCounts      map[plumbing.Hash]int
	seen          map[plumbing.Hash]bool
}

func NewCommitIterTopoOrder(c *Commit, seenExternal map[plumbing.Hash]bool, ignore []plumbing.Hash) *commitTopoOrderIterator {
	heap := &commitHeap{
		Heap: binaryheap.NewWith(func(a, b any) int {
			return b.(*Commit).Committer.When.Compare(a.(*Commit).Committer.When)
		}),
	}
	stack := &commitStack{
		stack: make([]*Commit, 0, 8),
	}
	seen := composeIgnores(ignore, seenExternal)
	if !seen[c.Hash] {
		heap.Push(c)
		stack.Push(c)
	}
	return &commitTopoOrderIterator{
		explorerStack: heap,
		visitStack:    stack,
		inCounts:      make(map[plumbing.Hash]int),
		seen:          seen,
	}
}

func (w *commitTopoOrderIterator) Next(ctx context.Context) (*Commit, error) {
	var next *Commit
	for {
		var ok bool
		next, ok = w.visitStack.Pop()
		if !ok {
			return nil, io.EOF
		}
		if w.inCounts[next.Hash] == 0 {
			break
		}
	}
	parents := make([]*Commit, 0, len(next.Parents))
	for _, h := range next.Parents {
		pc, err := next.b.Commit(ctx, h)
		if plumbing.IsNoSuchObject(err) {
			parents = append(parents, nil) // no such object --> shallow checkout
			continue
		}
		if err != nil {
			return nil, err
		}
		parents = append(parents, pc)
	}
	// EXPLORE
	for {
		toExplore, ok := w.explorerStack.Peek()
		if !ok {
			break
		}
		if toExplore.Hash != next.Hash && w.explorerStack.Size() == 1 {
			break
		}
		w.explorerStack.Pop()
		for _, h := range toExplore.Parents {
			if w.seen[h] {
				continue
			}
			w.inCounts[h]++
			if w.inCounts[h] == 1 {
				pc, err := toExplore.b.Commit(ctx, h)
				if plumbing.IsNoSuchObject(err) {
					continue
				}
				if err != nil {
					return nil, err
				}
				w.explorerStack.Push(pc)
			}
		}
	}
	for i, h := range next.Parents {
		if w.seen[h] {
			continue
		}
		w.inCounts[h]--
		if w.inCounts[h] == 0 {
			if pc := parents[i]; pc != nil {
				w.visitStack.Push(pc)
			}
		}
	}
	delete(w.inCounts, next.Hash)

	return next, nil
}

func (w *commitTopoOrderIterator) ForEach(ctx context.Context, cb func(*Commit) error) error {
	for {
		c, err := w.Next(ctx)
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		err = cb(c)
		if err == plumbing.ErrStop {
			break
		}
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *commitTopoOrderIterator) Close() {}
