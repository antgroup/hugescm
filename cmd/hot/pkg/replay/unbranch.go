// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package replay

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
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
func (r *Replayer) commitsToLinear(branch string) ([][]byte, error) {
	psArgs := []string{"rev-list", "--reverse", "--topo-order", "--first-parent"}
	if len(branch) == 0 {
		psArgs = append(psArgs, "--all")
	} else {
		psArgs = append(psArgs, branch)
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

func (r *Replayer) unbranch(branchName string, keep int) error {
	commits, err := r.commitsToLinear(branchName)
	if err != nil {
		return fmt.Errorf("commits to linear error: %w", err)
	}
	if keep > 0 && keep < len(commits) {
		commits = commits[len(commits)-keep:]
	}
	b := bar.NewBar(tr.W("rewrite commits"), len(commits), r.stepCurrent, r.stepEnd, r.verbose)
	r.stepCurrent++
	trace.DbgPrint("commits: %v", len(commits))
	for _, oid := range commits {
		original, err := r.odb.Commit(oid)
		if err != nil {
			return err
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
				return err
			}
		}
		// Cache that commit so that we can reassign children of this
		// commit.
		r.cacheCommit(oid, newSha)
		b.Add(1)
	}
	b.Done()
	return nil
}

func (r *Replayer) Unbranch(branchName string, confirm bool, prune bool, keep int) error {
	if err := r.unbranch(branchName, keep); err != nil {
		return err
	}
	if !confirm {
		prompt := &survey.Confirm{
			Message: tr.W("Do you want to rewrite local branches and tags"),
		}
		_ = survey.AskOne(prompt, &confirm)
		if !confirm {
			return nil
		}
	}
	if len(branchName) != 0 {
		return r.unbranchOne(branchName, prune)
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
	return r.cleanup(prune)
}

func (r *Replayer) unbranchOne(branchName string, prune bool) error {
	ref, err := git.ReferencePrefixMatch(r.ctx, r.repoPath, branchName)
	if err != nil {
		return err
	}
	refs := []*git.Reference{
		{
			Name:       ref.Name,
			Hash:       ref.Hash,
			ObjectType: ref.ObjectType,
			ShortName:  ref.ShortName,
		},
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
	return r.cleanup(prune)
}
