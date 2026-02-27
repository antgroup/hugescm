package object

import (
	"context"
	"io"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/emirpasic/gods/trees/binaryheap"
)

// commitStacker is an interface for commit collection data structures used by
// the topological order iterator. It provides basic stack/heap operations.
type commitStacker interface {
	Push(c *Commit)
	Pop() (*Commit, bool)
	Peek() (*Commit, bool)
	Size() int
}

// commitStack implements a LIFO stack for commits.
type commitStack struct {
	stack []*Commit
}

func (cs *commitStack) Push(c *Commit) {
	cs.stack = append(cs.stack, c)
}

// Pop removes and returns the most recently added commit from the stack.
// Returns false if the stack is empty.
func (cs *commitStack) Pop() (*Commit, bool) {
	if len(cs.stack) == 0 {
		return nil, false
	}
	c := cs.stack[len(cs.stack)-1]
	cs.stack = cs.stack[:len(cs.stack)-1]
	return c, true
}

// Peek returns the most recently added commit from the stack without removing it.
// Returns false if the stack is empty.
func (cs *commitStack) Peek() (*Commit, bool) {
	if len(cs.stack) == 0 {
		return nil, false
	}
	return cs.stack[len(cs.stack)-1], true
}

// Size returns the number of commits currently in the stack.
func (cs *commitStack) Size() int {
	return len(cs.stack)
}

// commitHeap implements commitStacker using a binary heap (priority queue).
// The heap is ordered by commit timestamp to ensure commits are visited
// in chronological order.
type commitHeap struct {
	*binaryheap.Heap
}

// Push adds a new commit to the heap.
func (h *commitHeap) Push(c *Commit) {
	h.Heap.Push(c)
}

// Pop removes and returns the top element from the heap.
// Returns false if the heap is empty.
func (h *commitHeap) Pop() (*Commit, bool) {
	c, ok := h.Heap.Pop()
	if !ok {
		return nil, false
	}
	return c.(*Commit), true
}

// Peek returns the top element from the heap without removing it.
// Returns false if the heap is empty.
func (h *commitHeap) Peek() (*Commit, bool) {
	c, ok := h.Heap.Peek()
	if !ok {
		return nil, false
	}
	return c.(*Commit), true
}

// Size returns the number of elements in the heap.
func (h *commitHeap) Size() int {
	return h.Heap.Size()
}

// composeIgnores combines the explicit ignore list with the seenExternal set
// to create a unified map of commits to skip during traversal.
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

// commitTopoOrderIterator implements topological sorting of commits in the commit graph.
// It ensures that a commit is only visited after all commits that point to it have been
// visited (i.e., parent commits are visited before their children).
// This is the standard "git log --topo-order" behavior.
type commitTopoOrderIterator struct {
	// explorerStack is a heap ordered by commit time, used to discover commits
	explorerStack commitStacker
	// visitStack is a LIFO stack that holds commits ready to be visited
	visitStack commitStacker
	// inCounts tracks how many unvisited children each commit has
	// A commit with inCount == 0 is ready to visit
	inCounts map[plumbing.Hash]int
	// seen tracks commits that should be skipped (ignore list or seenExternal)
	seen map[plumbing.Hash]bool
}

// NewCommitIterTopoOrder creates a new iterator that walks commits in topological order.
// This means commits are output such that they appear in reverse chronological order,
// but with a constraint that a commit appears before any of its descendants.
// This is similar to "git log --topo-order".
func NewCommitIterTopoOrder(c *Commit, seenExternal map[plumbing.Hash]bool, ignore []plumbing.Hash) *commitTopoOrderIterator {
	// Create a heap ordered by commit timestamp (newest first)
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

// Next returns the next commit in topological order.
//
// Algorithm:
//  1. Pop from visitStack until we find a commit with inCount == 0
//  2. Load the commit's parents (nil if missing in shallow clone)
//  3. EXPLORE phase: Pop from explorerStack, increment inCounts for all parents
//     This counts how many unvisited children each parent has
//  4. Decrement inCounts for the current commit's parents
//     If a parent's inCount reaches 0, it's ready to visit, so push to visitStack
//
// This ensures a commit is only visited after all commits pointing to it have been visited.
func (w *commitTopoOrderIterator) Next(ctx context.Context) (*Commit, error) {
	var next *Commit
	// Step 1: Find a commit ready to visit (inCount == 0)
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

	// Step 2: Load parent commits (nil if missing in shallow clone)
	parents := make([]*Commit, 0, len(next.Parents))
	for _, h := range next.Parents {
		pc, err := next.b.Commit(ctx, h)
		if plumbing.IsNoSuchObject(err) {
			parents = append(parents, nil) // Missing commit in shallow clone
			continue
		}
		if err != nil {
			return nil, err
		}
		parents = append(parents, pc)
	}

	// Step 3: EXPLORE phase - discover commits and count references
	// Pop commits from explorerStack until we're at the same level as next
	for {
		toExplore, ok := w.explorerStack.Peek()
		if !ok {
			break
		}
		if toExplore.Hash != next.Hash && w.explorerStack.Size() == 1 {
			break
		}
		w.explorerStack.Pop()
		// For each parent, increment inCount (counting how many children reference it)
		for _, h := range toExplore.Parents {
			if w.seen[h] {
				continue
			}
			w.inCounts[h]++
			if w.inCounts[h] == 1 {
				// First time seeing this commit, add to explorerStack
				pc, err := toExplore.b.Commit(ctx, h)
				if plumbing.IsNoSuchObject(err) {
					// Skip missing commits in shallow clone
					continue
				}
				if err != nil {
					return nil, err
				}
				w.explorerStack.Push(pc)
			}
		}
	}

	// Step 4: Decrement inCounts for current commit's parents
	// If inCount reaches 0, the parent is ready to visit
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

// ForEach iterates through all commits in topological order, calling the callback for each one.
// Iteration stops if the callback returns an error or ErrStop.
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

// Close is a no-op for the topological order iterator.
func (w *commitTopoOrderIterator) Close() {}
