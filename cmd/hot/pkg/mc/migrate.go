// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package mc

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/antgroup/hugescm/cmd/hot/pkg/bar"
	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/trace"
)

type Migrator struct {
	from string
	to   string
	// mu guards entries and commits (see below)
	mu *sync.Mutex
	// objects is a mapping of old objects SHAs (SHA1) to new ones (SHA256), where the ASCII
	// hex encoding of the SHA1 values are used as map keys.
	objects     map[string][]byte
	odb         *git.ODB
	newODB      *git.ODB
	worktree    string
	stepEnd     int
	stepCurrent int
	verbose     bool
}

func (m *Migrator) uncache(from []byte) ([]byte, bool) {
	m.mu.Lock()
	c, ok := m.objects[hex.EncodeToString(from)]
	m.mu.Unlock()
	return c, ok
}

func (m *Migrator) cache(from, to []byte) {
	m.mu.Lock()
	m.objects[hex.EncodeToString(from)] = to
	m.mu.Unlock()
}

type MigrateOptions struct {
	From    string
	To      string
	Format  string
	Bare    bool
	Verbose bool
	StepEnd int
}

func NewMigrator(ctx context.Context, opts *MigrateOptions) (*Migrator, error) {
	fromPath := git.RevParseRepoPath(ctx, opts.From)
	current, err := git.RevParseCurrent(ctx, nil, opts.From)
	if err != nil {
		return nil, err
	}
	oldFormat := git.HashFormatOK(fromPath)
	newFormat := git.HashFormatFromName(opts.Format)
	if oldFormat == newFormat {
		return nil, fmt.Errorf("source repository object format is already: %s", opts.Format)
	}
	odb, err := git.NewODB(fromPath, oldFormat)
	if err != nil {
		return nil, err
	}
	if err := git.NewRepo(ctx, opts.To, current, opts.Bare, newFormat); err != nil {
		_ = odb.Close()
		return nil, err
	}
	toPath := git.RevParseRepoPath(ctx, opts.To)
	newODB, err := git.NewODB(toPath, newFormat)
	if err != nil {
		_ = odb.Close()
		return nil, err
	}
	r := &Migrator{
		from:        fromPath,
		to:          toPath,
		mu:          new(sync.Mutex),
		objects:     make(map[string][]byte),
		odb:         odb,
		newODB:      newODB,
		stepEnd:     opts.StepEnd,
		stepCurrent: 1,
		verbose:     opts.Verbose,
	}
	if !opts.Bare {
		r.worktree = opts.To
	}
	return r, nil
}

func (m *Migrator) Close() error {
	if m.newODB != nil {
		_ = m.newODB.Close()
	}
	if m.odb != nil {
		_ = m.odb.Close()
	}
	return nil
}

