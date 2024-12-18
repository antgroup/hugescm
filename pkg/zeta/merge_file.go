package zeta

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/pkg/zeta/odb"
)

func (r *Repository) resolveMergeDriver() odb.MergeDriver {
	if driverName, ok := os.LookupEnv(ENV_ZETA_MERGE_TEXT_DRIVER); ok {
		switch driverName {
		case "git":
			if _, err := exec.LookPath("git"); err == nil {
				r.DbgPrint("Use git merge-file as text merge driver")
				return r.odb.ExternalMerge
			}
		case "diff3":
			if _, err := exec.LookPath("diff3"); err == nil {
				r.DbgPrint("Use diff3 as text merge driver")
				return r.odb.Diff3Merge
			}
		default:
			r.DbgPrint("Unsupport merge driver '%s'", driverName)
		}
	}
	var diffAlgorithm diferenco.Algorithm
	var err error
	if len(r.Diff.Algorithm) != 0 {
		if diffAlgorithm, err = diferenco.AlgorithmFromName(r.Diff.Algorithm); err != nil {
			warn("diff: bad config: diff.algorithm value: %s", r.Diff.Algorithm)
		}
	}
	mergeConflictStyle := diferenco.ParseConflictStyle(r.Merge.ConflictStyle)
	return func(ctx context.Context, o, a, b, labelO, labelA, labelB string) (string, bool, error) {
		return diferenco.Merge(ctx, &diferenco.MergeOptions{
			TextO:  o,
			TextA:  a,
			TextB:  b,
			LabelO: labelO,
			LabelA: labelA,
			LabelB: labelB,
			A:      diffAlgorithm,
			Style:  mergeConflictStyle,
		})
	}
}

type MergeFileOptions struct {
	O, A, B                string
	LabelO, LabelA, LabelB string
	Style                  int
	DiffAlgorithm          string
	Stdout                 bool
	TextConv               bool
}

func (opts *MergeFileOptions) diffAlgorithmFromName(defaultDiffAlgorithm string) diferenco.Algorithm {
	if len(opts.DiffAlgorithm) != 0 {
		if diffAlgorithm, err := diferenco.AlgorithmFromName(opts.DiffAlgorithm); err == nil {
			return diffAlgorithm
		}
		warn("diff: bad --diff-algorithm value: %s", opts.DiffAlgorithm)
	}
	if len(defaultDiffAlgorithm) != 0 {
		if diffAlgorithm, err := diferenco.AlgorithmFromName(defaultDiffAlgorithm); err == nil {
			return diffAlgorithm
		}
		warn("diff: bad config: diff.algorithm value: %s", defaultDiffAlgorithm)
	}
	return diferenco.Unspecified
}

func (r *Repository) MergeFile(ctx context.Context, opts *MergeFileOptions) error {
	diffAlgorithm := opts.diffAlgorithmFromName(r.Diff.Algorithm)
	r.DbgPrint("algorithm: %s conflict style: %v", diffAlgorithm, opts.Style)
	o, err := r.Revision(ctx, opts.O)
	if err != nil {
		return err
	}
	textO, _, err := r.readMissingText(ctx, o, false)
	if err != nil {
		return err
	}
	a, err := r.Revision(ctx, opts.A)
	if err != nil {
		return err
	}
	textA, _, err := r.readMissingText(ctx, a, false)
	if err != nil {
		return err
	}
	b, err := r.Revision(ctx, opts.B)
	if err != nil {
		return err
	}
	textB, _, err := r.readMissingText(ctx, b, false)
	if err != nil {
		return err
	}

	merged, conflict, err := diferenco.Merge(ctx, &diferenco.MergeOptions{
		TextO:  textO,
		TextA:  textA,
		TextB:  textB,
		LabelO: opts.LabelO,
		LabelA: opts.LabelA,
		LabelB: opts.LabelB,
		A:      diffAlgorithm,
		Style:  opts.Style,
	})
	if err != nil {
		return err
	}
	if opts.Stdout {
		_, _ = io.WriteString(os.Stdout, merged)
	} else {
		oid, err := r.odb.HashTo(ctx, strings.NewReader(merged), int64(len(merged)))
		if err != nil {
			return err
		}
		_, _ = fmt.Fprintln(os.Stdout, oid.String())
	}
	if conflict {
		return &ErrExitCode{ExitCode: 1, Message: "conflict"}
	}
	return nil
}
