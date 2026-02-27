package diferenco

import (
	"context"
	"io"
	"sort"
	"strings"
)

// NewHasConflict checks if there are any conflicts when merging three texts.
// It uses the same logic as NewMerge but only checks for conflicts without
// generating the merged result, making it more efficient for conflict detection.
func NewHasConflict(ctx context.Context, textO, textA, textB string) (bool, error) {
	sink := NewSink(NEWLINE_LF)

	// Parse the texts into indices
	oIdx, err := sink.parseLines(nil, textO)
	if err != nil {
		return false, err
	}
	aIdx, err := sink.parseLines(nil, textA)
	if err != nil {
		return false, err
	}
	bIdx, err := sink.parseLines(nil, textB)
	if err != nil {
		return false, err
	}

	// Step 1: Calculate diffs (O→A and O→B) using Histogram algorithm
	changesA, err := diffInternal(ctx, oIdx, aIdx, Histogram)
	if err != nil {
		return false, err
	}
	changesB, err := diffInternal(ctx, oIdx, bIdx, Histogram)
	if err != nil {
		return false, err
	}

	// Step 2: Find merge regions and check for conflicts
	regions := findMergeRegions(changesA, changesB, sink, aIdx, bIdx)

	// Step 3: Check if any region has a conflict
	for _, region := range regions {
		if region.isConflict {
			return true, nil
		}
	}

	return false, nil
}

// NewMerge performs a three-way merge based on Diff3 paper principles.
// It uses a cleaner, more modern Go 1.26+ implementation.
func NewMerge(ctx context.Context, opts *MergeOptions) (string, bool, error) {
	if err := opts.ValidateOptions(); err != nil {
		return "", false, err
	}

	sink := NewSink(NEWLINE_LF)
	oIdx, err := sink.parseLines(opts.RO, opts.TextO)
	if err != nil {
		return "", false, err
	}
	aIdx, err := sink.parseLines(opts.R1, opts.TextA)
	if err != nil {
		return "", false, err
	}
	bIdx, err := sink.parseLines(opts.R2, opts.TextB)
	if err != nil {
		return "", false, err
	}

	var builder strings.Builder
	result, err := newMergeInternal(ctx, sink, &builder, oIdx, aIdx, bIdx, opts)
	if err != nil {
		return "", false, err
	}

	return builder.String(), result.hasConflict, nil
}

// newMergeResult contains the merge result
type newMergeResult struct {
	hasConflict bool
}

// newMergeInternal performs the core three-way merge logic
func newMergeInternal(
	ctx context.Context,
	sink *Sink,
	out io.Writer,
	oIdx, aIdx, bIdx []int,
	opts *MergeOptions,
) (*newMergeResult, error) {
	result := &newMergeResult{}

	// Step 1: Calculate diffs (O→A and O→B)
	changesA, err := diffInternal(ctx, oIdx, aIdx, opts.A)
	if err != nil {
		return nil, err
	}
	changesB, err := diffInternal(ctx, oIdx, bIdx, opts.A)
	if err != nil {
		return nil, err
	}

	// Step 2: Find merge regions (groups of overlapping changes)
	regions := findMergeRegions(changesA, changesB, sink, aIdx, bIdx)

	// Step 3: Process each region
	pos := 0
	for _, region := range regions {
		// Write unchanged content before this region
		if pos < region.start {
			writeOriginLines(sink, out, oIdx, pos, region.start)
		}

		// Process the region
		if region.isConflict {
			result.hasConflict = true
			writeConflictRegion(sink, out, oIdx, aIdx, bIdx, region, opts)
		} else {
			writeNonConflictRegion(sink, out, aIdx, bIdx, region)
		}

		pos = region.end
	}

	// Write remaining unchanged content
	if pos < len(oIdx) {
		writeOriginLines(sink, out, oIdx, pos, len(oIdx))
	}

	return result, nil
}

// mergeRegion represents a group of changes that overlap in the original
type mergeRegion struct {
	start, end int // Range in O (original)
	isConflict bool
	changesA   []Change // Changes from O→A
	changesB   []Change // Changes from O→B
}

// findMergeRegions groups overlapping changes into regions
func findMergeRegions(changesA, changesB []Change, sink *Sink, aIdx, bIdx []int) []mergeRegion {
	var regions []mergeRegion

	// Combine all changes with their source
	allChanges := make([]struct {
		change Change
		side   int // 0 = A, 1 = B
	}, 0, len(changesA)+len(changesB))

	for _, ch := range changesA {
		allChanges = append(allChanges, struct {
			change Change
			side   int
		}{ch, 0})
	}
	for _, ch := range changesB {
		allChanges = append(allChanges, struct {
			change Change
			side   int
		}{ch, 1})
	}

	// Sort by position in O
	sort.Slice(allChanges, func(i, j int) bool {
		return allChanges[i].change.P1 < allChanges[j].change.P1
	})

	// Group overlapping changes
	if len(allChanges) == 0 {
		return regions
	}

	currentRegion := mergeRegion{
		start: allChanges[0].change.P1,
		end:   allChanges[0].change.P1 + allChanges[0].change.Del,
	}

	for _, item := range allChanges {
		ch := item.change
		regionEnd := ch.P1 + ch.Del

		// Check if this change overlaps with current region
		if ch.P1 <= currentRegion.end {
			// Overlaps, extend region if needed
			if regionEnd > currentRegion.end {
				currentRegion.end = regionEnd
			}
			if item.side == 0 {
				currentRegion.changesA = append(currentRegion.changesA, ch)
			} else {
				currentRegion.changesB = append(currentRegion.changesB, ch)
			}
		} else {
			// No overlap, finalize current region
			regions = append(regions, finalizeRegion(currentRegion, sink, aIdx, bIdx))

			// Start new region
			currentRegion = mergeRegion{
				start: ch.P1,
				end:   regionEnd,
			}
			if item.side == 0 {
				currentRegion.changesA = append(currentRegion.changesA, ch)
			} else {
				currentRegion.changesB = append(currentRegion.changesB, ch)
			}
		}
	}

	// Add the last region
	if len(currentRegion.changesA) > 0 || len(currentRegion.changesB) > 0 {
		regions = append(regions, finalizeRegion(currentRegion, sink, aIdx, bIdx))
	}

	return regions
}

