package merkletrie

import (
	"context"
	"fmt"
	"io"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
)

// A doubleIter is a convenience type to keep track of the current
// noders in two merkletrie that are going to be iterated in parallel.
// It has methods for:
//
// - iterating over the merkletrie, both at the same time or
// individually: nextFrom, nextTo, nextBoth, stepBoth
//
// - checking if there are noders left in one or both of them with the
// remaining method and its associated returned type.
//
// - comparing the current noders of both merkletrie in several ways,
// with the compare method and its associated returned type.
type doubleIter struct {
	from struct {
		iter    *Iter
		current noder.Path // nil if no more nodes
	}
	to struct {
		iter    *Iter
		current noder.Path // nil if no more nodes
	}
	hashEqual noder.Equal
}

// NewdoubleIter returns a new doubleIter for the merkletrie "from" and
// "to".  The hashEqual callback function will be used by the doubleIter
// to compare the hash of the noders in the merkletrie.  The doubleIter
// will be initialized to the first elements in each merkletrie if any.
func newDoubleIter(ctx context.Context, from, to noder.Noder, hashEqual noder.Equal) (
	*doubleIter, error) {
	var ii doubleIter
	var err error

	if ii.from.iter, err = NewIter(ctx, from); err != nil {
		return nil, fmt.Errorf("from: %w", err)
	}
	if ii.from.current, err = ii.from.iter.Next(ctx); turnEOFIntoNil(err) != nil {
		return nil, fmt.Errorf("from: %w", err)
	}

	if ii.to.iter, err = NewIter(ctx, to); err != nil {
		return nil, fmt.Errorf("to: %w", err)
	}
	if ii.to.current, err = ii.to.iter.Next(ctx); turnEOFIntoNil(err) != nil {
		return nil, fmt.Errorf("to: %w", err)
	}

	ii.hashEqual = hashEqual

	return &ii, nil
}

func turnEOFIntoNil(e error) error {
	if e != nil && e != io.EOF {
		return e
	}
	return nil
}

// NextBoth makes d advance to the next noder in both merkletries.  If
// any of them is a directory, it skips its contents.
func (d *doubleIter) nextBoth(ctx context.Context) error {
	if err := d.nextFrom(ctx); err != nil {
		return err
	}
	if err := d.nextTo(ctx); err != nil {
		return err
	}

	return nil
}

// NextFrom makes d advance to the next noder in the "from" merkletrie,
// skipping its contents if it is a directory.
func (d *doubleIter) nextFrom(ctx context.Context) (err error) {
	d.from.current, err = d.from.iter.Next(ctx)
	return turnEOFIntoNil(err)
}

// NextTo makes d advance to the next noder in the "to" merkletrie,
// skipping its contents if it is a directory.
func (d *doubleIter) nextTo(ctx context.Context) (err error) {
	d.to.current, err = d.to.iter.Next(ctx)
	return turnEOFIntoNil(err)
}

// StepBoth makes d advance to the next noder in both merkletries,
// getting deeper into directories if that is the case.
func (d *doubleIter) stepBoth(ctx context.Context) (err error) {
	if d.from.current, err = d.from.iter.Step(ctx); turnEOFIntoNil(err) != nil {
		return err
	}
	if d.to.current, err = d.to.iter.Step(ctx); turnEOFIntoNil(err) != nil {
		return err
	}
	return nil
}

// Remaining returns if there are no more noders in the tree, if both
// have noders or if one of them doesn't.
func (d *doubleIter) remaining() remaining {
	if d.from.current == nil && d.to.current == nil {
		return noMoreNoders
	}

	if d.from.current == nil && d.to.current != nil {
		return onlyToRemains
	}

	if d.from.current != nil && d.to.current == nil {
		return onlyFromRemains
	}

	return bothHaveNodes
}

// Remaining values tells you whether both trees still have noders, or
// only one of them or none of them.
type remaining int

const (
	noMoreNoders remaining = iota
	onlyToRemains
	onlyFromRemains
	bothHaveNodes
)

func (d *doubleIter) sameHash() bool {
	from := d.from.current.Last()
	to := d.to.current.Last()
	a, fromOK := from.(noder.Comparators)
	b, toOK := to.(noder.Comparators)
	if fromOK && toOK {
		if a.Mode() == b.Mode() && a.ModifiedAt().Equal(b.ModifiedAt()) {
			return true
		}
	}
	return d.hashEqual(d.from.current, d.to.current)
}

// Compare returns the comparison between the current elements in the
// merkletries.
func (d *doubleIter) compare(ctx context.Context) (s comparison, err error) {
	s.sameHash = d.sameHash()

	fromIsDir := d.from.current.IsDir()
	toIsDir := d.to.current.IsDir()

	s.bothAreDirs = fromIsDir && toIsDir
	s.bothAreFiles = !fromIsDir && !toIsDir
	s.fileAndDir = !s.bothAreDirs && !s.bothAreFiles

	fromNumChildren, err := d.from.current.NumChildren(ctx)
	if err != nil {
		return comparison{}, fmt.Errorf("from: %w", err)
	}

	toNumChildren, err := d.to.current.NumChildren(ctx)
	if err != nil {
		return comparison{}, fmt.Errorf("to: %w", err)
	}

	s.fromIsEmptyDir = fromIsDir && fromNumChildren == 0
	s.toIsEmptyDir = toIsDir && toNumChildren == 0

	return
}

// Answers to a lot of questions you can ask about how to noders are
// equal or different.
type comparison struct {
	// the following are only valid if both nodes have the same name
	// (i.e. nameComparison == 0)

	// Do both nodes have the same hash?
	sameHash bool
	// Are both nodes files?
	bothAreFiles bool

	// the following are only valid if any of the noders are dirs,
	// this is, if !bothAreFiles

	// Is one a file and the other a dir?
	fileAndDir bool
	// Are both nodes dirs?
	bothAreDirs bool
	// Is the from node an empty dir?
	fromIsEmptyDir bool
	// Is the to Node an empty dir?
	toIsEmptyDir bool
}