// getAllCommits: Return all branch/tags commit reverse order
func (m *Migrator) getAllCommits(ctx context.Context) ([][]byte, error) {
	// --topo-order is required to ensure topological order.
	reader, err := git.NewReader(ctx, &command.RunOpts{RepoPath: m.from}, "rev-list", "--reverse", "--topo-order", "--all")
	if err != nil {
		return nil, err
	}
	defer reader.Close()
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

func (m *Migrator) hashObject(oid []byte) ([]byte, error) {
	br, err := m.odb.Blob(oid)
	if err != nil {
		return nil, err
	}
	defer br.Close()
	return m.newODB.WriteBlob(&gitobj.Blob{
		Size:     br.Size,
		Contents: br.Contents,
	})
}

func countObjects(ctx context.Context, repoPath string) int {
	reader, err := git.NewReader(ctx, &command.RunOpts{RepoPath: repoPath}, "count-objects", "-v")
	if err != nil {
		return -1
	}
	defer reader.Close()
	nums := make(map[string]int)
	br := bufio.NewScanner(reader)
	for br.Scan() {
		k, v, ok := strings.Cut(br.Text(), ":")
		if !ok {
			continue
		}
		n, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil {
			return -1
		}
		nums[k] = n
	}
	if total := nums["count"] + nums["in-pack"]; total != 0 {
		return total
	}
	return -1
}

func (m *Migrator) hashObjects(ctx context.Context) error {
	reader, err := git.NewReader(ctx, &command.RunOpts{RepoPath: m.from}, "cat-file", "--batch-check", "--batch-all-objects", "--unordered")
	if err != nil {
		return fmt.Errorf("start git cat-file error %w", err)
	}
	defer reader.Close()
	br := bufio.NewScanner(reader)
	objectsCount := countObjects(ctx, m.from)
	b := bar.NewBar(tr.W("fast rewrite objects"), objectsCount, m.stepCurrent, m.stepEnd, m.verbose)
	m.stepCurrent++
	// format: 1a1db8dba9f976364fb6dab3e29deaf0f1140ed8 blob 5155
	for br.Scan() {
		line := br.Text()
		sv := strings.Fields(line)
		if len(sv) < 3 {
			b.Add(1)
			continue
		}
		if sv[1] != "blob" {
			b.Add(1)
			continue
		}
		oid, err := hex.DecodeString(sv[0])
		if err != nil {
			return fmt.Errorf("git cat-file decode hex error %w", err)
		}

		newOID, err := m.hashObject(oid)
		if err != nil {
			return fmt.Errorf("convert blob from sha1 to sha256 error %w", err)
		}
		m.cache(oid, newOID)
		b.Add(1)
	}
	b.Done()
	return nil
}

func (m *Migrator) rewriteTree(commitOID []byte, treeOID []byte) ([]byte, error) {
	tree, err := m.odb.Tree(treeOID)
	if err != nil {
		return nil, err
	}
	var oid []byte
	var ok bool
	entries := make([]*gitobj.TreeEntry, 0, len(tree.Entries))
	for _, e := range tree.Entries {
		switch e.Type() {
		case gitobj.BlobObjectType:
			if oid, ok = m.uncache(e.Oid); !ok {
				if oid, err = m.hashObject(e.Oid); err != nil {
					return nil, fmt.Errorf("rewrite %s error: %w", hex.EncodeToString(e.Oid), err)
				}
				m.cache(e.Oid, oid)
			}
			entries = append(entries, &gitobj.TreeEntry{Name: e.Name, Oid: oid, Filemode: e.Filemode})
		case gitobj.TreeObjectType:
			if oid, ok = m.uncache(e.Oid); !ok {
				if oid, err = m.rewriteTree(commitOID, e.Oid); err != nil {
					return nil, fmt.Errorf("rewrite %s error: %w", hex.EncodeToString(e.Oid), err)
				}
				m.cache(e.Oid, oid)
			}
			entries = append(entries, &gitobj.TreeEntry{Name: e.Name, Oid: oid, Filemode: e.Filemode})
		default:
			// FIXME: git currently does not support managing sha1 submodules in sha256 repositories
			// if e.Type() == gitobj.CommitObjectType {
			// 	newOID := make([]byte, len(e.Oid))
			// 	copy(newOID, e.Oid)
			// 	entries = append(entries, &gitobj.TreeEntry{Name: e.Name, Oid: newOID, Filemode: e.Filemode})
			// 	continue
			// }
			fmt.Fprintf(os.Stderr, "\nTreeEntry type '%s' not supported for migration\n", e.Type())
		}
	}
	return m.newODB.WriteTree(&gitobj.Tree{Entries: entries})
}

func (m *Migrator) rewriteCommits(ctx context.Context) error {
	commits, err := m.getAllCommits(ctx)
	if err != nil {
		return fmt.Errorf("commits to migrate error: %w", err)
	}
	b := bar.NewBar(tr.W("rewrite commits"), len(commits), m.stepCurrent, m.stepEnd, m.verbose)
	m.stepCurrent++
	trace.DbgPrint("commits: %v", len(commits))
	for _, oid := range commits {
		oc, err := m.odb.Commit(oid)
		if err != nil {
			return err
		}
		var newTree []byte
		var ok bool
		if newTree, ok = m.uncache(oc.TreeID); !ok {
			if newTree, err = m.rewriteTree(oid, oc.TreeID); err != nil {
				return err
			}
			m.cache(oc.TreeID, newTree)
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
		rewrittenParents := make([][]byte, 0, len(oc.ParentIDs))
		for _, sha1Parent := range oc.ParentIDs {
			rewrittenParent, ok := m.uncache(sha1Parent)
			if !ok {
				// If we haven't seen the parent before, this
				// means that we're doing a partial migration
				// and the parent that we're looking for isn't
				// included.
				//
				// Use the original parent to properly link
				// history across the migration boundary.
				continue
			}

			rewrittenParents = append(rewrittenParents, rewrittenParent)
		}

		// Construct a new commit using the original header information,
		// but the rewritten set of parents as well as root tree.
		rewrittenCommit := &gitobj.Commit{
			Author:       oc.Author,
			Committer:    oc.Committer,
			ExtraHeaders: oc.ExtraHeaders,
			Message:      oc.Message,

			ParentIDs: rewrittenParents,
			TreeID:    newTree,
		}

		var newSha []byte
		if newSha, err = m.newODB.WriteCommit(rewrittenCommit); err != nil {
			return err
		}
		// Cache that commit so that we can reassign children of this
		// commit.
		m.cache(oid, newSha)
		b.Add(1)
	}
	b.Done()
	return nil
}

// getReferences returns a list of references to migrate, or an error if loading
// those references failed.
func (m *Migrator) getReferences(ctx context.Context) ([]*git.Reference, error) {
	refs, err := git.ParseReferences(ctx, m.from, git.OrderNone)
	if err != nil {
		return nil, err
	}
	references := make([]*git.Reference, 0, len(refs))
	for _, ref := range refs {
		if strings.HasPrefix(ref.Name, "refs/remotes/") {
			continue
		}
		references = append(references, ref)
	}

	return references, nil
}

func (m *Migrator) encodeTag(tag *gitobj.Tag, newObj []byte) ([]byte, error) {
	newTag, err := m.newODB.WriteTag(&gitobj.Tag{
		Object:     newObj,
		ObjectType: tag.ObjectType,
		Name:       tag.Name,
		Tagger:     tag.Tagger,

		Message: tag.Message,
	})

	if err != nil {
		return nil, fmt.Errorf("could not rewrite tag: %s", tag.Name)
	}
	return newTag, nil
}

func (m *Migrator) rewriteTag(oid []byte) ([]byte, error) {
	tag, err := m.odb.Tag(oid)
	if err != nil {
		return nil, err
	}
	if tag.ObjectType == gitobj.TagObjectType {
		newTag, err := m.rewriteTag(tag.Object)
		if err != nil {
			return nil, err
		}
		return m.encodeTag(tag, newTag)

	}
	if tag.ObjectType == gitobj.CommitObjectType {
		if to, ok := m.uncache(tag.Object); ok {
			return m.encodeTag(tag, to)
		}
	}
	return oid, nil
}

func (m *Migrator) rewriteOneRef(ref *git.Reference) ([]byte, error) {
	oid, err := hex.DecodeString(ref.Hash)
	if err != nil {
		return nil, fmt.Errorf("could not decode: '%s'", ref.Hash)
	}
	if newOID, ok := m.uncache(oid); ok {
		return newOID, nil
	}
	if ref.ObjectType == git.CommitObject {
		// BUGS: We have completed the conversion of all commits
		return nil, nil
	}
	return m.rewriteTag(oid)
}

func (m *Migrator) reconstruct(ctx context.Context) error {
	refs, err := m.getReferences(ctx)
	if err != nil {
		return err
	}
	if len(refs) == 0 {
		fmt.Fprintf(os.Stderr, "No references to be deleted\n")
		return nil
	}
	b := bar.NewBar(tr.W("rewrite references"), len(refs), m.stepCurrent, m.stepEnd, m.verbose)
	m.stepCurrent++
	var oid []byte
	u, err := git.NewRefUpdater(ctx, m.to, nil, false)
	if err != nil {
		return err
	}
	defer u.Close() // nolint
	if err := u.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "RefUpdater: Start ref updater error: %v\n", err)
		return err
	}
	for _, ref := range refs {
		if oid, err = m.rewriteOneRef(ref); err != nil {
			return fmt.Errorf("rewrite one ref '%s' error: %v", ref.Name, err)
		}
		if oid == nil {
			continue
		}
		if err := u.Create(ref.Name, hex.EncodeToString(oid)); err != nil {
			return fmt.Errorf("update-ref '%s' error: %v", ref.Name, err)
		}
		b.Add(1)
	}
	if err := u.Prepare(); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rRefUpdater: Prepare error: %v\n", err)
		return err
	}
	if err := u.Commit(); err != nil {
		fmt.Fprintf(os.Stderr, "\x1b[2K\rRefUpdater: Commit error: %v\n", err)
		return err
	}
	b.Done()
	return nil
}

