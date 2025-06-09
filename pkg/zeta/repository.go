// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/vfs"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/config"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/modules/zeta/reflog"
	"github.com/antgroup/hugescm/modules/zeta/refs"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/zeta/odb"

	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/transport/client"
)

const (
	// ZetaDirName this is a special folder where all the zeta stuff is.
	ZetaDirName = ".zeta"
)

var (
	warningFs = map[string]bool{
		"nfs":  true,
		"ceph": true,
		"smb":  true,
		"smd2": true,
	}
)

type StringArray []string

func valuesMapArray(values []string) map[string]StringArray {
	m := make(map[string]StringArray)
	for _, v := range values {
		i := strings.IndexByte(v, '=')
		if i == -1 {
			continue
		}
		k := strings.ToLower(v[:i])
		v := v[i+1:]
		if _, ok := m[k]; ok {
			m[k] = append(m[k], v)
			continue
		}
		m[k] = []string{v}
	}
	return m
}

func getStringFromValues(k string, values map[string]StringArray) (string, bool) {
	if len(values) == 0 {
		return "", false
	}
	sa, ok := values[strings.ToLower(k)]
	if !ok {
		return "", false
	}
	if len(sa) == 0 {
		return "", true
	}
	return sa[len(sa)-1], true
}

func getStringsFromValues(k string, values map[string]StringArray) ([]string, bool) {
	if len(values) == 0 {
		return nil, false
	}
	sa, ok := values[strings.ToLower(k)]
	if !ok {
		return nil, false
	}
	return sa, true
}

func getFromValueOrEnv(k, e string, values map[string]StringArray) (string, bool) {
	if s, ok := getStringFromValues(k, values); ok {
		return s, true
	}
	return os.LookupEnv(e)
}

type NewOptions struct {
	Remote      string
	Branch      string
	TagName     string
	Commit      string
	Refname     string
	Destination string
	Depth       int
	SparseDirs  []string
	Snapshot    bool
	SizeLimit   int64
	Values      []string
	One         bool
	Quiet       bool
	Verbose     bool
}

const (
	dot      = "."
	dotDot   = ".."
	pathRoot = "/"
)

func (opts *NewOptions) tidySparse() {
	if len(opts.SparseDirs) == 0 {
		return
	}
	sparseDirs := make([]string, 0, len(opts.SparseDirs))
	for _, s := range opts.SparseDirs {
		if filepath.IsAbs(s) {
			_, _ = term.Fprintf(os.Stderr, "\x1b[01;33m%s: \x1b[0;33m'%s' %s\x1b[0m\n", W("WARNING"), s, W("is an absolute path and cannot be set as a sparse dir."))
			continue
		}
		p := path.Clean(s)
		if p == dot || p == dotDot || p == pathRoot {
			continue
		}
		sparseDirs = append(sparseDirs, p)
	}
	opts.SparseDirs = sparseDirs
}

func (opts *NewOptions) Validate() error {
	opts.tidySparse()
	if len(opts.Branch) != 0 && !plumbing.ValidateBranchName([]byte(opts.Branch)) {
		die("'%s' is not a valid branch name", opts.Branch)
		return &plumbing.ErrBadReferenceName{Name: opts.Branch}
	}
	if len(opts.Commit) != 0 && !plumbing.ValidateHashHex(opts.Commit) {
		fmt.Fprintf(os.Stderr, "mistake commit hex string: '%s'\n", opts.Commit)
		return errors.New("mistake commit hex string")
	}
	return nil
}

type Repository struct {
	*config.Config
	refs.Backend
	odb               *odb.ODB
	rdb               *reflog.DB
	baseDir           string // worktree
	zetaDir           string
	missingNotFailure bool
	values            map[string]StringArray
	quiet             bool
	verbose           bool
}

func parseInsecureSkipTLS(cfg *config.Config, values map[string]StringArray) bool {
	if sslVerify, ok := getStringFromValues("http.sslVerify", values); ok {
		return !strengthen.SimpleAtob(sslVerify, true) // sslVerify == false skip TLS check
	}
	if noSSLVerify, ok := os.LookupEnv(ENV_ZETA_SSL_NO_VERIFY); ok {
		return strengthen.SimpleAtob(noSSLVerify, false) // ZETA_SSL_NO_VERIFY == TRUE skip TLS check
	}
	return cfg.HTTP.SSLVerify.False()
}

