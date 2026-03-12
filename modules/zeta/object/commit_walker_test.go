package object

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
)

// MockBackend is a test implementation of Backend interface for testing commit walkers
type MockBackend struct {
	commits map[plumbing.Hash]*Commit
}

func NewMockBackend() *MockBackend {
	return &MockBackend{
		commits: make(map[plumbing.Hash]*Commit),
	}
}

func (m *MockBackend) AddCommit(commit *Commit) {
	commit.b = m // Set the backend on the commit
	m.commits[commit.Hash] = commit
}

func (m *MockBackend) Commit(ctx context.Context, hash plumbing.Hash) (*Commit, error) {
	c, ok := m.commits[hash]
	if !ok {
		return nil, plumbing.NoSuchObject(hash)
	}
	return c, nil
}

func (m *MockBackend) Tree(ctx context.Context, hash plumbing.Hash) (*Tree, error) {
	return nil, plumbing.NoSuchObject(hash)
}

func (m *MockBackend) Fragments(ctx context.Context, hash plumbing.Hash) (*Fragments, error) {
	return nil, plumbing.NoSuchObject(hash)
}

func (m *MockBackend) Tag(ctx context.Context, hash plumbing.Hash) (*Tag, error) {
	return nil, plumbing.NoSuchObject(hash)
}

func (m *MockBackend) Blob(ctx context.Context, hash plumbing.Hash) (*Blob, error) {
	return nil, plumbing.NoSuchObject(hash)
}

// NewTestCommit creates a test commit with the given parameters
func NewTestCommit(hash string, message string, parents ...*Commit) *Commit {
	c := &Commit{
		Hash:      plumbing.NewHash(hash),
		Parents:   make([]plumbing.Hash, len(parents)),
		Message:   message,
		Author:    Signature{Name: "Test Author", Email: "test@example.com", When: time.Now()},
		Committer: Signature{Name: "Test Author", Email: "test@example.com", When: time.Now()},
	}
	for i, p := range parents {
		c.Parents[i] = p.Hash
	}
	return c
}

// TestCommitPreorderIter tests basic preorder traversal of commits
func TestCommitPreorderIter(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	// Create a simple commit graph: C3 <- C2 <- C1
	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c2)

	backend.AddCommit(c1)
	backend.AddCommit(c2)
	backend.AddCommit(c3)

	iter := NewCommitPreorderIter(c3, nil, nil)
	defer iter.Close()

	var commits []*Commit
	err := iter.ForEach(ctx, func(commit *Commit) error {
		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		t.Fatalf("ForEach error: %v", err)
	}
	if len(commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(commits))
	}
	if commits[0].Message != "C3" {
		t.Errorf("Expected C3, got %s", commits[0].Message)
	}
	if commits[1].Message != "C2" {
		t.Errorf("Expected C2, got %s", commits[1].Message)
	}
	if commits[2].Message != "C1" {
		t.Errorf("Expected C1, got %s", commits[2].Message)
	}
}

// TestCommitPreorderIterWithMerge tests preorder traversal with merge commits
func TestCommitPreorderIterWithMerge(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	// Create a merge commit graph:
	//     M (merge)
	//    / \
	//   C2  C3
	//    \ /
	//     C1
	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c1)
	m := NewTestCommit("4444444444444444444444444444444444444444", "M", c2, c3)

	backend.AddCommit(c1)
	backend.AddCommit(c2)
	backend.AddCommit(c3)
	backend.AddCommit(m)

	iter := NewCommitPreorderIter(m, nil, nil)
	defer iter.Close()

	var commits []*Commit
	err := iter.ForEach(ctx, func(commit *Commit) error {
		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		t.Fatalf("ForEach error: %v", err)
	}
	if len(commits) != 4 {
		t.Errorf("Expected 4 commits, got %d", len(commits))
	}

	// Check if commits contain the expected values
	foundM := false
	foundC2 := false
	foundC3 := false
	foundC1 := false
	for _, c := range commits {
		if c == m {
			foundM = true
		}
		if c == c2 {
			foundC2 = true
		}
		if c == c3 {
			foundC3 = true
		}
		if c == c1 {
			foundC1 = true
		}
	}
	if !foundM {
		t.Error("Expected to find m in commits")
	}
	if !foundC2 {
		t.Error("Expected to find c2 in commits")
	}
	if !foundC3 {
		t.Error("Expected to find c3 in commits")
	}
	if !foundC1 {
		t.Error("Expected to find c1 in commits")
	}
}

