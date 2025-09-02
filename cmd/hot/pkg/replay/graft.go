package replay

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"

	"github.com/antgroup/hugescm/cmd/hot/pkg/bar"
	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
	"github.com/antgroup/hugescm/modules/survey"
)

func (r *Replayer) resolveCommit(ref *git.Reference) ([]byte, *gitobj.Commit, error) {
	sha, err := hex.DecodeString(ref.Hash)
	if err != nil {
		return nil, nil, fmt.Errorf("could not decode: %q", ref.Hash)
	}
	for i := 0; i < 20; i++ {
		obj, err := r.odb.Object(sha)
		if err != nil {
			return nil, nil, fmt.Errorf("open git object error: %w", err)
		}
		if obj.Type() == gitobj.CommitObjectType {
			return sha, obj.(*gitobj.Commit), nil
		}
		if obj.Type() != gitobj.TagObjectType {
			return nil, nil, fmt.Errorf("oid: %s unsupported object type: %s", hex.EncodeToString(sha), obj.Type())
		}
		tag := obj.(*gitobj.Tag)
		sha = tag.Object
	}
	return nil, nil, fmt.Errorf("ref '%s' recursion depth is not supported", ref.Name)
}

// graft HEAD
func (r *Replayer) graftHEAD() error {
	oldRev, _, err := git.RevParseCurrentEx(r.ctx, os.Environ(), r.repoPath)
	if err != nil {
		return err
	}
	oid, err := hex.DecodeString(oldRev)
	if err != nil {
		return err
	}
	original, err := r.odb.Commit(oid)
	if err != nil {
		return err
	}
	rewrittenParents := make([][]byte, 0, len(original.ParentIDs))
	for _, originalParent := range original.ParentIDs {
		rewrittenParent, ok := r.uncacheCommit(originalParent)
		if !ok {
			// If we haven't seen the parent before, this
			// means that we're doing a partial migration
			// and the parent that we're looking for isn't
			// included.
			//
			// Use the original parent to properly link
			// history across the migration boundary.
			rewrittenParent = originalParent
		}

		rewrittenParents = append(rewrittenParents, rewrittenParent)
	}
	// Construct a new commit using the original header information,
	// but the rewritten set of parents as well as root tree.
	rewrittenCommit := &gitobj.Commit{
		Author:       original.Author,
		Committer:    original.Committer,
		ExtraHeaders: original.ExtraHeaders,
		Message:      original.Message,

		ParentIDs: rewrittenParents,
		TreeID:    original.TreeID,
	}

	var newSha []byte

	if original.Equal(rewrittenCommit) {
		newSha = make([]byte, len(oid))
		copy(newSha, oid)
	} else {
		newSha, err = r.odb.WriteCommit(rewrittenCommit)
		if err != nil {
			return err
		}
	}
	// Cache that commit so that we can reassign children of this
	// commit.
	r.cacheCommit(oid, newSha)
	return nil
}

func (r *Replayer) graftCommits(refs []*git.Reference, headOnly bool) error {
	if headOnly {
		b := bar.NewBar(tr.W("graft commits"), 1, r.stepCurrent, r.stepEnd, r.verbose)
		r.stepCurrent++
		if err := r.graftHEAD(); err != nil {
			return err
		}
		b.Done()
		return nil
	}
	b := bar.NewBar(tr.W("graft commits"), len(refs), r.stepCurrent, r.stepEnd, r.verbose)
	r.stepCurrent++
	for _, ref := range refs {
		oid, original, err := r.resolveCommit(ref)
		if err != nil {
			return err
		}

		rewrittenParents := make([][]byte, 0, len(original.ParentIDs))
		for _, originalParent := range original.ParentIDs {
			rewrittenParent, ok := r.uncacheCommit(originalParent)
			if !ok {
				// If we haven't seen the parent before, this
				// means that we're doing a partial migration
				// and the parent that we're looking for isn't
				// included.
				//
				// Use the original parent to properly link
				// history across the migration boundary.
				rewrittenParent = originalParent
			}

			rewrittenParents = append(rewrittenParents, rewrittenParent)
		}
		// Construct a new commit using the original header information,
		// but the rewritten set of parents as well as root tree.
		rewrittenCommit := &gitobj.Commit{
			Author:       original.Author,
			Committer:    original.Committer,
			ExtraHeaders: original.ExtraHeaders,
			Message:      original.Message,

			ParentIDs: rewrittenParents,
			TreeID:    original.TreeID,
		}

		var newSha []byte

		if original.Equal(rewrittenCommit) {
			newSha = make([]byte, len(oid))
			copy(newSha, oid)
		} else {
			newSha, err = r.odb.WriteCommit(rewrittenCommit)
			if err != nil {
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

func (r *Replayer) Graft(m Matcher, confirm bool, prune bool, headOnly bool) error {
	if err := r.rewriteCommits(m); err != nil {
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
	refs, err := r.referencesToRewrite()
	if err != nil {
		return errors.New("could not find refs to update")
	}

	if err := r.graftCommits(refs, headOnly); err != nil {
		return err
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
