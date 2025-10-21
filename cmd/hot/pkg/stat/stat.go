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
	"strings"

	"github.com/antgroup/hugescm/cmd/hot/pkg/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/deflect"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/stats"
)

var (
	emailRegex = regexp.MustCompile(`^[A-Za-z\d]+([-_.][A-Za-z\d]+)*@([A-Za-z\d]+[-.])+[A-Za-z\d]{2,4}$`)
)

type StatOptions struct {
	RepoPath string
	Limit    int64
}

type Values map[string]string

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

func scanIdentity(vs Values) {
	if name, ok := vs["user.name"]; !ok {
		_, _ = tr.Fprintf(os.Stderr, "error: '%s' is not configured correctly\n", colorE("user.name"))
	} else {
		fmt.Fprintf(os.Stderr, "%s 'user.name' --> '%s' ✅\n", tr.W("check"), blue(name))
	}
	email, ok := vs["user.email"]
	if !ok {
		_, _ = tr.Fprintf(os.Stderr, "error: '%s' is not configured correctly\n", colorE("user.email"))
		return
	}
	if !emailRegex.MatchString(email) {
		_, _ = tr.Fprintf(os.Stderr, "error: invalid email '%s' (from user.email)\n", colorE(email))
		return
	}
	fmt.Fprintf(os.Stderr, "%s 'user.email' --> '%s' ✅\n", tr.W("check"), blue(email))
}

func safePassword(s string) string {
	if len(s) < 5 {
		return strings.Repeat("x", 5)
	}
	return s[0:2] + strings.Repeat("x", len(s)-2)
}

func checkRemote(vs Values) {
	remote, ok := vs["remote.origin.url"]
	if !ok {
		return
	}
	u, err := url.Parse(remote)
	if err != nil {
		if git.MatchesScpLike(remote) {
			fmt.Fprintf(os.Stderr, "%s %s ✅\n", tr.W("remote:"), blue(remote))
			return
		}
		fmt.Fprintf(os.Stderr, "parse remote '%s' error: %s\n", colorE(remote), err)
		return
	}
	username := u.User.Username()
	password, ok := u.User.Password()
	if ok {
		newPassword := safePassword(password)
		u.User = url.UserPassword(username, newPassword)
		_, _ = tr.Fprintf(os.Stderr, "insecure remote: remote url contains the password '%s' ❌\n", colorE(newPassword))
		fmt.Fprintf(os.Stderr, "%s %s ❌ (%s)\n", tr.W("remote:"), colorE(u.String()), tr.W("sanitized"))
		return
	}
	fmt.Fprintf(os.Stderr, "%s %s ✅\n", tr.W("remote:"), blue(u.String()))
}

func partialClone(vs Values) (sparse bool, partial bool) {
	if v, ok := vs["core.sparsecheckout"]; ok && strings.EqualFold(v, "true") {
		fmt.Fprintf(os.Stderr, "%s: %s\n", tr.W("sparse checkout"), tr.W("enabled"))
		sparse = true
	}
	if v, ok := vs["remote.origin.promisor"]; ok && strings.EqualFold(v, "true") {
		fmt.Fprintf(os.Stderr, "%s: %s\n", tr.W("partial checkout"), tr.W("enabled"))
		partial = true
	}
	return
}

