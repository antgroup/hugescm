package diferenco

import (
	"context"

	"github.com/antgroup/hugescm/modules/diferenco/lcs"
)

// MinimalDiff: Myers: An O(ND) Difference Algorithm and Its Variations
func MinimalDiff[E comparable](ctx context.Context, L1 []E, L2 []E) ([]Change, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	diffs := lcs.DiffSlices(L1, L2)
	changes := make([]Change, 0, len(diffs))
	for _, d := range diffs {
		changes = append(changes, Change{P1: d.Start, P2: d.ReplStart, Del: d.End - d.Start, Ins: d.ReplEnd - d.ReplStart})
	}
	return changes, nil
}
