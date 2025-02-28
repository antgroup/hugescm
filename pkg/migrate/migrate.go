// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package migrate

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
	"github.com/antgroup/hugescm/modules/lfs"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type Debugger interface {
	DbgPrint(format string, args ...any)
}

type blob struct {
	oid       plumbing.Hash
	size      int64
	fragments bool
}

type MigrateOptions struct {
	Environ  []string
	From     string
	To       string
	StepEnd  int
	Squeeze  bool
	LFS      bool
	Quiet    bool
	Verbose  bool
	Values   []string
	Debugger Debugger
}

type Migrator struct {
	environ []string
	from    string
	to      string
	current string
	squeeze bool
	lfs     bool
	// mu guards entries and commits (see below)
	mu           *sync.Mutex
	metadata     map[string]plumbing.Hash
	blobs        map[string]*blob
	gitODB       *git.ODB
	r            *zeta.Repository
	modification int64
	stepEnd      int
	stepCurrent  int
	verbose      bool
	Debugger
}

func NewMigrator(ctx context.Context, opts *MigrateOptions) (*Migrator, error) {
	fromPath := git.RevParseRepoPath(ctx, opts.From)
	current, err := git.RevParseCurrent(ctx, opts.Environ, opts.From)
	if err != nil {
		return nil, err
	}

	odb, err := git.NewODB(fromPath, git.HashAlgoAlwaysOK(opts.From))
	if err != nil {
		return nil, err
	}
	repo, err := zeta.Init(ctx, &zeta.InitOptions{Worktree: opts.To, MustEmpty: true, Values: opts.Values, Quiet: opts.Quiet, Verbose: opts.Verbose})
	if err != nil {
		_ = odb.Close()
		return nil, err
	}
	r := &Migrator{
		environ:      opts.Environ,
		from:         fromPath,
		to:           opts.To,
		current:      current,
		squeeze:      opts.Squeeze,
		lfs:          opts.LFS,
		mu:           new(sync.Mutex),
		metadata:     make(map[string]plumbing.Hash),
		blobs:        make(map[string]*blob),
		gitODB:       odb,
		r:            repo,
		modification: time.Now().Unix(),
		Debugger:     opts.Debugger,
		stepEnd:      opts.StepEnd,
		stepCurrent:  1,
		verbose:      opts.Verbose,
	}
	return r, nil
}

func (m *Migrator) Close() error {
	if m.r != nil {
		_ = m.r.Close()
	}
	if m.gitODB != nil {
		_ = m.gitODB.Close()
	}
	return nil
}

func (m *Migrator) uncacheMD(from []byte) (plumbing.Hash, bool) {
	m.mu.Lock()
	c, ok := m.metadata[hex.EncodeToString(from)]
	m.mu.Unlock()
	return c, ok
}

func (m *Migrator) cacheMD(from []byte, to plumbing.Hash) {
	m.mu.Lock()
	m.metadata[hex.EncodeToString(from)] = to
	m.mu.Unlock()
}

func (m *Migrator) uncache(from []byte) (*blob, bool) {
	m.mu.Lock()
	c, ok := m.blobs[hex.EncodeToString(from)]
	m.mu.Unlock()
	return c, ok
}

func (m *Migrator) cache(from []byte, to *blob) {
	m.mu.Lock()
	m.blobs[hex.EncodeToString(from)] = to
	m.mu.Unlock()
}

