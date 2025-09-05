// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
)

const (
	// pruneTargetPrefix
	//  refs/pull/${ID}/merge
	//  refs/pull/cloudide/turbodev
	pruneTargetPrefix = "refs/pull/"
)

var (
	// remove expired
	prefixToPrune = []string{
		"refs/heads/FASTQ",
		"refs/heads/conflict_fix_",
		"refs/heads/cooperate/cloudideantservice-",
		"refs/heads/cooperate/reading-FASTQ1",
		"refs/tags/cstone_stc_scan_",
	}
	extremePrefixToPrune = []string{
		"refs/heads/FASTQ",
		"refs/heads/conflict_fix_",
		"refs/heads/cooperate/cloudideantservice-",
		"refs/heads/cooperate/linkc-",
		"refs/heads/cooperate/reading-FASTQ1",
		"refs/heads/cooperate/sop_",
		"refs/heads/eval_ai_ide_",
		"refs/heads/eval_codefuse_augment_",
		"refs/heads/eval_idea_plugin_",
		"refs/heads/next_",
		"refs/heads/next_master_dev_",
		"refs/heads/unit_test_temp",
		"refs/heads/unit_test_temp_xdev",
		"refs/heads/xdev/",
		"refs/tags/cstone_stc_scan_",
	}

	// always remove
	dirtyRefPrefixes = []string{
		"refs/merge-requests/",
		"refs/tmp/",
	}
)

var statReferencesFormatFields = []string{
	"%(refname)", "%(refname:short)", "%(objectname)", "%(committername)", "%(creatordate:iso-strict)",
}

type Reference struct {
	Name       string    `json:"name"`
	ShortName  string    `json:"short_name"`
	Hash       string    `json:"hash"`
	Committer  string    `json:"committer"`
	LastUpdate time.Time `json:"last_update"`
}

func parseReferenceLine(referenceLine string) (*Reference, error) {
	elements := strings.SplitN(referenceLine, "\x00", len(statReferencesFormatFields))
	if len(elements) != len(statReferencesFormatFields) {
		return nil, fmt.Errorf("invalid output from git for-each-ref command: %v", referenceLine)
	}
	return &Reference{
		Name:       elements[0],
		ShortName:  elements[1],
		Hash:       elements[2],
		Committer:  elements[3],
		LastUpdate: git.PareTimeFallback(elements[4]),
	}, nil
}

func GetReferences(ctx context.Context, repoPath string, m func(*Reference) bool) ([]*Reference, error) {
	stderr := command.NewStderr()
	reader, err := git.NewReader(ctx, &command.RunOpts{RepoPath: repoPath, Stderr: stderr}, "for-each-ref", "--format", strings.Join(statReferencesFormatFields, "%00"))
	if err != nil {
		return nil, fmt.Errorf("run git for-each-ref error: %v", err)
	}
	defer reader.Close() // nolint
	references := make([]*Reference, 0, 100)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		r, err := parseReferenceLine(scanner.Text())
		if err != nil {
			continue
		}
		if m(r) {
			references = append(references, r)
		}
	}
	return references, nil
}

func isDirtyReference(name string) bool {
	return slices.ContainsFunc(dirtyRefPrefixes, func(prefix string) bool {
		return strings.HasPrefix(name, prefix)
	})
}

func prefixesMatch(name string, prefixes []string) bool {
	return slices.ContainsFunc(prefixes, func(prefix string) bool {
		return strings.HasPrefix(name, prefix)
	})
}

