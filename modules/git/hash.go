package git

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/env"
	"github.com/zeebo/blake3"
)

var (
	refsHashablePrefix = [][]byte{
		[]byte("refs/heads/"),
		[]byte("refs/tags/"),
		[]byte("refs/pull/"),
		[]byte("refs/merge-requests/"),
	}
)

// RevParseHEAD: resolve the reference pointed to by HEAD
//
// not git repo: fatal: not a git repository (or any parent directory): .git
//
// empty repo: HEAD
// fatal: 有歧义的参数 'HEAD'：未知的版本或路径不存在于工作区中。
// 使用 '--' 来分隔版本和路径，例如：
// 'git <命令> [<版本>...] -- [<文件>...]'
//
// ref not exists: HEAD
//
// ref exists: refs/heads/master
func RevParseHEAD(ctx context.Context, environ []string, repoPath string) (string, error) {
	//  git rev-parse --symbolic-full-name HEAD
	cmd := command.NewFromOptions(ctx, &command.RunOpts{RepoPath: repoPath, Environ: environ},
		"git", "rev-parse", "--symbolic-full-name", "HEAD")
	line, err := cmd.OneLine()
	if err != nil {
		return ReferenceNameDefault, err
	}
	return line, nil
}

// ParseReference parse symref return hash and refname
func ParseReference(ctx context.Context, repoPath string, symref string) (string, string, error) {
	//  git rev-parse HEAD --symbolic-full-name HEAD
	cmd := command.NewFromOptions(ctx, &command.RunOpts{RepoPath: repoPath},
		"git", "rev-parse", symref, "--symbolic-full-name", symref)
	output, err := cmd.Output()
	if err != nil {
		return "", ReferenceNameDefault, err
	}
	var hash, refname string
	lines := strings.Split(string(output), "\n")
	if len(lines) >= 2 {
		refname = lines[1]
	}
	if len(lines) >= 1 {
		hash = lines[0]
	}
	return hash, refname, nil
}

// afa70145a25e81faa685dc0b465e52b45d2444bd refs/heads/master
func startsWithHashablePrefix(line []byte) bool {
	_, ref, ok := bytes.Cut(line, []byte(" "))
	if !ok {
		return false
	}
	for _, p := range refsHashablePrefix {
		if bytes.HasPrefix(ref, p) {
			return true
		}
	}
	return false
}

// HashFromEnv: Calculate the hash of the repository at the specified path and environment block
func HashFromEnv(ctx context.Context, environ []string, repoPath string) (string, error) {
	if _, err := os.Stat(repoPath); err != nil && os.IsNotExist(err) {
		return "", err
	}
	head, err := RevParseHEAD(ctx, environ, repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable resolve %s HEAD error: %v\n", repoPath, err)
	}
	h := blake3.New()
	fmt.Fprintf(h, "ref: %s\n", head)
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		Environ:  environ,
		RepoPath: repoPath,
		Stderr:   stderr,
	}, "git", "show-ref")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("unable create stdout pipe %v", err)
	}
	defer out.Close()
	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("unable create stdout pipe %v", err)
	}
	sr := bufio.NewScanner(out)
	for sr.Scan() {
		line := bytes.TrimSpace(sr.Bytes())
		if !startsWithHashablePrefix(line) {
			continue
		}
		_, _ = h.Write(line)
		_, _ = h.Write([]byte("\n"))
	}
	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			fmt.Fprintf(os.Stderr, "hash %s error: %s\n", repoPath, stderr.String())
		}
		return "", fmt.Errorf("hash error %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Hash: Calculate the hash of the repository at the specified path
func Hash(ctx context.Context, repoPath string) (string, error) {
	return HashFromEnv(ctx, env.Environ(), repoPath)
}

type HashResult struct {
	HEAD       string
	Hash       string
	References int
}

// HashEx: Calculates the hash of the repository at the specified path and returns HEAD, the number of references
func HashEx(ctx context.Context, repoPath string) (*HashResult, error) {
	if _, err := os.Stat(repoPath); err != nil && os.IsNotExist(err) {
		return nil, err
	}
	hr := &HashResult{}
	head, err := RevParseHEAD(ctx, env.Environ(), repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable resolve %s HEAD error: %v\n", repoPath, err)
	}
	hr.HEAD = head
	h := blake3.New()
	fmt.Fprintf(h, "ref: %s\n", head)
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx,
		&command.RunOpts{RepoPath: repoPath, Stderr: stderr},
		"git", "show-ref")
	out, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("unable create stdout pipe %v", err)
	}
	defer out.Close()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("unable create stdout pipe %v", err)
	}

	sr := bufio.NewScanner(out)
	for sr.Scan() {
		line := bytes.TrimSpace(sr.Bytes())
		if !startsWithHashablePrefix(line) {
			continue
		}
		hr.References++
		_, _ = h.Write(line)
		_, _ = h.Write([]byte("\n"))
	}
	if err := cmd.Wait(); err != nil {
		if stderr.Len() > 0 {
			fmt.Fprintf(os.Stderr, "hash %s error: %s\n", repoPath, stderr.String())
		}
		return nil, fmt.Errorf("hash error %w", err)
	}
	hr.Hash = hex.EncodeToString(h.Sum(nil))
	return hr, nil
}
