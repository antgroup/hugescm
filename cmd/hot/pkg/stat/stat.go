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

	"github.com/antgroup/hugescm/cmd/hot/tr"
	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/deflect"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/stats"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/term"
)

func colorTextW(s string) string {
	switch term.StderrLevel {
	case term.Level16M:
		return "\x1b[38;2;254;225;64m" + s + "\x1b[0m"
	case term.Level256:
		return "\x1b[33m" + s + "\x1b[0m"
	default:
	}
	return s
}

func colorTextG(s string) string {
	switch term.StderrLevel {
	case term.Level16M:
		return "\x1b[38;2;67;233;123m" + s + "\x1b[0m"
	case term.Level256:
		return "\x1b[32m" + s + "\x1b[0m"
	default:
	}
	return s
}

func colorTextE(s string) string {
	switch term.StderrLevel {
	case term.Level16M:
		return "\x1b[38;2;250;112;154m" + s + "\x1b[0m"
	case term.Level256:
		return "\x1b[31m" + s + "\x1b[0m"
	default:
	}
	return s
}

func colorText(s string) string {
	switch term.StderrLevel {
	case term.Level16M:
		return "\x1b[38;2;0;201;255m" + s + "\x1b[0m"
	case term.Level256:
		return "\x1b[34m" + s + "\x1b[0m"
	default:
	}
	return s
}

func colorSize(i int64) string {
	return colorText(strengthen.FormatSize(i))
}

func colorSizeU(i uint64) string {
	return colorText(strengthen.FormatSizeU(i))
}

func colorInt[I int | uint64 | int64](i I) string {
	return colorText(fmt.Sprintf("%d", i))
}

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
		_, _ = tr.Fprintf(os.Stderr, "error: '%s' is not configured correctly\n", colorTextE("user.name"))
	} else {
		fmt.Fprintf(os.Stderr, "%s 'user.name' --> '%s' ✅\n", tr.W("check"), colorText(name))
	}
	email, ok := vs["user.email"]
	if !ok {
		_, _ = tr.Fprintf(os.Stderr, "error: '%s' is not configured correctly\n", colorTextE("user.email"))
		return
	}
	if !emailRegex.MatchString(email) {
		_, _ = tr.Fprintf(os.Stderr, "error: invalid email '%s' (from user.email)\n", colorTextE(email))
		return
	}
	fmt.Fprintf(os.Stderr, "%s 'user.email' --> '%s' ✅\n", tr.W("check"), colorText(email))
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
			fmt.Fprintf(os.Stderr, "%s %s ✅\n", tr.W("remote:"), colorText(remote))
			return
		}
		fmt.Fprintf(os.Stderr, "parse remote '%s' error: %s\n", colorTextE(remote), err)
		return
	}
	username := u.User.Username()
	password, ok := u.User.Password()
	if ok {
		newPassword := safePassword(password)
		u.User = url.UserPassword(username, newPassword)
		_, _ = tr.Fprintf(os.Stderr, "insecure remote: remote url contains the password '%s' ❌\n", colorTextE(newPassword))
		fmt.Fprintf(os.Stderr, "%s %s ❌ (%s)\n", tr.W("remote:"), colorTextE(u.String()), tr.W("sanitized"))
		return
	}
	fmt.Fprintf(os.Stderr, "%s %s ✅\n", tr.W("remote:"), colorText(u.String()))
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
	_, _ = tr.Fprintf(os.Stderr, "Location: %s\n", colorText(o.RepoPath))
	vs, err := listConfig(ctx, o.RepoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list git config error: %v\n", err)
		return err
	}
	scanIdentity(vs)
	shaFormat, refFormat := git.ExtensionsFormat(o.RepoPath)
	if defaultBranch, ok := vs["init.defaultbranch"]; ok {
		fmt.Fprintf(os.Stderr, "%s 'init.defaultBranch' --> '%s' ✅\n", tr.W("check"), colorText(defaultBranch))
	}
	if defaultObjectFormat, ok := vs["init.defaultobjectformat"]; ok {
		fmt.Fprintf(os.Stderr, "%s 'init.defaultObjectFormat' --> '%s' ✅\n", tr.W("check"), colorText(defaultObjectFormat))
	}
	if defaultRefFormat, ok := vs["init.defaultrefformat"]; ok {
		fmt.Fprintf(os.Stderr, "%s 'init.defaultRefFormat' --> '%s' ✅\n", tr.W("check"), colorText(defaultRefFormat))
	}
	if hooksPath, ok := vs["core.hookspath"]; ok {
		_, _ = tr.Fprintf(os.Stderr, "warning: '%s' is set to '%s', which may affect Git LFS\n", colorTextW("core.hooksPath"), colorTextW(hooksPath))
	}
	_, _ = tr.Fprintf(os.Stderr, "Repository object format (sha format):      %s ✅\n", colorText(shaFormat.String()))
	_, _ = tr.Fprintf(os.Stderr, "Repository references backend (ref format): %s ✅\n", colorText(refFormat))
	checkRemote(vs)
	var careful bool
	sparse, partial := partialClone(vs)
	careful = sparse || partial
	shallow := parseShallowCommit(o.RepoPath)
	if len(shallow) != 0 {
		_, _ = tr.Fprintf(os.Stderr, "shallow clone started at: %s\n", shallow)
	}
	if oid, current, err := git.RevParseCurrentEx(ctx, nil, o.RepoPath); err == nil {
		refname := git.ReferenceName(current)
		if refname.IsBranch() {
			fmt.Fprintf(os.Stderr, "%s: %s (commit: %s)\n", tr.W("On branch"), colorText(refname.BranchName()), colorTextG(oid[:9]))
		} else {
			fmt.Fprintf(os.Stderr, "%s %s\n", tr.W("HEAD detached at"), colorText(oid))
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
		_ = showHugeObjects(ctx, o.RepoPath, objects)
	}
	return nil
}