func parseExtraHeader(cfg *config.Config, values map[string]StringArray) []string {
	extraHeader := make([]string, 0, len(cfg.HTTP.ExtraHeader))
	if sa, ok := getStringsFromValues("http.extraHeader", values); ok {
		extraHeader = append(extraHeader, sa...)
	}
	extraHeader = append(extraHeader, cfg.HTTP.ExtraHeader...)
	return extraHeader
}

func parseExtraEnv(cfg *config.Config, values map[string]StringArray) []string {
	extraEnv := make([]string, 0, len(cfg.SSH.ExtraEnv))
	if sa, ok := getStringsFromValues("ssh.extraEnv", values); ok {
		extraEnv = append(extraEnv, sa...)
	}
	extraEnv = append(extraEnv, cfg.SSH.ExtraEnv...)
	return extraEnv
}

func parseSharingRoot(cfg *config.Config, values map[string]StringArray) (string, bool) {
	if sharingRoot, ok := getStringFromValues("core.sharingRoot", values); ok && len(sharingRoot) > 0 && filepath.IsAbs(sharingRoot) {
		return sharingRoot, true
	}
	if sharingRoot, ok := os.LookupEnv(ENV_ZETA_CORE_SHARING_ROOT); ok && len(sharingRoot) > 0 && filepath.IsAbs(sharingRoot) {
		return sharingRoot, true
	}
	if len(cfg.Core.SharingRoot) > 0 && filepath.IsAbs(cfg.Core.SharingRoot) {
		return cfg.Core.SharingRoot, true
	}
	return "", false
}

