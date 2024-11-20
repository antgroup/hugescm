package zeta

import (
	"context"
	"errors"
)

type CherryPickOptions struct {
	From     string // From commit
	FF       bool
	Abort    bool
	Skip     bool
	Continue bool
}

func (r *Repository) CherryPick(ctx context.Context, opts *CherryPickOptions) error {

	return errors.New("unimplemented")
}
