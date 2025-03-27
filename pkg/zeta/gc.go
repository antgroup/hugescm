// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/pkg/progress"
	"github.com/antgroup/hugescm/pkg/tr"
)

type GcOptions struct {
	Prune time.Duration
}

func (r *Repository) Gc(ctx context.Context, opts *GcOptions) error {
	if err := r.Packed(); err != nil {
		fmt.Fprintf(os.Stderr, "packed refs error: %v\n", err)
		return err
	}
	packOpts := &backend.PackOptions{
		ZetaDir:         r.zetaDir,
		SharingRoot:     r.Core.SharingRoot,
		Quiet:           r.quiet,
		CompressionALGO: r.Core.CompressionALGO,
	}
	if !r.quiet {
		packOpts.Logger = func(format string, a ...any) {
			_, _ = tr.Fprintf(os.Stderr, format, a...)
		}
		packOpts.NewIndicators = func(description, completed string, total uint64, quiet bool) backend.Indicators {
			return progress.NewIndicators(description, completed, total, quiet)
		}
	}
	if err := backend.PackObjects(ctx, packOpts); err != nil {
		fmt.Fprintf(os.Stderr, "pack-objects error: %v\n", err)
		return err
	}
	return nil
}
