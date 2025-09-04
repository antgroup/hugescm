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
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/trace"
)

var (
	emailRegex = regexp.MustCompile(`^[A-Za-z\d]+([-_.][A-Za-z\d]+)*@([A-Za-z\d]+[-.])+[A-Za-z\d]{2,4}$`)
)

type StatOptions struct {
	RepoPath string
}

type Values map[string]string

func listConfig(ctx context.Context, repoPath string) (Values, error) {
	var stderr strings.Builder
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath: repoPath,
		Stderr:   &stderr,
	}, "git", "config", "list", "-z")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer stdout.Close()
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
		_, _ = tr.Fprintf(os.Stderr, "error: '\u001b[31m%s\u001b[0m' is not configured correctly\n", "user.name")
	} else {
		fmt.Fprintf(os.Stderr, "%s 'user.name' --> '%s' ✅\n", tr.W("check"), name)
	}
	email, ok := vs["user.email"]
	if !ok {
		_, _ = tr.Fprintf(os.Stderr, "error: '\u001b[31m%s\u001b[0m' is not configured correctly\n", "user.email")
		return
	}
	if !emailRegex.MatchString(email) {
		_, _ = tr.Fprintf(os.Stderr, "error: invalid email (from user.email) '\u001b[31m%s\u001b[0m'\n", email)
		return
	}
	fmt.Fprintf(os.Stderr, "%s 'user.email' --> '%s' ✅\n", tr.W("check"), email)
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
			fmt.Fprintf(os.Stderr, "%s %s ✅\n", tr.W("remote:"), remote)
			return
		}
		fmt.Fprintf(os.Stderr, "parse remote '%s' error: %s\n", remote, err)
		return
	}
	username := u.User.Username()
	password, ok := u.User.Password()
	if ok {
		newPassword := safePassword(password)
		u.User = url.UserPassword(username, newPassword)
		_, _ = tr.Fprintf(os.Stderr, "insecure remote: remote url contains the password '%s' ❌\n", newPassword)
		fmt.Fprintf(os.Stderr, "%s %s ❌\n", tr.W("remote:"), u.String())
		return
	}
	fmt.Fprintf(os.Stderr, "%s %s ✅\n", tr.W("remote:"), u.String())
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
	_, _ = tr.Fprintf(os.Stderr, "Location: %s\n", o.RepoPath)
	vs, err := listConfig(ctx, o.RepoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "list git config error: %v\n", err)
		return err
	}
	scanIdentity(vs)
	shaFormat, refFormat := git.ExtensionsFormat(o.RepoPath)
	if defaultBranch, ok := vs["init.defaultbranch"]; ok {
		fmt.Fprintf(os.Stderr, "%s 'init.defaultBranch' --> '%s' ✅\n", tr.W("check"), defaultBranch)
	}
	if defaultObjectFormat, ok := vs["init.defaultobjectformat"]; ok {
		fmt.Fprintf(os.Stderr, "%s 'init.defaultObjectFormat' --> '%s' ✅\n", tr.W("check"), defaultObjectFormat)
	}
	if defaultRefFormat, ok := vs["init.defaultrefformat"]; ok {
		fmt.Fprintf(os.Stderr, "%s 'init.defaultRefFormat' --> '%s' ✅\n", tr.W("check"), defaultRefFormat)
	}
	if hooksPath, ok := vs["core.hookspath"]; ok {
		_, _ = tr.Fprintf(os.Stderr, "\u001b[33mcore.hooksPath\u001b[0m is set to '%s', which may affect Git LFS\n", hooksPath)
	}
	_, _ = tr.Fprintf(os.Stderr, "Repository object format (sha format):      %s ✅\n", shaFormat)
	_, _ = tr.Fprintf(os.Stderr, "Repository references backend (ref format): %s ✅\n", refFormat)
	checkRemote(vs)
	var careful bool
	sparse, partial := partialClone(vs)
	careful = sparse || partial
	shallow := parseShallowCommit(o.RepoPath)
	if len(shallow) != 0 {
		_, _ = tr.Fprintf(os.Stderr, "shallow clone started at: %s\n", shallow)
	}
	trace.DbgPrint("careful: %v", careful)
	if oid, current, err := git.RevParseCurrentEx(ctx, nil, o.RepoPath); err == nil {
		fmt.Fprintf(os.Stderr, "%s: %s (commit: %s)\n", tr.W("On branch"), strings.TrimPrefix(current, "refs/heads/"), oid[:9])
	}

	if careful {
		// We do not proceed with checking out repositories that use partial clones or sparse checkouts.
		return nil
	}
	return nil
}