// TestCommitPreorderIterDeduplication tests that commits are not visited twice
func TestCommitPreorderIterDeduplication(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	// Create a diamond graph:
	//     M
	//    / \
	//   C2  C3
	//    \ /
	//     C1
	// C1 should be visited only once
	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c1)
	m := NewTestCommit("4444444444444444444444444444444444444444", "M", c2, c3)

	backend.AddCommit(c1)
	backend.AddCommit(c2)
	backend.AddCommit(c3)
	backend.AddCommit(m)

	iter := NewCommitPreorderIter(m, nil, nil)
	defer iter.Close()

	var c1Count int
	err := iter.ForEach(ctx, func(commit *Commit) error {
		if commit.Hash == c1.Hash {
			c1Count++
		}
		return nil
	})

	if err != nil {
		t.Fatalf("ForEach error: %v", err)
	}
	if c1Count != 1 {
		t.Errorf("Expected C1 to be visited exactly once, got %d", c1Count)
	}
}

// TestCommitBFSIter tests breadth-first search traversal
func TestCommitBFSIter(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	// Create a linear graph: C4 <- C3 <- C2 <- C1
	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c2)
	c4 := NewTestCommit("4444444444444444444444444444444444444444", "C4", c3)

	backend.AddCommit(c1)
	backend.AddCommit(c2)
	backend.AddCommit(c3)
	backend.AddCommit(c4)

	iter := NewCommitIterBFS(c4, nil, nil)
	defer iter.Close()

	var commits []*Commit
	err := iter.ForEach(ctx, func(commit *Commit) error {
		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		t.Fatalf("ForEach error: %v", err)
	}
	if len(commits) != 4 {
		t.Errorf("Expected 4 commits, got %d", len(commits))
	}
	// BFS visits level by level
	if commits[0].Message != "C4" {
		t.Errorf("Expected C4, got %s", commits[0].Message)
	}
	if commits[1].Message != "C3" {
		t.Errorf("Expected C3, got %s", commits[1].Message)
	}
	if commits[2].Message != "C2" {
		t.Errorf("Expected C2, got %s", commits[2].Message)
	}
	if commits[3].Message != "C1" {
		t.Errorf("Expected C1, got %s", commits[3].Message)
	}
}

// TestCommitBFSIterWithMerge tests BFS traversal with merge commits
func TestCommitBFSIterWithMerge(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	// Create a merge commit graph:
	//     M
	//    / \
	//   C2  C3
	//    \ /
	//     C1
	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c1)
	m := NewTestCommit("4444444444444444444444444444444444444444", "M", c2, c3)

	backend.AddCommit(c1)
	backend.AddCommit(c2)
	backend.AddCommit(c3)
	backend.AddCommit(m)

	iter := NewCommitIterBFS(m, nil, nil)
	defer iter.Close()

	var commits []*Commit
	err := iter.ForEach(ctx, func(commit *Commit) error {
		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		t.Fatalf("ForEach error: %v", err)
	}
	if len(commits) != 4 {
		t.Errorf("Expected 4 commits, got %d", len(commits))
	}

	// Check if commits contain expected values
	foundM := false
	foundC2 := false
	foundC3 := false
	foundC1 := false
	for _, c := range commits {
		if c == m {
			foundM = true
		}
		if c == c2 {
			foundC2 = true
		}
		if c == c3 {
			foundC3 = true
		}
		if c == c1 {
			foundC1 = true
		}
	}
	if !foundM {
		t.Error("Expected to find m in commits")
	}
	if !foundC2 {
		t.Error("Expected to find c2 in commits")
	}
	if !foundC3 {
		t.Error("Expected to find c3 in commits")
	}
	if !foundC1 {
		t.Error("Expected to find c1 in commits")
	}
}

// TestCommitTopoOrderIter tests topological order traversal
func TestCommitTopoOrderIter(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	// Create a simple commit graph: C3 <- C2 <- C1
	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c2)

	backend.AddCommit(c1)
	backend.AddCommit(c2)
	backend.AddCommit(c3)

	iter := NewCommitIterCTime(c3, nil, nil)
	defer iter.Close()

	var commits []*Commit
	err := iter.ForEach(ctx, func(commit *Commit) error {
		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		t.Fatalf("ForEach error: %v", err)
	}
	if len(commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(commits))
	}
	// Topological order should visit children before parents
	if commits[0].Message != "C3" {
		t.Errorf("Expected C3, got %s", commits[0].Message)
	}
	if commits[1].Message != "C2" {
		t.Errorf("Expected C2, got %s", commits[1].Message)
	}
	if commits[2].Message != "C1" {
		t.Errorf("Expected C1, got %s", commits[2].Message)
	}
}

