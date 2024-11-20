package zeta

import (
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func TestResolveAncestor(t *testing.T) {
	ss := []string{
		"master^^^",
		"master^^---",
		"master~12",
		"master",
	}
	for _, s := range ss {
		rev, a, err := resolveAncestor(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad: %s %v\n", s, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "good: %s %s %d\n", s, rev, a)
	}
}

func TestNewOptionsEmpty(t *testing.T) {
	opts := &NewOptions{}

	// In particular, when the commit is not empty,
	// we need to get the commit of the mainline to ensure that our expectations are correct,
	// that is, to create a new branch based on the commit.
	refname := plumbing.HEAD
	switch {
	case len(opts.Commit) != 0:
		// NO
	case len(opts.Branch) != 0:
		refname = plumbing.NewBranchReferenceName(opts.Branch)
	default:
	}
	fmt.Fprintf(os.Stderr, "refname: %s", refname)
}

func TestNewOptionsBranch(t *testing.T) {
	opts := &NewOptions{
		Branch: "mainline",
	}

	// In particular, when the commit is not empty,
	// we need to get the commit of the mainline to ensure that our expectations are correct,
	// that is, to create a new branch based on the commit.
	refname := plumbing.HEAD
	switch {
	case len(opts.Commit) != 0:
		// NO
	case len(opts.Branch) != 0:
		refname = plumbing.NewBranchReferenceName(opts.Branch)
	default:
	}
	fmt.Fprintf(os.Stderr, "refname: %s", refname)
}

func TestNewOptionsCommit(t *testing.T) {
	opts := &NewOptions{
		Commit: "01060407d8a2b9fda2527dfe00995d0c6cb28bcefeede2b2eec768747caadbf5",
	}

	// In particular, when the commit is not empty,
	// we need to get the commit of the mainline to ensure that our expectations are correct,
	// that is, to create a new branch based on the commit.
	refname := plumbing.HEAD
	switch {
	case len(opts.Commit) != 0:
		// NO
	case len(opts.Branch) != 0:
		refname = plumbing.NewBranchReferenceName(opts.Branch)
	default:
	}
	fmt.Fprintf(os.Stderr, "refname: %s", refname)
}

func TestNewOptionsBoth(t *testing.T) {
	opts := &NewOptions{
		Branch: "mainline",
		Commit: "01060407d8a2b9fda2527dfe00995d0c6cb28bcefeede2b2eec768747caadbf5",
	}

	// In particular, when the commit is not empty,
	// we need to get the commit of the mainline to ensure that our expectations are correct,
	// that is, to create a new branch based on the commit.
	refname := plumbing.HEAD
	switch {
	case len(opts.Commit) != 0:
		// NO
	case len(opts.Branch) != 0:
		refname = plumbing.NewBranchReferenceName(opts.Branch)
	default:
	}
	fmt.Fprintf(os.Stderr, "refname: %s", refname)
}

func TestParseReflogRev(t *testing.T) {
	ss := []string{
		"",
		"sss",
		"stash@",
		"stash@{",
		"stash@{}",
		"stash@{0}",
		"stash@{1}",
		"stash@{abc}",
		"stash@{abc}ddd",
	}
	for _, s := range ss {
		n, d, err := parseReflogRev(s)
		if err != nil {
			fmt.Fprintf(os.Stderr, "BAD: [%s] error: %v\n", s, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "GOOD: [%s %d]\n", n, d)
	}
}
