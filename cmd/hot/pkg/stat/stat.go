package stat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/deflect"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/stats"
	"github.com/antgroup/hugescm/modules/strengthen"
)

var (
	emailRegex = regexp.MustCompile(`^[A-Za-z\d]+([-_.][A-Za-z\d]+)*@([A-Za-z\d]+[-.])+[A-Za-z\d]{2,4}$`)
)

type StatOptions struct {
	RepoPath string
	Limit    int64
}

type Values map[string]string

// checkState describes the outcome of a single check.
type checkState int

const (
	stateOK checkState = iota
	stateWarn
	stateError
	stateInfo
)

// checkItem is a single key/value style entry rendered inside a card.
type checkItem struct {
	Label string
	Value string
	State checkState
	Hint  string
}

// repoSnapshot holds the structured data the dashboard wants to display.
// Rendering is delegated to render.go so the two layers stay decoupled, and
// every field here is read at least once by the renderer — keep it that way.
type repoSnapshot struct {
	RepoPath   string
	GitVersion string

	Identity   []checkItem
	Remote     []checkItem
	Repository []checkItem
	References []checkItem
	Storage    storageSummary
	LFS        *lfsSummary

	// scoring inputs
	Sparse         bool
	Partial        bool
	HasHooks       bool
	UnsafeURL      bool
	OversizedCount int
}

// storageSummary mirrors the subset of stats.Stat the renderer needs.
type storageSummary struct {
	DiskSize     int64
	LooseCount   uint64
	LooseSize    uint64
	PackCount    uint64
	PackSize     uint64
	RecentSize   uint64
	StaleSize    uint64
	KeepSize     uint64
	GarbageCount uint32
	GarbageSize  int64
}

type lfsSummary struct {
	Count uint64
	Size  uint64
}

func listConfig(ctx context.Context, repoPath string) (Values, error) {
	var stderr strings.Builder
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Environ:  os.Environ(),
		RepoPath: repoPath,
		Stderr:   &stderr,
	}, "git", "config", "list", "-z")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer stdout.Close() // nolint
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	defer cmd.Wait() // nolint
	vs := make(Values)
	br := bufio.NewReader(stdout)
	for {
		line, err := br.ReadString(0)
		if err != nil && err != io.EOF {
			return nil, err
		}
		// line including '\n' always >= 1
		if len(line) == 0 {
			break
		}
		line = line[0 : len(line)-1]
		k, v, ok := strings.Cut(line, "\n")
		if !ok {

			continue
		}
		vs[strings.ToLower(k)] = v
	}
	return vs, nil
}

func collectIdentity(vs Values, snap *repoSnapshot) {
	if name, ok := vs["user.name"]; ok {
		snap.Identity = append(snap.Identity, checkItem{Label: "user.name", Value: name, State: stateOK})
	} else {
		snap.Identity = append(snap.Identity, checkItem{Label: "user.name", Value: "<unset>", State: stateError, Hint: "git config --global user.name <name>"})
	}
	email, ok := vs["user.email"]
	if !ok {
		snap.Identity = append(snap.Identity, checkItem{Label: "user.email", Value: "<unset>", State: stateError, Hint: "git config --global user.email <addr>"})
		return
	}
	if !emailRegex.MatchString(email) {
		snap.Identity = append(snap.Identity, checkItem{Label: "user.email", Value: email, State: stateError, Hint: "invalid email format"})
		return
	}
	snap.Identity = append(snap.Identity, checkItem{Label: "user.email", Value: email, State: stateOK})
}

func safePassword(s string) string {
	if len(s) < 5 {
		return strings.Repeat("x", 5)
	}
	return s[0:2] + strings.Repeat("x", len(s)-2)
}

func collectRemote(vs Values, snap *repoSnapshot) {
	remote, ok := vs["remote.origin.url"]
	if !ok {
		snap.Remote = append(snap.Remote, checkItem{Label: "origin", Value: "<none>", State: stateInfo})
		return
	}
	u, err := url.Parse(remote)
	if err != nil {
		if git.MatchesScpLike(remote) {
			snap.Remote = append(snap.Remote, checkItem{Label: "origin", Value: remote, State: stateOK})
			return
		}
		snap.Remote = append(snap.Remote, checkItem{Label: "origin", Value: remote, State: stateError, Hint: fmt.Sprintf("parse error: %v", err)})
		return
	}
	username := u.User.Username()
	password, ok := u.User.Password()
	if ok {
		newPassword := safePassword(password)
		u.User = url.UserPassword(username, newPassword)
		snap.UnsafeURL = true
		snap.Remote = append(snap.Remote, checkItem{Label: "origin", Value: u.String(), State: stateError, Hint: "remote URL embeds a password — strip it"})
		return
	}
	snap.Remote = append(snap.Remote, checkItem{Label: "origin", Value: u.String(), State: stateOK})
}

func collectPartial(vs Values, snap *repoSnapshot) {
	if v, ok := vs["core.sparsecheckout"]; ok && strings.EqualFold(v, "true") {
		snap.Sparse = true
		snap.Repository = append(snap.Repository, checkItem{Label: "sparse checkout", Value: "enabled", State: stateInfo})
	}
	if v, ok := vs["remote.origin.promisor"]; ok && strings.EqualFold(v, "true") {
		snap.Partial = true
		snap.Repository = append(snap.Repository, checkItem{Label: "partial clone", Value: "enabled", State: stateInfo})
	}
}