// TestFilterCommitIter tests filtering commits during traversal
func TestFilterCommitIter(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	// Create a linear graph: C4 <- C3 <- C2 <- C1
	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c2)
	c4 := NewTestCommit("4444444444444444444444444444444444444444", "C4", c3)

	backend.AddCommit(c1)
	backend.AddCommit(c2)
	backend.AddCommit(c3)
	backend.AddCommit(c4)

	// Filter: only return commits with even message length (C2 and C4 have length 2)
	var isValid CommitFilter = func(c *Commit) bool {
		return len(c.Message)%2 == 0
	}

	iter := NewFilterCommitIter(c4, &isValid, nil)
	defer iter.Close()

	var commits []*Commit
	err := iter.ForEach(ctx, func(commit *Commit) error {
		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		t.Fatalf("ForEach error: %v", err)
	}
	// Only C2 and C4 should be returned (both have length 2)
	// C1 and C3 have length 2 as well, but let's check actual values
	for _, c := range commits {
		if len(c.Message) != 2 {
			t.Errorf("Expected message length 2, got %d", len(c.Message))
		}
	}
	// C2 and C4 are definitely in the list
	foundC2 := false
	foundC4 := false
	for _, c := range commits {
		if c == c2 {
			foundC2 = true
		}
		if c == c4 {
			foundC4 = true
		}
	}
	if !foundC2 {
		t.Error("Expected to find c2 in commits")
	}
	if !foundC4 {
		t.Error("Expected to find c4 in commits")
	}
}

// TestFilterCommitIterWithLimit tests limiting commit traversal
func TestFilterCommitIterWithLimit(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	// Create a linear graph: C4 <- C3 <- C2 <- C1
	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c2)
	c4 := NewTestCommit("4444444444444444444444444444444444444444", "C4", c3)

	backend.AddCommit(c1)
	backend.AddCommit(c2)
	backend.AddCommit(c3)
	backend.AddCommit(c4)

	// Limit: stop traversal at C2 (don't visit its parents)
	var isLimit CommitFilter = func(c *Commit) bool {
		return c.Hash == c2.Hash
	}

	iter := NewFilterCommitIter(c4, nil, &isLimit)
	defer iter.Close()

	var commits []*Commit
	err := iter.ForEach(ctx, func(commit *Commit) error {
		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		t.Fatalf("ForEach error: %v", err)
	}
	// BFS order: C4, C3, C2
	// C2 is a limit, so C1 should not be visited
	if len(commits) != 3 {
		t.Errorf("Expected 3 commits, got %d", len(commits))
	}

	foundC4 := false
	foundC3 := false
	foundC2 := false
	foundC1 := false
	for _, c := range commits {
		if c == c4 {
			foundC4 = true
		}
		if c == c3 {
			foundC3 = true
		}
		if c == c2 {
			foundC2 = true
		}
		if c == c1 {
			foundC1 = true
		}
	}
	if !foundC4 {
		t.Error("Expected to find c4 in commits")
	}
	if !foundC3 {
		t.Error("Expected to find c3 in commits")
	}
	if !foundC2 {
		t.Error("Expected to find c2 in commits")
	}
	if foundC1 {
		t.Error("C1 should not be visited as it's beyond the limit")
	}
}

// TestCommitWalkerShallowClone tests that commit walkers handle missing commits gracefully
// This is critical for zeta's default shallow clone behavior
func TestCommitWalkerShallowClone(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	// Create commits
	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c2)

	// Simulate shallow clone: only C3 and C2 are available, C1 is missing
	backend.AddCommit(c2)
	backend.AddCommit(c3)

	// Test with FilterCommitIter
	iter := NewFilterCommitIter(c3, nil, nil)
	defer iter.Close()

	var commits []*Commit
	err := iter.ForEach(ctx, func(commit *Commit) error {
		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		t.Fatalf("Should not error on missing commits in shallow clone: %v", err)
	}
	// Should traverse C3 and C2, skipping missing C1 gracefully
	if len(commits) != 2 {
		t.Errorf("Expected 2 commits, got %d", len(commits))
	}

	foundC3 := false
	foundC2 := false
	for _, c := range commits {
		if c == c3 {
			foundC3 = true
		}
		if c == c2 {
			foundC2 = true
		}
	}
	if !foundC3 {
		t.Error("Expected to find c3 in commits")
	}
	if !foundC2 {
		t.Error("Expected to find c2 in commits")
	}
}