// finalizeRegion determines if a region is a conflict and finalizes it
func finalizeRegion(region mergeRegion, sink *Sink, aIdx, bIdx []int) mergeRegion {
	// Region is a conflict if both A and B have changes
	region.isConflict = len(region.changesA) > 0 && len(region.changesB) > 0

	// Check for false conflict (same content on both sides)
	if region.isConflict {
		if isFalseConflict(region, sink, aIdx, bIdx) {
			region.isConflict = false
		}
	}

	return region
}

// isFalseConflict checks if A and B made the same change
func isFalseConflict(region mergeRegion, sink *Sink, aIdx, bIdx []int) bool {
	if len(region.changesA) != 1 || len(region.changesB) != 1 {
		return false
	}

	chA := region.changesA[0]
	chB := region.changesB[0]

	// Check if both delete the same range
	if chA.P1 != chB.P1 || chA.Del != chB.Del {
		return false
	}

	// Check if both insert the same content
	if chA.Ins != chB.Ins {
		return false
	}

	// Compare the inserted content
	if chA.Ins > 0 {
		contentA := getChangeContent(sink, aIdx, chA)
		contentB := getChangeContent(sink, bIdx, chB)
		if !contentEquals(contentA, contentB) {
			return false
		}
	}

	// Same operation and same content
	return true
}

// getChangeContent returns the content of a change
func getChangeContent(sink *Sink, idx []int, ch Change) []string {
	if ch.Ins == 0 {
		return nil
	}
	content := make([]string, ch.Ins)
	for i := 0; i < ch.Ins; i++ {
		content[i] = sink.Lines[idx[ch.P2+i]]
	}
	return content
}

// contentEquals compares two string slices
func contentEquals(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// writeOriginLines writes unchanged lines from O
func writeOriginLines(sink *Sink, out io.Writer, oIdx []int, start, end int) {
	sink.WriteLine(out, oIdx[start:end]...)
}

// writeNonConflictRegion writes a region without conflicts
func writeNonConflictRegion(sink *Sink, out io.Writer, aIdx, bIdx []int, region mergeRegion) {
	// Prefer A's changes if available, otherwise B
	if len(region.changesA) > 0 {
		writeChanges(sink, out, aIdx, region.changesA)
	} else if len(region.changesB) > 0 {
		writeChanges(sink, out, bIdx, region.changesB)
	}
}

// writeChanges writes a list of changes to output
func writeChanges(sink *Sink, out io.Writer, idx []int, changes []Change) {
	for _, ch := range changes {
		// Write inserted content
		if ch.Ins > 0 {
			sink.WriteLine(out, idx[ch.P2:ch.P2+ch.Ins]...)
		}
	}
}

// writeConflictRegion writes a region with conflicts
func writeConflictRegion(
	sink *Sink,
	out io.Writer,
	oIdx, aIdx, bIdx []int,
	region mergeRegion,
	opts *MergeOptions,
) {
	// Calculate A, O, B ranges for this region
	aLhs, aRhs := calculateRange(region.changesA, aIdx, region.start, region.end)
	oLhs, oRhs := region.start, region.end
	bLhs, bRhs := calculateRange(region.changesB, bIdx, region.start, region.end)

	conflict := &Conflict[int]{
		a: aIdx[aLhs:aRhs],
		o: oIdx[oLhs:oRhs],
		b: bIdx[bLhs:bRhs],
	}

	sink.writeConflict(out, opts, conflict)
}

// calculateRange calculates the content range for a set of changes
func calculateRange(changes []Change, idx []int, regionStart, regionEnd int) (lhs, rhs int) {
	if len(changes) == 0 {
		return regionStart, regionEnd
	}

	// Initialize with extreme values to find min/max
	abLhs := len(idx)
	abRhs := -1
	oLhs := regionEnd
	oRhs := regionStart

	for _, ch := range changes {
		// Track origin range (oLhs, oRhs)
		if ch.P1 < oLhs {
			oLhs = ch.P1
		}
		originEnd := ch.P1 + ch.Del
		if originEnd > oRhs {
			oRhs = originEnd
		}

		// Track content range (abLhs, abRhs)
		if ch.P2 < abLhs {
			abLhs = ch.P2
		}
		contentEnd := ch.P2 + ch.Ins
		if contentEnd > abRhs {
			abRhs = contentEnd
		}
	}

	// Apply offset formula
	lhs = abLhs + (regionStart - oLhs)
	rhs = abRhs + (regionEnd - oRhs)

	// Ensure bounds are valid
	if lhs < 0 {
		lhs = 0
	}
	if rhs > len(idx) {
		rhs = len(idx)
	}
	if lhs > rhs {
		lhs = rhs
	}

	return
}