func (m *Migrator) Execute(ctx context.Context) error {
	if err := m.hashObjects(ctx); err != nil {
		return err
	}
	if err := m.rewriteCommits(ctx); err != nil {
		return err
	}
	if err := m.reconstruct(ctx); err != nil {
		return err
	}
	return m.cleanup(ctx)
}

func (m *Migrator) reset(ctx context.Context) error {
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Environ:   os.Environ(),
		RepoPath:  m.worktree,
		Stderr:    os.Stderr,
		Stdout:    os.Stdout,
		Stdin:     os.Stdin,
		NoSetpgid: true,
	}, "git", "reset", "--hard")
	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "checkout error: %v", err)
		return err
	}
	return nil
}

func (m *Migrator) cleanup(ctx context.Context) error {
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Environ:   os.Environ(),
		RepoPath:  m.to,
		Stderr:    os.Stderr,
		Stdout:    os.Stdout,
		Stdin:     os.Stdin,
		NoSetpgid: true,
	}, "git", "-c", "repack.writeBitmaps=true", "-c", "pack.packSizeLimit=16g", "gc")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run git gc error: %w", err)
	}
	diskSize, err := strengthen.Du(filepath.Join(m.to, "objects"))
	if err != nil {
		return fmt.Errorf("du repo size error: %w", err)
	}
	fmt.Fprintf(os.Stderr, "\x1b[38;2;72;198;239m[%d/%d]\x1b[0m %s: \x1b[38;2;32;225;215m%s\x1b[0m %s: \x1b[38;2;72;198;239m%s\x1b[0m\n",
		m.stepCurrent, m.stepEnd, tr.W("Repository"), m.to, tr.W("size"), strengthen.FormatSize(diskSize))
	if len(m.worktree) != 0 {
		_ = m.reset(ctx)
	}
	return nil
}