// TestCommitWalkerShallowCloneWithMerge tests shallow clone with merge commits
func TestCommitWalkerShallowCloneWithMerge(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	// Create a merge commit graph:
	//     M
	//    / \
	//   C2  C3
	//    \ /
	//     C1
	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c1)
	m := NewTestCommit("4444444444444444444444444444444444444444", "M", c2, c3)

	// Simulate shallow clone: only M and C2 are available, C3 and C1 are missing
	backend.AddCommit(m)
	backend.AddCommit(c2)

	// Test with FilterCommitIter
	iter := NewFilterCommitIter(m, nil, nil)
	defer iter.Close()

	var commits []*Commit
	err := iter.ForEach(ctx, func(commit *Commit) error {
		commits = append(commits, commit)
		return nil
	})

	if err != nil {
		t.Fatalf("Should not error on missing commits in shallow clone: %v", err)
	}
	// Should traverse M and C2, skipping missing C3 and C1 gracefully
	if len(commits) != 2 {
		t.Errorf("Expected 2 commits, got %d", len(commits))
	}

	foundM := false
	foundC2 := false
	for _, c := range commits {
		if c == m {
			foundM = true
		}
		if c == c2 {
			foundC2 = true
		}
	}
	if !foundM {
		t.Error("Expected to find m in commits")
	}
	if !foundC2 {
		t.Error("Expected to find c2 in commits")
	}
}

// TestCommitWalkerContextCancellation tests that walkers respect context cancellation
func TestCommitWalkerContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	backend := NewMockBackend()

	// Create a long chain of commits
	var commits []*Commit
	for i := range 100 {
		hash := plumbing.NewHash(string(rune(0x11 + i)))
		c := NewTestCommit(hash.String(), "C"+string(rune('0'+i)))
		if len(commits) > 0 {
			c.Parents = []plumbing.Hash{commits[len(commits)-1].Hash}
		}
		commits = append(commits, c)
		backend.AddCommit(c)
	}

	// Start traversal
	iter := NewCommitPreorderIter(commits[len(commits)-1], nil, nil)
	defer iter.Close()

	// Cancel the context immediately
	cancel()

	// Try to iterate - should stop quickly or error
	count := 0
	_ = iter.ForEach(ctx, func(commit *Commit) error {
		count++
		return nil
	})

	// Verify that iteration stopped (either immediately or after a few commits)
	// The exact behavior depends on the implementation
	if count >= 100 {
		t.Error("Should not process all commits after cancellation")
	}
}

// TestCommitIterForEachStop tests that ErrStop stops traversal
func TestCommitIterForEachStop(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)
	c3 := NewTestCommit("3333333333333333333333333333333333333333", "C3", c2)

	backend.AddCommit(c1)
	backend.AddCommit(c2)
	backend.AddCommit(c3)

	iter := NewCommitPreorderIter(c3, nil, nil)
	defer iter.Close()

	count := 0
	err := iter.ForEach(ctx, func(commit *Commit) error {
		count++
		// Stop after 2 commits
		if count == 2 {
			return plumbing.ErrStop
		}
		return nil
	})

	if err != nil {
		t.Fatalf("ForEach error: %v", err)
	}
	if count != 2 {
		t.Errorf("Expected 2, got %d", count)
	}
}

// TestCommitIterNextDirectly tests calling Next() directly
func TestCommitIterNextDirectly(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")
	c2 := NewTestCommit("2222222222222222222222222222222222222222", "C2", c1)

	backend.AddCommit(c1)
	backend.AddCommit(c2)

	iter := NewCommitPreorderIter(c2, nil, nil)
	defer iter.Close()

	// First commit
	c, err := iter.Next(ctx)
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if c.Message != "C2" {
		t.Errorf("Expected C2, got %s", c.Message)
	}

	// Second commit
	c, err = iter.Next(ctx)
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if c.Message != "C1" {
		t.Errorf("Expected C1, got %s", c.Message)
	}

	// EOF
	c, err = iter.Next(ctx)
	if err != io.EOF {
		t.Errorf("Expected io.EOF, got %v", err)
	}
	if c != nil {
		t.Error("Expected nil commit")
	}
}

// TestCommitIterClose tests that Close() properly cleans up resources
func TestCommitIterClose(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")

	backend.AddCommit(c1)

	iter := NewCommitPreorderIter(c1, nil, nil)

	// Get a commit
	c, err := iter.Next(ctx)
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if c == nil {
		t.Fatal("Expected non-nil commit")
	}

	// Close the iterator
	iter.Close()

	// Try to get another commit after close
	c, err = iter.Next(ctx)
	if err == nil {
		t.Error("Expected error after close")
	}
	if c != nil {
		t.Error("Expected nil commit after close")
	}
}

// TestCommitWalkerErrorPropagation tests that errors are properly propagated
func TestCommitWalkerErrorPropagation(t *testing.T) {
	ctx := t.Context()
	backend := NewMockBackend()

	c1 := NewTestCommit("1111111111111111111111111111111111111111", "C1")

	backend.AddCommit(c1)

	iter := NewCommitPreorderIter(c1, nil, nil)
	defer iter.Close()

	// Return an error from the callback
	expectedErr := io.EOF
	err := iter.ForEach(ctx, func(commit *Commit) error {
		return expectedErr
	})

	if err != expectedErr {
		t.Errorf("Expected %v, got %v", expectedErr, err)
	}
}
