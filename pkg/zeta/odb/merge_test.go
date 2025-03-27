package odb

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/plumbing"
)

func TestMerge0(t *testing.T) {
	odb, err := NewODB("/private/tmp/b2/.zeta")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open odb error: %v\n", err)
		return
	}
	defer odb.Close() // nolint
	o, err := odb.Tree(t.Context(), plumbing.NewHash("dcfe2d5aaa20344a565da7724516700a761c7695285b47ed2e097e44eb6c7b55"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new o %v\n", err)
		return
	}
	a, err := odb.Tree(t.Context(), plumbing.NewHash("9c3c905c4d8b1c6c4990beb3a56184a2b325a780d9e199a08cb8aa8440822dee"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new a %v\n", err)
		return
	}
	b, err := odb.Tree(t.Context(), plumbing.NewHash("d736c423e7a4d726f6fefe0e382d4cfd4f16b31f3b2aaca2732a87437c9d65bf"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new b %v\n", err)
		return
	}
	d, err := odb.mergeDifferences(t.Context(), o, a, b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare %v\n", err)
		return
	}
	for p, e := range d.entries {
		fmt.Fprintf(os.Stderr, "%s conflict: %v\n", p, e.hasConflict())
	}
	for p, e := range d.renames {
		fmt.Fprintf(os.Stderr, "%s conflict: %v\n", p, e.conflict())
	}
	result, err := odb.MergeTree(t.Context(), o, a, b, &MergeOptions{Textconv: true})
	if err != nil {
		fmt.Fprintf(os.Stderr, "merge tree: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\n", result.NewTree)
	for _, e := range result.Conflicts {
		if e.Ancestor.Path != "" {
			fmt.Fprintf(os.Stderr, "%s %s 1 %s\n", e.Ancestor.Mode, e.Ancestor.Hash, e.Ancestor.Path)
		}
		if e.Our.Path != "" {
			fmt.Fprintf(os.Stderr, "%s %s 2 %s\n", e.Our.Mode, e.Our.Hash, e.Our.Path)
		}
		if e.Their.Path != "" {
			fmt.Fprintf(os.Stderr, "%s %s 3 %s\n", e.Their.Mode, e.Their.Hash, e.Their.Path)
		}
	}
}

func TestMerge1(t *testing.T) {
	odb, err := NewODB("/private/tmp/b2/.zeta")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open odb error: %v\n", err)
		return
	}
	defer odb.Close() // nolint
	o, err := odb.Tree(t.Context(), plumbing.NewHash("dcfe2d5aaa20344a565da7724516700a761c7695285b47ed2e097e44eb6c7b55"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new o %v\n", err)
		return
	}
	a, err := odb.Tree(t.Context(), plumbing.NewHash("117c2199f51dd4e9bda78cab847d59f58fb46f7adc7c3b52dbe95b2916747814"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new a %v\n", err)
		return
	}
	b, err := odb.Tree(t.Context(), plumbing.NewHash("399ad3c84a2d386b3bb58c6875f4b9358eda3809ac5a4c477a7eeab010d7ff38"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new b %v\n", err)
		return
	}
	d, err := odb.mergeDifferences(t.Context(), o, a, b)
	if err != nil {
		fmt.Fprintf(os.Stderr, "compare %v\n", err)
		return
	}
	for p, e := range d.entries {
		fmt.Fprintf(os.Stderr, "%s conflict: %v\n", p, e.hasConflict())
	}
	for p, e := range d.renames {
		fmt.Fprintf(os.Stderr, "%s conflict: %v\n", p, e.conflict())
	}
	// conflicts := d.analyzeNameConflicts()
	// d.replace(conflicts, "branch-2")
	result, err := odb.MergeTree(t.Context(), o, a, b, &MergeOptions{Textconv: true, Branch1: "dev-2"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "merge tree: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\n", result.NewTree)
	for _, e := range result.Conflicts {
		if e.Ancestor.Path != "" {
			fmt.Fprintf(os.Stderr, "%s %s 1 %s\n", e.Ancestor.Mode, e.Ancestor.Hash, e.Ancestor.Path)
		}
		if e.Our.Path != "" {
			fmt.Fprintf(os.Stderr, "%s %s 2 %s\n", e.Our.Mode, e.Our.Hash, e.Our.Path)
		}
		if e.Their.Path != "" {
			fmt.Fprintf(os.Stderr, "%s %s 3 %s\n", e.Their.Mode, e.Their.Hash, e.Their.Path)
		}
	}
}

func TestMerge3(t *testing.T) {
	odb, err := NewODB("/private/tmp/b2/.zeta")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open odb error: %v\n", err)
		return
	}
	defer odb.Close() // nolint
	o, err := odb.Tree(t.Context(), plumbing.NewHash("dcfe2d5aaa20344a565da7724516700a761c7695285b47ed2e097e44eb6c7b55"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new o %v\n", err)
		return
	}
	a, err := odb.Tree(t.Context(), plumbing.NewHash("117c2199f51dd4e9bda78cab847d59f58fb46f7adc7c3b52dbe95b2916747814"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new a %v\n", err)
		return
	}
	b, err := odb.Tree(t.Context(), plumbing.NewHash("399ad3c84a2d386b3bb58c6875f4b9358eda3809ac5a4c477a7eeab010d7ff38"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "new b %v\n", err)
		return
	}
	result, err := odb.MergeTree(t.Context(), o, a, b, &MergeOptions{Textconv: true, Branch1: "dev-2"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "merge tree: %v\n", err)
		return
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	if err := enc.Encode(result); err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
	}
}

func TestMergeText(t *testing.T) {
	const textO = `celery
garlic
onions
salmon
tomatoes
wine
`

	const textA = `celery
salmon
tomatoes
garlic
onions
wine
`

	const textB = `celery
garlic
salmon
tomatoes
onions
wine
`
	s, conflict, err := diferenco.DefaultMerge(t.Context(), textO, textA, textB, "a.txt", "a.txt", "b.txt")
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "conflict: %v\n%s\n", conflict, s)
}

func TestCreateMergeFile(t *testing.T) {
	d := &ODB{
		root: "/tmp/merge-root",
	}
	p, err := d.writeMergeFileToTemp("####")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mergefile error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "write to merge file: %s\n", p)
}

func TestExternalMerge(t *testing.T) {
	const textO = `celery
garlic
onions
salmon
tomatoes
wine
`

	const textA = `celery
salmon
tomatoes
garlic
onions
wine
`

	const textB = `celery
garlic
salmon
tomatoes
onions
wine
`
	d := &ODB{root: "/tmp/git-merge-file"}
	s, conflict, err := d.ExternalMerge(t.Context(), textO, textA, textB, "a.txt", "a.txt", "b.txt")
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "conflict: %v\n\n\n%s\n", conflict, s)
}

func TestDiff3Merge(t *testing.T) {
	const textO = `celery
garlic
onions
salmon
tomatoes
wine
`

	const textA = `celery
salmon
tomatoes
garlic
onions
wine
`

	const textB = `celery
garlic
salmon
tomatoes
onions
wine
`
	d := &ODB{root: "/tmp/diff3-merge"}
	s, conflict, err := d.Diff3Merge(t.Context(), textO, textA, textB, "a.txt", "a.txt", "b.txt")
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "conflict: %v\n\n\n%s\n", conflict, s)
}