// create a new repo using zeta checkout command
func New(ctx context.Context, opts *NewOptions) (*Repository, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}

	// New config from global config
	cfg, err := config.LoadBaseline()
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve global config error: %v\n", err)
		return nil, err
	}
	values := valuesMapArray(opts.Values)
	target := plumbing.NewHash(opts.Commit)
	endpoint, err := transport.NewEndpoint(opts.Remote, &transport.Options{
		InsecureSkipTLS: parseInsecureSkipTLS(cfg, values),
		ExtraHeader:     parseExtraHeader(cfg, values),
		ExtraEnv:        parseExtraEnv(cfg, values),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad remote: %v\n", err)
		return nil, err
	}
	repoName := strings.TrimSuffix(filepath.Base(strings.TrimSuffix(endpoint.Path, "/")), ".zeta")
	destination, exists, err := checkDestination(repoName, opts.Destination, true)
	if err != nil {
		return nil, err
	}
	zetaDir := filepath.Join(destination, ".zeta")
	var checkoutSuccess bool
	defer func() {
		if checkoutSuccess {
			return
		}
		if exists {
			_ = os.RemoveAll(zetaDir)
			return
		}
		_ = os.RemoveAll(destination)
	}()

	tr, err := client.NewTransport(ctx, endpoint, transport.DOWNLOAD, opts.Verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect remote: %v\n", err)
		return nil, err
	}
	// In particular, when the commit is not empty,
	// we need to get the commit of the mainline to ensure that our expectations are correct,
	// that is, to create a new branch based on the commit.
	refname := plumbing.HEAD
	switch {
	case len(opts.TagName) != 0:
		refname = plumbing.NewTagReferenceName(opts.TagName)
	case len(opts.Commit) != 0:
		// NO thing to do
		// IF --commit we fetch HEAD reference
	case len(opts.Branch) != 0:
		refname = plumbing.NewBranchReferenceName(opts.Branch)
	case strings.HasPrefix(opts.Refname, plumbing.ReferencePrefix):
		refname = plumbing.ReferenceName(opts.Refname)
		// compatible
		switch {
		case refname.IsBranch():
			opts.Branch = refname.BranchName()
		case refname.IsTag():
			opts.TagName = refname.TagName()
		}
	default:
	}
	ref, err := tr.FetchReference(ctx, refname)
	if err != nil {
		die_error("Fetch reference '%s': %v", refname, err)
		return nil, err
	}
	if target.IsZero() {
		target = plumbing.NewHash(ref.Hash)
	}

	odbOpts := make([]backend.Option, 0, 2)
	odbOpts = append(odbOpts, backend.WithCompressionALGO(ref.CompressionALGO), backend.WithEnableLRU(true))
	var sharingRoot string
	var sharingSet bool
	if sharingRoot, sharingSet = parseSharingRoot(cfg, values); sharingSet {
		odbOpts = append(odbOpts, backend.WithSharingRoot(sharingRoot))
	}
	odb, err := odb.NewODB(zetaDir, odbOpts...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "new objects database error: %v\n", err)
		return nil, err
	}
	defer func() {
		if checkoutSuccess {
			return
		}
		_ = odb.Close()
	}()

	newConfig := &config.Config{
		Core: config.Core{
			Remote:          endpoint.String(),
			SparseDirs:      opts.SparseDirs,
			Snapshot:        opts.Snapshot,
			CompressionALGO: ref.CompressionALGO,
		},
	}
	// Flush sharingRoot
	if sharingSet {
		newConfig.Core.SharingRoot = sharingRoot
	}
	// Write new config to disk
	if err := config.Encode(zetaDir, newConfig); err != nil {
		fmt.Fprintf(os.Stderr, "encode config error: %v\n", err)
		return nil, err
	}
	// Use local config overwrite global config
	cfg.Overwrite(newConfig)
	r := &Repository{
		Config:  cfg,
		odb:     odb,
		Backend: refs.NewBackend(zetaDir),
		rdb:     reflog.NewDB(zetaDir),
		zetaDir: zetaDir,
		baseDir: destination,
		values:  values,
		quiet:   opts.Quiet,
		verbose: opts.Verbose,
	}
	if opts.SizeLimit != -1 {
		r.missingNotFailure = true
	}
	fetchOpts := &FetchOptions{
		Target:    target,
		Deepen:    opts.Depth,         // deepen: commit depth
		Depth:     transport.AnyDepth, // tree depth = -1: all tree
		SizeLimit: opts.SizeLimit,
	}
	if opts.One {
		fetchOpts.SkipLarges = true
		r.missingNotFailure = true
	}
	fmt.Fprintf(os.Stderr, W("Checkout into '%s'...\n"), filepath.Base(destination))
	ds, err := strengthen.GetDiskFreeSpaceEx(zetaDir)
	if err != nil {
		die("check disk free space error: %v", err)
		return nil, err
	}
	if warningFs[strings.ToLower(ds.FS)] {
		fsName := ds.FS
		if term.StderrLevel != term.LevelNone {
			fsName = "\x1b[01;33m" + ds.FS + "\x1b[0m"
		}
		warn("The repository filesystem is '%s', which may affect zeta's operation.", fsName)
	}
	fmt.Fprintf(os.Stderr, "Checkout from '%s' to %s... sparse-checkout: %v, snapshot: %v\n", r.cleanedRemote(), target.String()[0:8], len(r.Core.SparseDirs) != 0, opts.Snapshot)
	if len(r.Core.SparseDirs) != 0 {
		fmt.Fprintf(os.Stderr, "")
	}
	if err := r.fetch(ctx, tr, fetchOpts); err != nil {
		fmt.Fprintf(os.Stderr, "fetch objects error: %v\n", err)
		return nil, err
	}
	commit := target
	if len(ref.Peeled) != 0 {
		commit = plumbing.NewHash(ref.Peeled)
	}
	if err := r.storeShallow(ctx, commit); err != nil {
		die_error("unable record shallow %v", err)
		return nil, err
	}

	switch {
	case ref.Name.IsBranch() && target == plumbing.NewHash(ref.Hash):
		originBranch := plumbing.NewRemoteReferenceName(plumbing.Origin, ref.Name.BranchName())
		if err := r.ReferenceUpdate(plumbing.NewHashReference(originBranch, target), nil); err != nil {
			fmt.Fprintf(os.Stderr, "update-ref '%s' error: %v\n", originBranch, err)
			return nil, err
		}
	case ref.Name.IsTag():
		originTag := plumbing.NewTagReferenceName(ref.Name.TagName())
		if err := r.ReferenceUpdate(plumbing.NewHashReference(originTag, target), nil); err != nil {
			fmt.Fprintf(os.Stderr, "update-ref '%s' error: %v\n", originTag, err)
			return nil, err
		}
	default:
	}
	branchSwitched := opts.Branch
	if len(opts.Commit) == 0 && len(branchSwitched) == 0 && len(opts.TagName) == 0 {
		branchSwitched = ref.Name.Short() // Switch to HEAD
		r.DbgPrint("switch to %s", branchSwitched)
	}
	so := &SwitchOptions{Force: true, ForceCreate: true, firstSwitch: true, one: opts.One}
	if len(branchSwitched) != 0 {
		if err = r.SwitchNewBranch(ctx, branchSwitched, commit.String(), so); err != nil {
			return nil, err
		}
		checkoutSuccess = true
		return r, nil
	}
	if err = r.SwitchDetach(ctx, commit.String(), so); err != nil {
		return nil, err
	}
	checkoutSuccess = true
	return r, nil
}

