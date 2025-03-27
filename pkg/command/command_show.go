package command

import (
	"context"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/pkg/zeta"
)

// merge commit: only show commit metadata
// commit: show commit metadata and diff
// tree: tree path
//   tree list
// blob: blob content

// Show various types of objects
type Show struct {
	Textconv      bool     `name:"textconv" help:"Converting text to Unicode"`
	Histogram     bool     `name:"histogram" help:"Generate a diff using the \"Histogram diff\" algorithm"`
	ONP           bool     `name:"onp" help:"Generate a diff using the \"O(NP) diff\" algorithm"`
	Myers         bool     `name:"myers" help:"Generate a diff using the \"Myers diff\" algorithm"`
	Patience      bool     `name:"patience" help:"Generate a diff using the \"Patience diff\" algorithm"`
	Minimal       bool     `name:"minimal" help:"Spend extra time to make sure the smallest possible diff is produced"`
	DiffAlgorithm string   `name:"diff-algorithm" help:"Choose a diff algorithm, supported: histogram|onp|myers|patience|minimal" placeholder:"<algorithm>"`
	Limit         int64    `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
	Objects       []string `arg:"" optional:"" name:"object" help:""`
}

const (
	showSummaryFormat = `%szeta show [<options>] <object>...`
)

func (c *Show) Summary() string {
	return fmt.Sprintf(showSummaryFormat, W("Usage: "))
}

func (c *Show) checkAlgorithm() (diferenco.Algorithm, error) {
	if len(c.DiffAlgorithm) != 0 {
		return diferenco.AlgorithmFromName(c.DiffAlgorithm)
	}
	switch {
	case c.Histogram:
		return diferenco.Histogram, nil
	case c.ONP:
		return diferenco.ONP, nil
	case c.Myers:
		return diferenco.Myers, nil
	case c.Patience:
		return diferenco.Patience, nil
	case c.Minimal:
		return diferenco.Minimal, nil
	default:
	}
	return diferenco.Unspecified, nil
}

func (c *Show) Run(g *Globals) error {
	if len(c.Objects) == 0 {
		c.Objects = append(c.Objects, "HEAD")
	}
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	a, err := c.checkAlgorithm()
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse options error: %v\n", err)
		return err
	}
	return r.Show(context.Background(), &zeta.ShowOptions{
		Objects:   c.Objects,
		Textconv:  c.Textconv,
		Limit:     c.Limit,
		Algorithm: a,
	})
}
