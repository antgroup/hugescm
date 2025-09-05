// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package replay

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/antgroup/hugescm/cmd/hot/pkg/bar"
	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
	"github.com/antgroup/hugescm/modules/survey"
	"github.com/antgroup/hugescm/modules/trace"
)

// 4MB size limit for squashed commit message
const maxSizeForSquashedCommitMessage = 4 << 20

func (r *Replayer) makeSquashMessage0(commits []string, message string) (string, error) {
	messages := []string{message}
	messageSize := len(message)
	for idx, s := range commits {
		if messageSize > maxSizeForSquashedCommitMessage {
			oversizeNotice := fmt.Sprintf("\n\n...\n %d more commit(s) ignored to avoid oversized message\n", len(commits)-idx)
			message := strings.Join(messages, "\n")
			return message[:maxSizeForSquashedCommitMessage] + oversizeNotice, nil
		}
		oid, err := hex.DecodeString(s)
		if err != nil {
			return "", err
		}
		cc, err := r.odb.Commit(oid)
		if err != nil {
			return "", err
		}
		if len(cc.ParentIDs) > 1 {
			// skip commit message for merge commit
			continue
		}
		messages = append(messages, "* "+cc.Subject())
		// 3 more chars[ *\n] will be appended for each message
		messageSize += 3 + len(cc.Message)
	}
	return strings.Join(messages, "\n"), nil
}

func (r *Replayer) makeSquashMessage(cc *gitobj.Commit) (string, error) {
	commits, err := git.RevUniqueList(r.ctx, r.repoPath, hex.EncodeToString(cc.ParentIDs[0]), hex.EncodeToString(cc.ParentIDs[1]))
	if err != nil {
		return "", err
	}
	// already merged
	if len(commits) == 0 {
		return cc.Message, nil
	}
	return r.makeSquashMessage0(commits, cc.Message)
}

// --first-parent
// Return all branch/tags commit reverse order
func (r *Replayer) commitsToLinear(revision string) ([][]byte, error) {
	psArgs := []string{"rev-list", "--reverse", "--topo-order", "--first-parent"}
	if len(revision) == 0 {
		psArgs = append(psArgs, "--all")
	} else {
		psArgs = append(psArgs, revision)
	}
	// --topo-order is required to ensure topological order.
	reader, err := git.NewReader(r.ctx, &command.RunOpts{RepoPath: r.repoPath}, psArgs...)
	if err != nil {
		return nil, err
	}
	defer reader.Close() // nolint
	sr := bufio.NewScanner(reader)
	var commits [][]byte
	for sr.Scan() {
		oid, err := hex.DecodeString(strings.TrimSpace(sr.Text()))
		if err != nil {
			continue
		}
		commits = append(commits, oid)
	}
	return commits, nil
}

func (r *Replayer) unbranch(revision string, keep int) ([]byte, error) {
	commits, err := r.commitsToLinear(revision)
	if err != nil {
		return nil, fmt.Errorf("commits to linear error: %w", err)
	}
	if keep > 0 && keep < len(commits) {
		commits = commits[len(commits)-keep:]
	}
	if len(commits) == 0 {
		return nil, errors.New("missing commits")
	}
	top := slices.Clone(commits[len(commits)-1])
	b := bar.NewBar(tr.W("rewrite commits"), len(commits), r.stepCurrent, r.stepEnd, r.verbose)
	r.stepCurrent++
	trace.DbgPrint("commits: %v", len(commits))
	for _, oid := range commits {
		original, err := r.odb.Commit(oid)
		if err != nil {
			return nil, err
		}
		message := original.Message
		rewrittenParents := make([][]byte, 0, len(original.ParentIDs))
		if len(original.ParentIDs) > 0 {
			if rewrittenParent, ok := r.uncacheCommit(original.ParentIDs[0]); ok {
				rewrittenParents = append(rewrittenParents, rewrittenParent)
			}
		}
		if len(original.ParentIDs) > 1 {
			if m, err := r.makeSquashMessage(original); err == nil {
				message = m
			}
		}
		// Construct a new commit using the original header information,
		// but the rewritten set of parents as well as root tree.
		rewrittenCommit := &gitobj.Commit{
			Author:       original.Author,
			Committer:    original.Committer,
			ExtraHeaders: original.ExtraHeaders,
			Message:      message,

			ParentIDs: rewrittenParents,
			TreeID:    original.TreeID,
		}

		var newSha []byte

		if original.Equal(rewrittenCommit) {
			newSha = make([]byte, len(oid))
			copy(newSha, oid)
		} else {
			if newSha, err = r.odb.WriteCommit(rewrittenCommit); err != nil {
				return nil, err
			}
		}
		// Cache that commit so that we can reassign children of this
		// commit.
		r.cacheCommit(oid, newSha)
		b.Add(1)
	}
	b.Done()
	return top, nil
}

type UnbranchOptions struct {
	Branch  string
	Target  string
	Confirm bool
	Prune   bool
	Keep    int
}

func (r *Replayer) Unbranch(o *UnbranchOptions) error {
	top, err := r.unbranch(o.Branch, o.Keep)
	if err != nil {
		return err
	}
	if len(o.Branch) != 0 {
		return r.unbranchOne(o, top)
	}
	if !o.Confirm {
		var confirm bool
		prompt := &survey.Confirm{
			Message: tr.W("Do you want to rewrite local branches and tags"),
		}
		_ = survey.AskOne(prompt, &confirm)
		if !confirm {
			return nil
		}
	}
	refs, err := r.referencesToRewrite()
	if err != nil {
		return errors.New("could not find refs to update")
	}

	updater := &refUpdater{
		CacheFn:    r.uncacheCommit,
		References: refs,
		RepoPath:   r.repoPath,
		odb:        r.odb,
	}
	b := bar.NewBar(tr.W("rewrite references"), len(refs), r.stepCurrent, r.stepEnd, r.verbose)
	r.stepCurrent++
	if err := updater.UpdateRefs(r.ctx, b); err != nil {
		return errors.New("could not update refs")
	}
	b.Done()
	return r.cleanup(o.Prune)
}

func (r *Replayer) unbranchOne(o *UnbranchOptions, top []byte) error {
	newOID, ok := r.uncacheCommit(top)
	if !ok {
		return fmt.Errorf("find migrate commit error, origin: %s", hex.EncodeToString(top))
	}
	newRev := hex.EncodeToString(newOID)
	var oldRev, refname string
	ref, err := git.ReferencePrefixMatch(r.ctx, r.repoPath, o.Branch)
	switch {
	case git.IsErrNotExist(err):
		if len(o.Target) == 0 {
			_, _ = fmt.Fprintf(os.Stdout, "Dangling: %s\n", newRev)
			return nil
		}
		oldRev = git.ConformingHashZero(newRev)
		refname = git.JoinBranchPrefix(o.Target)
	case err != nil:
		return err
	case len(o.Target) != 0:
		oldRev = git.ConformingHashZero(newRev)
		refname = git.JoinBranchPrefix(o.Target)
	default:
		oldRev = ref.Target
		refname = ref.Name.String()
	}
	fmt.Fprintf(os.Stderr, "Update '%s' %s --> %s\n", refname, oldRev, newRev)
	if err := git.UpdateRef(r.ctx, r.repoPath, refname, oldRev, newRev, false); err != nil {
		return err
	}
	return nil
}