func (r *Repository) storeShallow(ctx context.Context, commit plumbing.Hash) error {
	our := commit
	current := our
	for {
		cc, err := r.odb.Commit(ctx, current)
		if plumbing.IsNoSuchObject(err) {
			break
		}
		if err != nil {
			return err
		}
		our = current
		if len(cc.Parents) == 0 {
			return nil
		}
		current = cc.Parents[0]
	}
	return r.odb.Shallow(our)
}

type OpenOptions struct {
	Worktree string
	Quiet    bool
	Verbose  bool
	Values   []string
}

func Open(ctx context.Context, opts *OpenOptions) (*Repository, error) {
	worktree, zetaDir, err := FindZetaDir(opts.Worktree)
	if err != nil {
		die_error("%v", err)
		return nil, err
	}
	cfg, err := config.Load(zetaDir)
	if err != nil {
		die_error("%v", err)
		return nil, err
	}
	odbOpts := make([]backend.Option, 0, 2)
	odbOpts = append(odbOpts, backend.WithCompressionALGO(cfg.Core.CompressionALGO), backend.WithEnableLRU(true))
	values := valuesMapArray(opts.Values)

	if sharingRoot, sharingSet := parseSharingRoot(cfg, values); sharingSet {
		odbOpts = append(odbOpts, backend.WithSharingRoot(sharingRoot))
	}
	odb, err := odb.NewODB(zetaDir, odbOpts...)
	if err != nil {
		die("open odb: %v", err)
		return nil, err
	}
	r := &Repository{
		Config:  cfg,
		zetaDir: zetaDir,
		baseDir: worktree,
		odb:     odb,
		Backend: refs.NewBackend(zetaDir),
		rdb:     reflog.NewDB(zetaDir),
		values:  values,
		quiet:   opts.Quiet,
		verbose: opts.Verbose,
	}
	return r, nil
}

type InitOptions struct {
	Branch    string
	Worktree  string
	MustEmpty bool
	Quiet     bool
	Verbose   bool
	Values    []string
}

func Init(ctx context.Context, opts *InitOptions) (*Repository, error) {
	destination, _, err := checkDestination("", opts.Worktree, opts.MustEmpty)
	if err != nil {
		return nil, err
	}
	zetaDir := filepath.Join(destination, ".zeta")

	// New config from global
	cfg, err := config.LoadBaseline()
	if err != nil {
		die("local config: %v", err)
		return nil, err
	}

	cfg.Core.CompressionALGO = odb.DefaultCompressionALGO

	odbOpts := make([]backend.Option, 0, 2)
	odbOpts = append(odbOpts, backend.WithCompressionALGO(odb.DefaultCompressionALGO), backend.WithEnableLRU(true))
	values := valuesMapArray(opts.Values)
	var sharingRoot string
	var sharingSet bool
	if sharingRoot, sharingSet = parseSharingRoot(cfg, values); sharingSet {
		odbOpts = append(odbOpts, backend.WithSharingRoot(sharingRoot))
	}
	newConfig := &config.Config{
		Core: config.Core{
			CompressionALGO: odb.DefaultCompressionALGO,
		},
	}
	if sharingSet {
		newConfig.Core.SharingRoot = sharingRoot
	}
	// Write new config to disk
	if err := config.Encode(zetaDir, newConfig); err != nil {
		die("encode config: %v")
		return nil, err
	}

	o, err := odb.NewODB(zetaDir, odbOpts...)
	if err != nil {
		die("new odb: %v", err)
		return nil, err
	}

	r := &Repository{
		Config:  cfg,
		odb:     o,
		Backend: refs.NewBackend(zetaDir),
		rdb:     reflog.NewDB(zetaDir),
		zetaDir: zetaDir,
		values:  values,
		baseDir: destination,
		quiet:   opts.Quiet,
		verbose: opts.Verbose,
	}
	if len(opts.Branch) != 0 {
		branchName := plumbing.NewBranchReferenceName(opts.Branch)
		head := plumbing.NewSymbolicReference(plumbing.HEAD, branchName)
		if err := r.ReferenceUpdate(head, nil); err != nil {
			die_error("update HEAD to %s error: %v", branchName, err)
		}
	}
	return r, nil
}

