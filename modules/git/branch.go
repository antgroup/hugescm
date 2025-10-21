package git

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/env"
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
	ErrNotSymbolicRef = errors.New("ref HEAD is not a symbolic ref")
)

// RevParseCurrentName: resolve the reference pointed to by HEAD
func RevParseCurrentName(ctx context.Context, environ []string, repoPath string) (string, error) {
	//  git symbolic-ref HEAD
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath: repoPath,
		Environ:  environ,
		Stderr:   stderr,
	}, "git", "symbolic-ref", "HEAD")
	line, err := cmd.OneLine()
	if err != nil {
		message := stderr.String()
		switch {
		case strings.Contains(message, "ref HEAD is not a symbolic ref"):
			return ReferenceNameDefault, ErrNotSymbolicRef
		case len(message) != 0:
			return ReferenceNameDefault, errors.New(message)
		}
		return ReferenceNameDefault, err
	}
	return line, nil
}

// RevParseCurrent parse HEAD return hash and refname
func RevParseCurrent(ctx context.Context, environ []string, repoPath string) (refname string, hash string, err error) {
	if refname, err = RevParseCurrentName(ctx, environ, repoPath); err != nil {
		if err == ErrNotSymbolicRef {
			// try parse HEAD
			stderr := command.NewStderr()
			cmd := command.NewFromOptions(ctx, &command.RunOpts{RepoPath: repoPath, Environ: environ, Stderr: stderr}, "git", "rev-parse", "HEAD")
			if hash, err = cmd.OneLine(); err != nil {
				if message := stderr.String(); len(message) != 0 {
					err = errors.Join(err, errors.New(message))
				}
				return ReferenceNameDefault, "", err
			}
			return "", hash, nil
		}
		return
	}
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{RepoPath: repoPath, Environ: environ, Stderr: stderr}, "git", "rev-parse", refname)
	if hash, err = cmd.OneLine(); err != nil {
		if message := stderr.String(); len(message) != 0 {
			err = errors.Join(err, errors.New(message))
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

var (
	branchMatches = map[string]bool{
		"refs/heads/master":   true,
		"refs/heads/main":     true,
		"refs/heads/mainline": true,
		"refs/heads/trunk":    true,
	}
	orderBranches = []string{
		"refs/heads/master",
		"refs/heads/main",
		"refs/heads/mainline",
		"refs/heads/trunk",
	}
)

func searchDefaultBranch(ctx context.Context, environ []string, repoPath string) (string, error) {
	reader, err := NewReader(ctx, &command.RunOpts{RepoPath: repoPath, Environ: environ}, "for-each-ref", refHeadPrefix, "--format=%(refname)")
	if err != nil {
		return "", err
	}
	defer reader.Close() // nolint
	scanner := bufio.NewScanner(reader)
	branches := make(map[string]bool)
	var firstBranch string
	for scanner.Scan() {
		if len(branches) == len(orderBranches) && len(firstBranch) != 0 {
			break
		}
		branch := strings.TrimSpace(scanner.Text())
		if branchMatches[branch] {
			branches[branch] = true
			continue
		}
		if len(firstBranch) == 0 {
			firstBranch = branch
			continue
		}
	}
	for _, b := range orderBranches {
		if branches[b] {
			return b, nil
		}
	}
	if len(firstBranch) != 0 {
		return firstBranch, nil
	}
	return "", ErrNoBranches
}

// resolveCurrentReference: Returns the default branch. If the default branch does not exist,
// returns the valid branch from the branch list. The priority of the branch is as follows:
//  1. refs/heads/master
//  2. refs/heads/main
//  3. refs/heads/mainline
//  4. refs/heads/trunk
//
// If none of these branches exist, return the first branch in the branch list.
// Return: refname, needCorrect
func resolveCurrentReference(ctx context.Context, environ []string, repoPath string) (current string, needfix bool, err error) {
	if current, err = RevParseCurrentName(ctx, environ, repoPath); err == nil && strings.HasPrefix(current, refHeadPrefix) {
		return
	}
	needfix = true
	current, err = searchDefaultBranch(ctx, environ, repoPath)
	return
}

func DefaultBranchName(ctx context.Context, repoPath string) (string, error) {
	branchName, _, err := resolveCurrentReference(ctx, env.Environ(), repoPath)
	if err == ErrNoBranches {
		return "", nil
	}
	return branchName, err
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