// commitsToMigrate: Return all branch/tags commit reverse order
func (m *Migrator) commitsToMigrate(ctx context.Context) ([][]byte, error) {
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

const (
	// blobSizeCutoff is used to determine which files to scan for Git LFS
	// pointers.  Any file with a size below this cutoff will be scanned.
	blobSizeCutoff = 1024
)

func (m *Migrator) hashTo(ctx context.Context, r io.Reader, size int64) (*blob, error) {
	newOID, fragments, err := m.r.HashTo(ctx, r, size)
	if err != nil {
		return nil, err
	}
	return &blob{oid: newOID, size: size, fragments: fragments}, nil
}

func (m *Migrator) lfsJoin(p *lfs.Pointer) string {
	return filepath.Join(m.from, "lfs/objects", p.Oid[0:2], p.Oid[2:4], p.Oid)
}

func (m *Migrator) migrateLFSObject(ctx context.Context, br *gitobj.Blob) (*blob, error) {
	b, err := io.ReadAll(br.Contents)
	if err != nil {
		return nil, err
	}
	p, err := lfs.Decode(b)
	if err != nil {
		return m.hashTo(ctx, bytes.NewReader(b), br.Size)
	}
	fd, err := os.Open(m.lfsJoin(p))
	if err != nil {
		return m.hashTo(ctx, bytes.NewReader(b), br.Size)
	}
	defer fd.Close()
	return m.hashTo(ctx, fd, p.Size)
}

func (m *Migrator) migrateBlob(ctx context.Context, oid []byte) (*blob, error) {
	br, err := m.gitODB.Blob(oid)
	if err != nil {
		return nil, err
	}
	defer br.Close()
	if m.lfs && br.Size < blobSizeCutoff {
		return m.migrateLFSObject(ctx, br)
	}
	return m.hashTo(ctx, br.Contents, br.Size)
}

type migrateGroup struct {
	ch     chan []byte
	errors chan error
	wg     sync.WaitGroup
	bar    *ProgressBar
}

func (cg *migrateGroup) waitClose() {
	close(cg.ch)
	cg.wg.Wait()
}

func (cg *migrateGroup) submit(ctx context.Context, oid []byte) error {
	// In case the context has been cancelled, we have a race between observing an error from
	// the killed Git process and observing the context cancellation itself. But if we end up
	// here because of cancellation of the Git process, we don't want to pass that one down the
	// pipeline but instead just stop the pipeline gracefully. We thus have this check here up
	// front to error messages from the Git process.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-cg.errors:
		return err
	default:
	}

	select {
	case cg.ch <- oid:
		return nil
	case err := <-cg.errors:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (cg *migrateGroup) convert(ctx context.Context, m *Migrator) error {
	for oid := range cg.ch {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}
		if _, ok := m.uncache(oid); ok {
			continue
		}
		b, err := m.migrateBlob(ctx, oid)
		if err != nil {
			return err
		}
		m.cache(oid, b)
		cg.bar.Add(1)
	}
	return nil
}

func (cg *migrateGroup) run(ctx context.Context, m *Migrator) {
	cg.wg.Add(1)
	go func() {
		defer cg.wg.Done()
		err := cg.convert(ctx, m)
		cg.errors <- err
	}()
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

const (
	batchLimit = 8
)

func (m *Migrator) migrateBlobs(ctx context.Context) error {
	reader, err := git.NewReader(ctx, &command.RunOpts{RepoPath: m.from}, "cat-file", "--batch-check", "--batch-all-objects", "--unordered")
	if err != nil {
		return fmt.Errorf("start git cat-file error %w", err)
	}
	defer reader.Close()
	br := bufio.NewReader(reader)
	objectsCount := countObjects(ctx, m.from)
	bar := NewBar(tr.W("Migrate Blobs"), objectsCount, m.stepCurrent, m.stepEnd, m.verbose)
	m.stepCurrent++

	newCtx, cancelCtx := context.WithCancelCause(ctx)
	defer cancelCtx(nil)

	cg := &migrateGroup{
		ch:     make(chan []byte, 20), // 8 goroutine
		errors: make(chan error, batchLimit),
		bar:    bar,
	}
	for i := 0; i < batchLimit; i++ {
		cg.run(newCtx, m)
	}
	for {
		line, err := br.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			cg.waitClose()
			return fmt.Errorf("git cat-file error %w", err)
		}
		line = line[:len(line)-1]
		sv := strings.Split(line, " ")
		if len(sv) < 3 {
			bar.Add(1)
			continue
		}
		if sv[1] != "blob" {
			bar.Add(1)
			continue
		}
		oid, err := hex.DecodeString(sv[0])
		if err != nil {
			cg.waitClose()
			return fmt.Errorf("git cat-file decode hex error %w", err)
		}
		if err := cg.submit(newCtx, oid); err != nil {
			cg.waitClose()
			return err
		}
	}
	cg.waitClose()
	close(cg.errors)
	for err = range cg.errors {
		if err != nil {
			return err
		}
	}
	bar.Done()
	return nil
}

func (m *Migrator) migrateTrees(ctx context.Context, ur *backend.Unpacker, treeOID []byte, parent string) (plumbing.Hash, error) {
	tree, err := m.gitODB.Tree(treeOID)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	var oid plumbing.Hash
	var b *blob
	var ok bool
	entries := make([]*object.TreeEntry, 0, len(tree.Entries))
	for _, e := range tree.Entries {
		if e.Type() == gitobj.BlobObjectType {
			if b, ok = m.uncache(e.Oid); !ok {
				if b, err = m.migrateBlob(ctx, e.Oid); err != nil {
					return plumbing.ZeroHash, fmt.Errorf("rewrite %s error: %w", hex.EncodeToString(e.Oid), err)
				}
				m.cache(e.Oid, b)
			}
			if b.fragments {
				entries = append(entries, &object.TreeEntry{Name: e.Name, Hash: b.oid, Mode: filemode.FileMode(e.Filemode) | filemode.Fragments, Size: b.size})
				continue
			}
			entries = append(entries, &object.TreeEntry{Name: e.Name, Hash: b.oid, Mode: filemode.FileMode(e.Filemode), Size: b.size})
			continue
		}
		if e.Type() == gitobj.TreeObjectType {
			if oid, ok = m.uncacheMD(e.Oid); !ok {
				if oid, err = m.migrateTrees(ctx, ur, e.Oid, path.Join(parent, e.Name)); err != nil {
					return plumbing.ZeroHash, fmt.Errorf("rewrite %s error: %w", hex.EncodeToString(e.Oid), err)
				}
				m.cacheMD(e.Oid, oid)
			}
			entries = append(entries, &object.TreeEntry{Name: e.Name, Hash: oid, Mode: filemode.FileMode(e.Filemode)})
			continue
		}
		fmt.Fprintf(os.Stderr, "\x1b[2K\rskip: %s(%s: %s) \n", path.Join(parent, e.Name), e.Type(), hex.EncodeToString(e.Oid))
	}
	return ur.WriteEncoded(&object.Tree{Entries: entries}, m.squeeze, m.modification)
}

func (m *Migrator) migrateCommits(ctx context.Context, ur *backend.Unpacker) error {
	commits, err := m.commitsToMigrate(ctx)
	if err != nil {
		return fmt.Errorf("commits to migrate error: %w", err)
	}
	bar := NewBar(tr.W("Rewrite commits"), len(commits), m.stepCurrent, m.stepEnd, m.verbose)
	m.stepCurrent++
	m.DbgPrint("commits: %v", len(commits))
	for _, oid := range commits {
		oc, err := m.gitODB.Commit(oid)
		if err != nil {
			return err
		}
		var newTree plumbing.Hash
		var ok bool
		if newTree, ok = m.uncacheMD(oc.TreeID); !ok {
			if newTree, err = m.migrateTrees(ctx, ur, oc.TreeID, ""); err != nil {
				return err
			}
			m.cacheMD(oc.TreeID, newTree)
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
		parents := make([]plumbing.Hash, 0, len(oc.ParentIDs))
		for _, sha1Parent := range oc.ParentIDs {
			rewrittenParent, ok := m.uncacheMD(sha1Parent)
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

			parents = append(parents, rewrittenParent)
		}

		// Construct a new commit using the original header information,
		// but the rewritten set of parents as well as root tree.
		nc := &object.Commit{
			Message: oc.Message,
			Parents: parents,
			Tree:    newTree,
		}
		nc.Author.Decode([]byte(oc.Author))
		nc.Committer.Decode([]byte(oc.Committer))
		for _, e := range oc.ExtraHeaders {
			nc.ExtraHeaders = append(nc.ExtraHeaders, &object.ExtraHeader{K: e.K, V: e.V})
		}

		var newOID plumbing.Hash
		if newOID, err = ur.WriteEncoded(nc, m.squeeze, m.modification); err != nil {
			return err
		}
		// Cache that commit so that we can reassign children of this
		// commit.
		m.cacheMD(oid, newOID)
		bar.Add(1)
	}
	bar.Done()
	return nil
}

// refsToMigrate returns a list of references to migrate, or an error if loading
// those references failed.
func (r *Migrator) refsToMigrate(ctx context.Context) ([]*git.Reference, error) {
	refs, err := git.ParseReferences(ctx, r.from, git.OrderNone)
	if err != nil {
		return nil, err
	}

	var local []*git.Reference
	for _, ref := range refs {
		if strings.HasPrefix(ref.Name, "refs/remotes/") {
			continue
		}

		local = append(local, ref)
	}

	return local, nil
}

func (m *Migrator) encodeTag(ur *backend.Unpacker, tag *gitobj.Tag, oid plumbing.Hash) (plumbing.Hash, error) {
	signature := git.SignatureFromLine(tag.Tagger)
	newTag, err := ur.WriteEncoded(&object.Tag{
		Object:     oid,
		ObjectType: object.ObjectTypeFromString(tag.ObjectType.String()),
		Name:       tag.Name,
		Tagger: object.Signature{
			Name:  signature.Name,
			Email: signature.Email,
			When:  signature.When,
		},

		Content: tag.Message,
	}, m.squeeze, m.modification)

	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not rewrite tag: %s", tag.Name)
	}
	return newTag, nil
}

func (m *Migrator) rewriteTag(ur *backend.Unpacker, oid []byte) (plumbing.Hash, error) {
	tag, err := m.gitODB.Tag(oid)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	if tag.ObjectType == gitobj.TagObjectType {
		newTag, err := m.rewriteTag(ur, tag.Object)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		return m.encodeTag(ur, tag, newTag)

	}
	if tag.ObjectType == gitobj.CommitObjectType {
		if to, ok := m.uncacheMD(tag.Object); ok {
			return m.encodeTag(ur, tag, to)
		}
	}
	return plumbing.ZeroHash, nil
}

func (r *Migrator) rewriteOneRef(ur *backend.Unpacker, ref *git.Reference) (plumbing.Hash, error) {
	oid, err := hex.DecodeString(ref.Hash)
	if err != nil {
		return plumbing.ZeroHash, fmt.Errorf("could not decode: '%s'", ref.Hash)
	}
	if newOID, ok := r.uncacheMD(oid); ok {
		return newOID, nil
	}
	if ref.ObjectType == git.CommitObject {
		// BUGS: We have completed the conversion of all commits
		return plumbing.ZeroHash, nil
	}
	return r.rewriteTag(ur, oid)
}

func (m *Migrator) rewriteRefs(ctx context.Context, ur *backend.Unpacker) error {
	refs, err := m.refsToMigrate(ctx)
	if err != nil {
		return err
	}
	bar := NewBar(tr.W("Rewrite references"), len(refs), m.stepCurrent, m.stepEnd, m.verbose)
	rdb := m.r.RDB()
	m.stepCurrent++
	var oid plumbing.Hash
	for _, ref := range refs {
		if oid, err = m.rewriteOneRef(ur, ref); err != nil {
			return fmt.Errorf("rewrite one ref '%s' error: %v", ref.Name, err)
		}
		if oid.IsZero() {
			continue
		}
		if err := rdb.ReferenceUpdate(plumbing.NewHashReference(plumbing.ReferenceName(ref.Name), oid), nil); err != nil {
			return fmt.Errorf("zeta update-ref '%s' error: %v", ref.Name, err)
		}
		bar.Add(1)
	}
	if err := rdb.ReferenceUpdate(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.ReferenceName(m.current)), nil); err != nil {
		return err
	}
	bar.Done()
	return nil
}