func (r *Repository) Worktree() *Worktree {
	return &Worktree{Repository: r, fs: vfs.NewVFS(r.baseDir)}
}

func (r *Repository) getFromValueOrEnv(k, e string) (string, bool) {
	return getFromValueOrEnv(k, e, r.values)
}

func (r *Repository) getIntFromValueOrEnv(k, e string) (int, bool) {
	a, ok := getFromValueOrEnv(k, e, r.values)
	if !ok {
		return 0, false
	}
	i, err := strconv.Atoi(a)
	if err != nil {
		return 0, false
	}
	return i, true
}

func (r *Repository) getSizeFromValueOrEnv(k, e string) (int64, bool) {
	a, ok := getFromValueOrEnv(k, e, r.values)
	if !ok {
		return 0, false
	}
	if size, err := strengthen.ParseSize(a); err == nil {
		return size, true
	}
	return 0, false
}

func (r *Repository) Accelerator() config.Accelerator {
	if s, ok := r.getFromValueOrEnv("core.accelerator", ENV_ZETA_CORE_ACCELERATOR); ok {
		return config.Accelerator(s)
	}
	return r.Core.Accelerator
}

func (r *Repository) IsExtreme() bool {
	if s, ok := r.getFromValueOrEnv("core.optimizeStrategy", ENV_ZETA_CORE_OPTIMIZE_STRATEGY); ok {
		return config.Strategy(s) == config.STRATEGY_EXTREME
	}
	return r.Core.IsExtreme()
}

func (r *Repository) ConcurrentTransfers() int {
	if i, ok := r.getIntFromValueOrEnv("core.concurrenttransfers", ENV_ZETA_CORE_CONCURRENT_TRANSFERS); ok && i > 0 && i < 50 {
		return i
	}
	if r.Core.ConcurrentTransfers > 0 && r.Core.ConcurrentTransfers < 50 {
		return r.Core.ConcurrentTransfers
	}
	return 1
}

func (r *Repository) authorName() string {
	if s, ok := r.getFromValueOrEnv("user.name", ENV_ZETA_AUTHOR_NAME); ok && len(s) > 0 {
		return stringNoCRUD(s)
	}
	return stringNoCRUD(r.User.Name)
}

func (r *Repository) authorEmail() string {
	if s, ok := r.getFromValueOrEnv("user.email", ENV_ZETA_AUTHOR_EMAIL); ok && len(s) > 0 {
		return stringNoCRUD(s)
	}
	return stringNoCRUD(r.User.Email)
}

func (r *Repository) committerName() string {
	if s, ok := r.getFromValueOrEnv("user.name", ENV_ZETA_COMMITTER_NAME); ok && len(s) > 0 {
		return stringNoCRUD(s)
	}
	return stringNoCRUD(r.User.Name)
}

func (r *Repository) committerEmail() string {
	if s, ok := r.getFromValueOrEnv("user.email", ENV_ZETA_COMMITTER_EMAIL); ok && len(s) > 0 {
		return stringNoCRUD(s)
	}
	return stringNoCRUD(r.User.Email)
}

// ZETA_CORE_PROMISOR=0 disable promisor
func (r *Repository) promisorEnabled() bool {
	return strengthen.SimpleAtob(os.Getenv(ENV_ZETA_CORE_PROMISOR), true)
}

