package command

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type MergeFile struct {
	Stdout        bool     `name:"stdout" short:"p" negatable:"" help:"Send results to standard output"`
	ObjectID      bool     `name:"object-id" negatable:"" help:"Use object IDs instead of filenames"`
	Diff3         bool     `name:"diff3" negatable:"" help:"Use a diff3 based merge"`
	ZDiff3        bool     `name:"zdiff3" negatable:"" help:"Use a zealous diff3 based merge"`
	DiffAlgorithm string   `name:"diff-algorithm" help:"Choose a diff algorithm, supported: histogram|onp|myers|patience|minimal"`
	L             []string `name:":L" short:"L" help:"Set labels for file1/orig-file/file2"`
	F1            string   `arg:"" name:"file1" help:""`
	O             string   `arg:"" name:"orig-file" help:""`
	F2            string   `arg:"" name:"file2" help:""`
}

const (
	mergeFileSummaryFormat = `%szeta merge-file [<options>] [-L <name1> [-L <orig> [-L <name2>]]] <file1> <orig-file> <file2>`
)

func (c *MergeFile) Summary() string {
	return fmt.Sprintf(mergeFileSummaryFormat, W("Usage: "))
}

func (c *MergeFile) labelName(i int, n string) string {
	if i < len(c.L) {
		return c.L[i]
	}
	return n
}

func (c *MergeFile) mergeExtra(g *Globals) error {
	var a diferenco.Algorithm
	var err error
	if len(c.DiffAlgorithm) != 0 {
		if a, err = diferenco.AlgorithmFromName(c.DiffAlgorithm); err != nil {
			fmt.Fprintf(os.Stderr, "parse diff.algorithm error: %v\n", err)
			return err
		}
	}
	var style int
	switch {
	case c.Diff3:
		style = diferenco.STYLE_DIFF3
	case c.ZDiff3:
		style = diferenco.STYLE_ZEALOUS_DIFF3
	}
	g.DbgPrint("algorithm: %s conflict style: %v", a, style)
	textO, err := zeta.ReadText(c.O, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "merge-file: open <orig-file> error: %v\n", err)
		return err
	}
	textA, err := zeta.ReadText(c.F1, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "merge-file: open <file1> error: %v\n", err)
		return err
	}
	textB, err := zeta.ReadText(c.F2, false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "merge-file: open <file2> error: %v\n", err)
		return err
	}
	opts := &diferenco.MergeOptions{
		TextO:  textO,
		TextA:  textA,
		TextB:  textB,
		A:      a,
		Style:  style,
		LabelA: c.labelName(0, c.F1),
		LabelO: c.labelName(1, c.O),
		LabelB: c.labelName(2, c.F2),
	}
	mergedText, conflict, err := diferenco.Merge(context.Background(), opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "merge-file: merge error: %v\n", err)
		return err
	}
	_, _ = io.WriteString(os.Stdout, mergedText)
	if conflict {
		return &zeta.ErrExitCode{ExitCode: 1, Message: "conflict"}
	}
	return nil
}

func (c *MergeFile) Run(g *Globals) error {
	if !c.ObjectID {
		return c.mergeExtra(g)
	}
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close()
	var style int
	switch {
	case c.Diff3:
		style = diferenco.STYLE_DIFF3
	case c.ZDiff3:
		style = diferenco.STYLE_ZEALOUS_DIFF3
	}
	opts := &zeta.MergeFileOptions{
		O: c.O, A: c.F1, B: c.F2,
		Style:         style,
		DiffAlgorithm: c.DiffAlgorithm,
		Stdout:        c.Stdout,
		LabelA:        c.labelName(0, c.F1),
		LabelO:        c.labelName(1, c.O),
		LabelB:        c.labelName(2, c.F2),
	}
	if err := r.MergeFile(context.Background(), opts); err != nil {
		if !zeta.IsExitCode(err, 1) {
			diev("merge-file: error: %v", err)
		}
		return err
	}
	return nil
}
