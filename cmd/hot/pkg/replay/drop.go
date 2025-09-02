package replay

import (
	"errors"
	"fmt"
	"path"

	"github.com/antgroup/hugescm/cmd/hot/pkg/bar"
	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
	"github.com/antgroup/hugescm/modules/survey"
	"github.com/antgroup/hugescm/modules/trace"
)

func (r *Replayer) rewriteTree(m Matcher, commitOID []byte, treeOID []byte, parent string) ([]byte, error) {
	tree, err := r.odb.Tree(treeOID)
	if err != nil {
		return nil, err
	}

	entries := make([]*gitobj.TreeEntry, 0, len(tree.Entries))
	for _, entry := range tree.Entries {
		name := path.Join(parent, entry.Name)
		// matched path
		if m.Match(entry, name) {
			continue
		}
		if entry.Type() == gitobj.BlobObjectType {
			entries = append(entries, copyEntry(entry))
			continue
		}
		// If this is a symlink, skip it
		if entry.Filemode == 0120000 {
			entries = append(entries, copyEntry(entry))
			continue
		}

		if cached := r.uncacheEntry(name, entry); cached != nil {
			entries = append(entries, copyEntryMode(cached, entry.Filemode))
			continue
		}

		var oid []byte

		switch entry.Type() {
		case gitobj.TreeObjectType:
			oid, err = r.rewriteTree(m, commitOID, entry.Oid, name)
		default:
			oid = entry.Oid

		}
		if err != nil {
			return nil, err
		}

		entries = append(entries, r.cacheEntry(name, entry, &gitobj.TreeEntry{
			Filemode: entry.Filemode,
			Name:     entry.Name,
			Oid:      oid,
		}))
	}
	rewritten := &gitobj.Tree{Entries: entries}
	if tree.Equal(rewritten) {
		return treeOID, nil
	}
	return r.odb.WriteTree(rewritten)
}

func (r *Replayer) rewriteCommits(m Matcher) error {
	commits, err := r.commitsToRewrite()
	if err != nil {
		return fmt.Errorf("commits to rewrite error: %w", err)
	}
	b := bar.NewBar(tr.W("rewrite commits"), len(commits), r.stepCurrent, r.stepEnd, r.verbose)
	r.stepCurrent++
	trace.DbgPrint("commits: %v", len(commits))
	for _, oid := range commits {
		original, err := r.odb.Commit(oid)
		if err != nil {
			return err
		}
		rewrittenTree, err := r.rewriteTree(m, oid, original.TreeID, "")
		if err != nil {
			return err
		}
		// Create a new list of parents from the original commit to
		// point at the rewritten parents in order to create a
		// topologically equivalent DAG.
		//
		// This operation is safe since we are visiting the commits in
		// reverse topological order and therefore have seen all parents
		// before children (in other words, r.uncacheCommit(...) will
		// always return a value, if the prospective parent is a part of
		// the migration).
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
			TreeID:    rewrittenTree,
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

func (r *Replayer) Drop(m Matcher, confirm bool, prune bool) error {
	if !confirm {
		if !git.IsBareRepository(r.ctx, r.repoPath) {
			// core.bare
			prompt := &survey.Confirm{
				Message: tr.W("Repository not bare repository, continue to rewrite"),
			}
			_ = survey.AskOne(prompt, &confirm)
			if !confirm {
				return nil
			}
		}
	}
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