func (r *Repository) maxEntries() int {
	if maxEntries, ok := r.getIntFromValueOrEnv("transport.maxEntries", ENV_ZETA_TRANSPORT_MAX_ENTRIES); ok && maxEntries > 0 {
		return maxEntries
	}
	return r.Transport.MaxEntries
}

func (r *Repository) largeSize() int64 {
	if largeSize, ok := r.getSizeFromValueOrEnv("transport.largeSize", ENV_ZETA_TRANSPORT_LARGE_SIZE); ok && largeSize > 0 {
		return largeSize
	}
	return r.Transport.LargeSize()
}

func (r *Repository) externalProxy() string {
	if externalProxy, ok := r.getFromValueOrEnv("transport.externalProxy", ENV_ZETA_TRANSPORT_EXTERNAL_PROXY); ok && len(externalProxy) > 0 {
		return externalProxy
	}
	return r.Transport.ExternalProxy
}

func (r *Repository) NewCommitter() *object.Signature {
	return &object.Signature{
		Name:  r.committerName(),
		Email: r.committerEmail(),
		When:  time.Now(),
	}
}

func (r *Repository) coreEditor() string {
	if s, ok := r.getFromValueOrEnv("core.editor", ENV_ZETA_EDITOR); ok && len(s) > 0 {
		return s
	}
	return r.Core.Editor
}

func (r *Repository) diffAlgorithm() string {
	if a, ok := getStringFromValues("diff.algorithm", r.values); ok && len(a) > 0 {
		return a
	}
	return r.Diff.Algorithm
}

func (r *Repository) mergeConflictStyle() string {
	if conflictStyle, ok := getStringFromValues("merge.conflictStyle", r.values); ok && len(conflictStyle) > 0 {
		return conflictStyle
	}
	return r.Merge.ConflictStyle
}

func (r *Repository) Postflight(ctx context.Context) error {
	if !r.IsExtreme() {
		return nil
	}
	oids, totalSize, err := r.odb.PruneObjects(ctx, extremeSize)
	if err != nil {
		return err
	}
	if len(oids) == 0 {
		return nil
	}
	_, _ = tr.Fprintf(os.Stderr, "postflight: remove large files in extreme mode: %d, reduce: %s.", len(oids), strengthen.FormatSize(totalSize))
	return nil
}

func (r *Repository) BaseDir() string {
	return r.baseDir
}

func (r *Repository) ZetaDir() string {
	return r.zetaDir
}

func (r *Repository) Current() (*plumbing.Reference, error) {
	ref, err := r.HEAD()
	if err != nil {
		return nil, err
	}
	if ref == nil {
		return nil, plumbing.ErrReferenceNotFound
	}
	if ref.Type() == plumbing.HashReference {
		return ref, nil
	}
	return r.Reference(ref.Target())
}

func (r *Repository) ODB() *odb.ODB {
	return r.odb
}

func (r *Repository) RDB() refs.Backend {
	return r.Backend
}

func (r *Repository) ReferenceResolve(name plumbing.ReferenceName) (ref *plumbing.Reference, err error) {
	return refs.ReferenceResolve(r.Backend, name)
}

func (r *Repository) cleanedRemote() string {
	u, err := url.Parse(r.Core.Remote)
	if err != nil {
		return r.Core.Remote
	}
	switch {
	case u.Scheme == "ssh" && u.User != nil:
		u.User = url.User(u.User.Username()) // hide ssh password
	default:
		u.User = nil
	}
	return u.String()
}

func (r *Repository) newTransport(ctx context.Context, operation transport.Operation) (transport.Transport, error) {
	endpoint, err := transport.NewEndpoint(r.Core.Remote, &transport.Options{
		InsecureSkipTLS: parseInsecureSkipTLS(r.Config, r.values),
		ExtraHeader:     parseExtraHeader(r.Config, r.values),
		ExtraEnv:        parseExtraEnv(r.Config, r.values),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "bad remote: %v\n", err)
		return nil, err
	}
	t, err := client.NewTransport(ctx, endpoint, operation, r.verbose)
	if err != nil {
		fmt.Fprintf(os.Stderr, "connect remote: %v\n", err)
		return nil, err
	}
	return t, nil
}

func (r *Repository) Close() error {
	if r.odb == nil {
		return nil
	}
	return r.odb.Close()
}