func parseShallowCommit(repoPath string) string {
	p := filepath.Join(repoPath, "shallow")
	data, err := os.ReadFile(p)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func Stat(ctx context.Context, o *StatOptions) error {
	_, _ = tr.Fprintf(os.Stderr, "Location: %s\n", blue(o.RepoPath))
	if version, err := git.VersionDetect(); err == nil {
		_, _ = tr.Fprintf(os.Stderr, "Git Version: %s\n", blue(version.String()))
	}
	vs, err := listConfig(ctx, o.RepoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list git config error: %v\n", err)
		return err
	}
	scanIdentity(vs)
	shaFormat, refFormat := git.ExtensionsFormat(o.RepoPath)
	if defaultBranch, ok := vs["init.defaultbranch"]; ok {
		fmt.Fprintf(os.Stderr, "%s 'init.defaultBranch' --> '%s' ✅\n", tr.W("check"), blue(defaultBranch))
	}
	if defaultObjectFormat, ok := vs["init.defaultobjectformat"]; ok {
		fmt.Fprintf(os.Stderr, "%s 'init.defaultObjectFormat' --> '%s' ✅\n", tr.W("check"), blue(defaultObjectFormat))
	}
	if defaultRefFormat, ok := vs["init.defaultrefformat"]; ok {
		fmt.Fprintf(os.Stderr, "%s 'init.defaultRefFormat' --> '%s' ✅\n", tr.W("check"), blue(defaultRefFormat))
	}
	if hooksPath, ok := vs["core.hookspath"]; ok {
		_, _ = tr.Fprintf(os.Stderr, "warning: '%s' is set to '%s', which may affect Git LFS\n", yellow("core.hooksPath"), yellow(hooksPath))
	}
	_, _ = tr.Fprintf(os.Stderr, "Repository object format (sha format):      %s ✅\n", blue(shaFormat.String()))
	_, _ = tr.Fprintf(os.Stderr, "Repository references backend (ref format): %s ✅\n", blue(refFormat))
	checkRemote(vs)
	var careful bool
	sparse, partial := partialClone(vs)
	careful = sparse || partial
	shallow := parseShallowCommit(o.RepoPath)
	if len(shallow) != 0 {
		_, _ = tr.Fprintf(os.Stderr, "shallow clone started at: %s\n", shallow)
	}
	if current, oid, err := git.RevParseCurrent(ctx, nil, o.RepoPath); err == nil {
		refname := git.ReferenceName(current)
		if refname.IsBranch() {
			fmt.Fprintf(os.Stderr, "%s: %s (commit: %s)\n", tr.W("On branch"), blue(refname.BranchName()), green(oid[:9]))
		} else {
			fmt.Fprintf(os.Stderr, "%s %s\n", tr.W("HEAD detached at"), blue(oid))
		}

	}
	si, err := stats.Status(ctx, o.RepoPath, refFormat)
	if err != nil {
		fmt.Fprintf(os.Stderr, "status error: %v\n", err)
		return err
	}
	if si.References.ReferenceBackendName == "reftable" {
		_, _ = tr.Fprintf(os.Stdout, "references (reftable) tables total: %s\n", colorInt(len(si.References.ReftableTables)))

	} else {
		_, _ = tr.Fprintf(os.Stdout, "loose references total: %s\n", colorInt(si.References.LooseReferencesCount))
		_, _ = tr.Fprintf(os.Stdout, "packed referenes size:  %s\n", colorSizeU(si.References.PackedReferencesSize))
	}
	// The loose objects size includes objects which are older than the grace period and thus
	// stale, so we need to subtract the size of stale objects from the overall size.
	recentLooseObjectsSize := si.LooseObjects.Size - si.LooseObjects.StaleSize
	// The packfiles size includes the size of cruft packs that contain unreachable objects, so
	// we need to subtract the size of cruft packs from the overall size.
	recentPackfilesSize := si.Packfiles.Size - si.Packfiles.CruftSize
	_, _ = tr.Fprintf(os.Stdout, "loose objects total:    %s\n", colorInt(si.LooseObjects.Count))
	_, _ = tr.Fprintf(os.Stdout, "packfiles count:        %s\n", colorInt(si.Packfiles.Count))
	_, _ = tr.Fprintf(os.Stdout, "objects size:           %s\n", colorSizeU(si.LooseObjects.Size+si.Packfiles.Size))
	_, _ = tr.Fprintf(os.Stdout, "recent size:            %s\n", colorSizeU(recentLooseObjectsSize+recentPackfilesSize))
	_, _ = tr.Fprintf(os.Stdout, "stale size:             %s\n", colorSizeU(si.LooseObjects.StaleSize+si.Packfiles.CruftSize))
	_, _ = tr.Fprintf(os.Stdout, "keep size:              %s\n", colorSizeU(si.Packfiles.KeepSize))
	if si.LFS.Count != 0 {
		_, _ = tr.Fprintf(os.Stdout, "downloaded lfs count:   %s\n", colorInt(si.LFS.Count))
		_, _ = tr.Fprintf(os.Stdout, "downloaded lfs size:    %s\n", colorSizeU(si.LFS.Size))
	}
	objects := make(map[string]int64)
	filter, err := deflect.NewFilter(o.RepoPath, shaFormat, &deflect.FilterOption{
		Limit: o.Limit,
		Rejector: func(oid string, size int64) error {
			objects[oid] = size
			return nil
		},
	})
	if err := filter.Execute(nil); err != nil {
		fmt.Fprintf(os.Stderr, "hot stat: check large file: %v\n", err)
		return err
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "hot stat: new filter: %v\n", err)
		return err
	}
	fmt.Fprintf(os.Stderr, "%s%s\n", tr.W("repository disk size:   "), colorSize(filter.Size()))
	if !careful {
		_ = showHugeObjects(ctx, o.RepoPath, objects, false)
	}
	return nil
}
