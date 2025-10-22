package git

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
)

func JoinBranchPrefix(b string) string {
	if strings.HasPrefix(b, refHeadPrefix) {
		return b
	}
	return refHeadPrefix + b
}

func JoinBranchRev(r string) string {
	if ValidateHexLax(r) {
		return r
	}
	if strings.HasPrefix(r, refPrefix) {
		return r
	}
	return refHeadPrefix + r
}

var (
	ErrDetachedHEAD = errors.New("detached HEAD")
)

// RevParseCurrentName: resolve the reference pointed to by HEAD
func RevParseCurrentName(ctx context.Context, environ []string, repoPath string) (string, error) {
	//  git symbolic-ref HEAD
	stderr := command.NewStderr()
	var stdout strings.Builder
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath: repoPath,
		Environ:  environ,
		Stderr:   stderr,
		Stdout:   &stdout,
	}, "git", "symbolic-ref", "HEAD")
	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if strings.Contains(message, "is not a symbolic ref") {
			return ReferenceNameDefault, ErrDetachedHEAD
		}
		if len(message) != 0 {
			err = errors.New(message)
		}
		return ReferenceNameDefault, err
	}
	symref, trailing, ok := strings.Cut(stdout.String(), "\n")
	if !ok {
		return ReferenceNameDefault, errors.New("expected symbolic reference to be terminated by newline")
	}
	if len(trailing) > 0 {
		return ReferenceNameDefault, errors.New("symbolic reference has trailing data")
	}
	return symref, nil
}

// RevParseCurrent parse HEAD return hash and refname
func RevParseCurrent(ctx context.Context, environ []string, repoPath string) (refname string, hash string, err error) {
	if refname, err = RevParseCurrentName(ctx, environ, repoPath); err != nil {
		if err != ErrDetachedHEAD {
			return
		}
		refname = "HEAD" // git checkout commit
	}
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{RepoPath: repoPath, Environ: environ, Stderr: stderr},
		"git", "rev-parse", "--verify", "--end-of-options", refname)
	if hash, err = cmd.OneLine(); err != nil {
		if message := strings.TrimSpace(stderr.String()); len(message) != 0 {
			err = errors.New(message)
		}
		return ReferenceNameDefault, "", err
	}
	return refname, hash, nil
}

// SymReferenceLink: Update default branch or current branch
func SymReferenceLink(ctx context.Context, repoPath string, refname string) error {
	cmd := command.New(ctx, repoPath, "git", "symbolic-ref", "HEAD", refname)
	if err := cmd.RunEx(); err != nil {
		return err
	}
	return nil
}

func FindBranch(ctx context.Context, repoPath string, name string) (*Reference, error) {
	stderr := command.NewStderr()
	reader, err := NewReader(ctx, &command.RunOpts{RepoPath: repoPath, Stderr: stderr}, "branch", "-l", "--format", ReferenceLineFormat, "--", name)
	if err != nil {
		return nil, err
	}
	defer reader.Close() // nolint
	scanner := bufio.NewScanner(reader)
	if scanner.Scan() {
		return ParseOneReference(scanner.Text())
	}
	return nil, NewBranchNotFound(name)
}

var BranchFormatFields = []string{
	"%(refname)", "%(refname:short)",
	"%(objectname)", "%(tree)", "%(contents:subject)",
	"%(authorname)", "%(authoremail)", "%(authordate:iso-strict)",
	"%(committername)", "%(committeremail)", "%(committerdate:iso-strict)",
}

func ParseBranchLineEx(referenceLine string) (*ReferenceEx, error) {
	elements := strings.SplitN(referenceLine, "\x00", len(BranchFormatFields))
	if len(elements) != len(BranchFormatFields) {
		return nil, fmt.Errorf("invalid output from git for-each-ref command: %v", referenceLine)
	}
	cc := &Commit{
		Hash:    elements[2],
		Tree:    elements[3],
		Message: elements[4],
		Author: Signature{
			Name:  elements[5],
			Email: elements[6],
			When:  PareTimeFallback(elements[7]),
		},
		Committer: Signature{
			Name:  elements[8],
			Email: elements[9],
			When:  PareTimeFallback(elements[10]),
		},
	}
	return &ReferenceEx{
		Name:      ReferenceName(elements[0]),
		Commit:    cc,
		ShortName: elements[1]}, nil
}
