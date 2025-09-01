package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git/config"
)

const (
	Sundries = "sundries"
)

func RevParseHashFormat(ctx context.Context, repoPath string) (string, error) {
	cmd := command.New(ctx, repoPath, "git", "rev-parse", "--show-object-format")
	format, err := cmd.OneLine()
	if err != nil {
		return "", fmt.Errorf("detect repo object format: %v", command.FromError(err))
	}
	return format, nil
}

func HashFormatResult(repoPath string) (HashFormat, error) {
	cfg, err := config.BareDecode(repoPath)
	if err != nil {
		return HashUNKNOWN, err
	}
	return HashFormatFromName(cfg.HashFormat()), nil
}

func HashFormatOK(repoPath string) HashFormat {
	if h, err := HashFormatResult(repoPath); err == nil {
		return h
	}
	return HashSHA1
}

// ExtensionsFormat: return objectFormat, refFormat
func ExtensionsFormat(repoPath string) (HashFormat, string) {
	cfg, err := config.BareDecode(repoPath)
	if err != nil {
		return HashSHA1, "files"
	}
	return HashFormatFromName(cfg.HashFormat()), cfg.ReferencesFormat()
}

// RevParseRepoPath parse repo dir
func RevParseRepoPath(ctx context.Context, p string) string {
	cmd := command.NewFromOptions(ctx,
		&command.RunOpts{
			Environ:  os.Environ(),
			RepoPath: p,
		},
		"git", "rev-parse", "--git-dir")
	repoPath, err := cmd.OneLine()
	if err != nil {
		return p
	}
	if filepath.IsAbs(repoPath) {
		return repoPath
	}
	return filepath.Join(p, repoPath)
}

var (
	ErrBlankRevision = errors.New("empty revision")
	ErrBadRevision   = errors.New("revision can't start with '-'")
)

// ValidateBytesRevision checks if a revision looks valid
func ValidateBytesRevision(revision []byte) error {
	if len(revision) == 0 {
		return ErrBlankRevision
	}
	if bytes.HasPrefix(revision, []byte("-")) {
		return ErrBadRevision
	}
	return nil
}

// ValidateBytesRevision checks if a revision looks valid
func ValidateRevision(revision string) error {
	if len(revision) == 0 {
		return ErrBlankRevision
	}
	if strings.HasPrefix(revision, "-") {
		return ErrBadRevision
	}
	return nil
}

// FallbackTimeValue is the value returned by `SafeTimeParse` in case it
// encounters a parse error. It's the maximum time value possible in golang.
// See https://gitlab.com/gitlab-org/gitaly/issues/556#note_40289573
var FallbackTimeValue = time.Unix(1<<63-62135596801, 999999999)

// PareTimeFallback parses a git date string with the RFC3339 format. If the date
// is invalid (possibly because the date is larger than golang's largest value)
// it returns the maximum date possible.
func PareTimeFallback(s string) time.Time {
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t
	}
	return FallbackTimeValue
}

func NewSundriesDir(repoPath string, pattern string) (string, error) {
	sundries := filepath.Join(repoPath, Sundries)
	if err := os.Mkdir(sundries, 0700); err != nil && !os.IsExist(err) {
		return "", err
	}
	return os.MkdirTemp(sundries, pattern)
}