func (m *Migrator) migrateMetadata(ctx context.Context) error {
	ur, err := m.r.ODB().NewUnpacker(0, true)
	if err != nil {
		return err
	}
	defer ur.Close()
	if err := m.migrateCommits(ctx, ur); err != nil {
		return err
	}
	if err := m.rewriteRefs(ctx, ur); err != nil {
		return err
	}
	return ur.Preserve()
}

func (m *Migrator) Execute(ctx context.Context) error {
	if err := m.migrateBlobs(ctx); err != nil {
		return err
	}
	if err := m.migrateMetadata(ctx); err != nil {
		return err
	}
	return m.cleanup(ctx)
}

func (m *Migrator) checkout(ctx context.Context) error {
	w := m.r.Worktree()
	current, err := m.r.Current()
	if err != nil {
		return err
	}
	return w.Checkout(ctx, &zeta.CheckoutOptions{Branch: current.Name(), Force: true, First: true})
}

func (m *Migrator) cleanup(ctx context.Context) error {
	if err := m.r.Gc(ctx, &zeta.GcOptions{Prune: time.Hour * 24 * 365}); err != nil {
		return err
	}
	diskSize, err := strengthen.Du(m.r.ODB().Root())
	if err != nil {
		return fmt.Errorf("du repo size error: %w", err)
	}
	if err := m.r.ODB().Reload(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "\x1b[38;2;72;198;239m[%d/%d]\x1b[0m %s: \x1b[38;2;32;225;215m%s\x1b[0m %s: \x1b[38;2;72;198;239m%s\x1b[0m\n",
		m.stepCurrent, m.stepEnd, tr.W("Repository"), m.to, tr.W("size"), strengthen.FormatSize(diskSize))
	if err := m.checkout(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "zeta reset error: %s\n", err)
		return err
	}
	return nil
}