type PruneRefs struct {
	Prefixes []string      `arg:"" optional:"" name:"prefixes" help:"Reference prefixes that need to be cleaned up"` // to targets
	CWD      string        `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Expires  time.Duration `short:"e" name:"expires" help:"Reference expiration time, support: m, h, d, w" type:"expire" default:"90d"`
	DryRun   bool          `name:"dry-run" short:"n" help:"Dry run"`
	Default  bool          `short:"D" name:"default" help:"Cleanup references using default prefix"`
	Extreme  bool          `short:"E" name:"extreme" help:"Remove more dirty references"`
}

func (c *PruneRefs) preparePrefixes() (prefixes []string) {
	switch {
	case len(c.Prefixes) != 0:
		prefixes = append(prefixes, c.Prefixes...)
		prefixes = append(prefixes, pruneTargetPrefix)
		// List all references
	case c.Extreme:
		prefixes = append(prefixes, extremePrefixToPrune...)
		prefixes = append(prefixes, pruneTargetPrefix)
	case c.Default:
		prefixes = append(prefixes, prefixToPrune...)
		prefixes = append(prefixes, pruneTargetPrefix)
	default:
		prefixes = append(prefixes, pruneTargetPrefix)
	}
	return
}

func (c *PruneRefs) record(repoPath string, refs []*Reference) error {
	tempDir := filepath.Join(repoPath, "temp")
	if err := os.Mkdir(tempDir, 0755); err != nil && !os.IsExist(err) {
		fmt.Fprintf(os.Stderr, "new extraCross error: %v", err)
		return err
	}
	saveTo := filepath.Join(tempDir, strengthen.NewSessionID()+".refs")
	fd, err := os.Create(saveTo)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create record json error: %v", err)
		return err
	}
	defer fd.Close() // nolint
	for _, ref := range refs {
		_, _ = fmt.Fprintf(fd, "%s %s\n", ref.Hash, ref.Name)
	}
	return nil
}

func (c *PruneRefs) pruneRefs(ctx context.Context, repoPath string, references []*Reference) error {
	u, err := git.NewRefUpdater(ctx, repoPath, os.Environ(), false)
	if err != nil {
		fmt.Fprintf(os.Stderr, "RefUpdater: new ref updater error: %v\n", err)
		return err
	}
	defer u.Close() // nolint
	if err := u.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "RefUpdater: Start ref updater error: %v\n", err)
		return err
	}
	for _, ref := range references {
		if !c.DryRun {
			if err := u.Delete(git.ReferenceName(ref.Name)); err != nil {
				fmt.Fprintf(os.Stderr, "\x1b[2K\rRefUpdater: Delete %s error: %v\n", ref.Name, err)
				return err
			}
		}
		fmt.Fprintf(os.Stderr, "\x1b[2K\rDELETE '%s' (OID: %s Date: %s Committer: %s)", ref.ShortName, ref.Hash, ref.LastUpdate.Format(time.RFC3339), ref.Committer)
	}
	if c.DryRun {
		return nil
	}
	if err := u.Prepare(); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rRefUpdater: Prepare error: %v\n", err)
		return err
	}
	if err := u.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rRefUpdater: Commit error: %v\n", err)
		return err
	}
	return nil
}

func (c *PruneRefs) Run(g *Globals) error {
	repoPath := git.RevParseRepoPath(context.Background(), c.CWD)
	prefixes := c.preparePrefixes()
	fmt.Fprintf(os.Stderr, "\x1b[38;2;254;225;64m* The following ref prefixes will be deleted:\x1b[0m\n")
	for _, p := range prefixes {
		fmt.Fprintf(os.Stderr, "\x1b[38;2;254;225;64m*  %s\x1b[0m\n", p)
	}
	expiredAt := time.Now().Add(-c.Expires)
	references, err := GetReferences(context.Background(), repoPath, func(r *Reference) bool {
		return isDirtyReference(r.Name) || (prefixesMatch(r.Name, prefixes) && expiredAt.After(r.LastUpdate))
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse references error: %v\n", err)
		return err
	}
	if len(references) == 0 {
		fmt.Fprintf(os.Stderr, "No references to be deleted\n")
		return nil
	}
	if err := c.record(repoPath, references); err != nil {
		return err
	}
	if err := c.pruneRefs(context.Background(), repoPath, references); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\nPrune refs success, total: %d\n", len(references))
	return nil
}