func parseShallowCommit(repoPath string) string {
	p := filepath.Join(repoPath, "shallow")
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// Stat collects everything we know about the repository and pretty-prints it.
func Stat(ctx context.Context, o *StatOptions) error {
	snap := &repoSnapshot{RepoPath: o.RepoPath}
	if version, err := git.VersionDetect(); err == nil {
		snap.GitVersion = version.String()
	}

	vs, err := listConfig(ctx, o.RepoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list git config error: %v\n", err)
		return err
	}

	collectIdentity(vs, snap)
	collectRemote(vs, snap)
	collectPartial(vs, snap)

	shaFormat, refFormat := git.ExtensionsFormat(o.RepoPath)
	snap.Repository = append(snap.Repository, checkItem{Label: "object format", Value: shaFormat.String(), State: stateOK})
	snap.Repository = append(snap.Repository, checkItem{Label: "ref backend", Value: refFormat, State: stateOK})

	if defaultBranch, ok := vs["init.defaultbranch"]; ok {
		snap.Repository = append(snap.Repository, checkItem{Label: "init.defaultBranch", Value: defaultBranch, State: stateOK})
	}
	if defaultObjectFormat, ok := vs["init.defaultobjectformat"]; ok {
		snap.Repository = append(snap.Repository, checkItem{Label: "init.defaultObjectFormat", Value: defaultObjectFormat, State: stateOK})
	}
	if defaultRefFormat, ok := vs["init.defaultrefformat"]; ok {
		snap.Repository = append(snap.Repository, checkItem{Label: "init.defaultRefFormat", Value: defaultRefFormat, State: stateOK})
	}
	if hooksPath, ok := vs["core.hookspath"]; ok {
		snap.HasHooks = true
		snap.Repository = append(snap.Repository, checkItem{Label: "core.hooksPath", Value: hooksPath, State: stateWarn, Hint: "may affect Git LFS"})
	}

	if shallow := parseShallowCommit(o.RepoPath); shallow != "" {
		snap.Repository = append(snap.Repository, checkItem{Label: "shallow @", Value: shallow, State: stateInfo})
	}

	if current, oid, err := git.RevParseCurrent(ctx, nil, o.RepoPath); err == nil {
		refname := git.ReferenceName(current)
		if refname.IsBranch() {
			snap.Repository = append(snap.Repository, checkItem{
				Label: "HEAD",
				Value: fmt.Sprintf("%s @ %s", refname.BranchName(), shortOID(oid)),
				State: stateOK,
			})
		} else {
			snap.Repository = append(snap.Repository, checkItem{
				Label: "HEAD",
				Value: fmt.Sprintf("detached @ %s", shortOID(oid)),
				State: stateWarn,
			})
		}
	}

	si, err := stats.Status(ctx, o.RepoPath, refFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "status error: %v\n", err)
		return err
	}

	recentLoose := si.LooseObjects.Size - si.LooseObjects.StaleSize
	recentPack := si.Packfiles.Size - si.Packfiles.CruftSize
	snap.Storage = storageSummary{
		LooseCount: si.LooseObjects.Count,
		LooseSize:  si.LooseObjects.Size,
		PackCount:  si.Packfiles.Count,
		PackSize:   si.Packfiles.Size,
		RecentSize: recentLoose + recentPack,
		StaleSize:  si.LooseObjects.StaleSize + si.Packfiles.CruftSize,
		KeepSize:   si.Packfiles.KeepSize,
	}

	if si.References.ReferenceBackendName == "reftable" {
		snap.References = append(snap.References, checkItem{Label: "reftable tables", Value: strconv.Itoa(len(si.References.ReftableTables)), State: stateOK})
	} else {
		snap.References = append(snap.References, checkItem{Label: "loose refs", Value: strconv.FormatUint(si.References.LooseReferencesCount, 10), State: stateOK})
		snap.References = append(snap.References, checkItem{Label: "packed-refs", Value: strengthen.FormatSizeU(si.References.PackedReferencesSize), State: stateOK})
	}

	if si.LFS.Count != 0 {
		snap.LFS = &lfsSummary{Count: si.LFS.Count, Size: si.LFS.Size}
	}

	careful := snap.Sparse || snap.Partial

	objects := make(map[string]int64)
	au := deflect.NewAuditor(o.RepoPath, shaFormat, &deflect.Option{
		Limit: o.Limit,
		OnOversized: func(oid string, size int64) error {
			objects[oid] = size
			return nil
		},
	})
	if err := au.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "hot stat: check large file: %v\n", err)
		return err
	}
	snap.Storage.DiskSize = au.Size()
	snap.Storage.GarbageCount = au.GarbageCount()
	snap.Storage.GarbageSize = au.GarbageSize()
	snap.OversizedCount = len(objects)

	// Render the dashboard first so users see the high-signal view before the
	// (potentially slower) huge-object listing.
	renderSnapshot(snap)

	if !careful {
		_ = showHugeObjects(ctx, o.RepoPath, objects, false)
	}
	return nil
}

func shortOID(oid string) string {
	if len(oid) > 9 {
		return oid[:9]
	}
	return oid
}
