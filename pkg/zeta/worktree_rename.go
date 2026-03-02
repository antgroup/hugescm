package zeta

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/strengthen"
)

type RenameOptions struct {
	DryRun bool
	Force  bool
}

// isSubdirectory checks if dest is a subdirectory of src
// This prevents moving a directory into itself, e.g., "parent" -> "parent/child"
func isSubdirectory(src, dest string) bool {
	// Normalize paths to use forward slashes (Git-style paths)
	src = filepath.ToSlash(src)
	dest = filepath.ToSlash(dest)

	// Ensure src ends with a slash for proper prefix matching
	if !strings.HasSuffix(src, "/") {
		src = src + "/"
	}

	// Check if dest starts with src
	return strings.HasPrefix(dest, src)
}

func (w *Worktree) validateRenameArgs(source, destination string) (string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		die("zeta rename error: %v", err)
		return "", "", err
	}

	sourceRel, err := filepath.Rel(w.baseDir, filepath.Join(cwd, source))
	if err != nil {
		die("zeta rename error: %v", err)
		return "", "", err
	}
	sourceRel = filepath.ToSlash(sourceRel)
	if hasDotDot(sourceRel) {
		die("'%s' is outside repository at '%s'", source, w.baseDir)
		return "", "", ErrAborting
	}

	destinationRel, err := filepath.Rel(w.baseDir, filepath.Join(cwd, destination))
	if err != nil {
		die("zeta rename error: %v", err)
		return "", "", err
	}
	destinationRel = filepath.ToSlash(destinationRel)
	if hasDotDot(destinationRel) {
		die("'%s' is outside repository at '%s'", destination, w.baseDir)
		return "", "", ErrAborting
	}

	return sourceRel, destinationRel, nil
}

func (w *Worktree) validateRenameable(source string, destination string, force bool) (bool, bool, error) {
	si, err := w.fs.Lstat(source)
	if err != nil {
		die("zeta rename error: %v", err)
		return false, false, err
	}

	// Check if destination is a subdirectory of source (not allowed)
	// This prevents moving a directory into itself, which would cause data loss
	if si.IsDir() && isSubdirectory(source, destination) {
		die("cannot move directory into itself, source=%s, destination=%s", source, destination)
		return false, false, ErrAborting
	}

	di, err := w.fs.Lstat(destination)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, si.IsDir(), nil
		}
		die("zeta rename error: %v", err)
		return false, false, err
	}

	if si.IsDir() != di.IsDir() {
		die("destination already exists, source=%s, destination=%s", source, destination)
		return false, false, ErrAborting
	}

	// Same file (including case-only rename on case-insensitive filesystems)
	// Compatible with Git's behavior: git mv allows case-only renames
	if systemCaseEqual(source, destination) {
		if source == destination {
			die("source and destination are the same file: %s", source)
			return false, false, ErrAborting
		}
		return true, si.IsDir(), nil
	}

	// For directories: always error if target exists (Git doesn't allow overwriting directories)
	// For files: only error if force is not set
	if si.IsDir() {
		die("destination already exists, source=%s, destination=%s", source, destination)
		return false, false, ErrAborting
	}
	if !force {
		die("destination already exists, source=%s, destination=%s", source, destination)
		return false, false, ErrAborting
	}
	return false, false, nil
}

func (w *Worktree) renameConflict(ctx context.Context, source, destination string, conflict bool) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	if !conflict {
		// Direct rename when source and destination are different files
		if err := w.fs.Rename(source, destination); err != nil {
			die("zeta rename error: %v", err)
			return err
		}
		return nil
	}
	// conflict = true means source and destination are the same file
	// (possibly with different case on case-insensitive filesystems)
	// Use a two-step rename to handle case-only renames on Windows/macOS
	// This is compatible with Git's behavior: git mv a A works on Windows
	//
	// The two-step rename strategy:
	//   1. source -> tempDest (to free up the original name)
	//   2. tempDest -> destination (to create the new name)
	//
	// Note: This is not atomic. If step 2 fails, the file will be at tempDest.
	tempDest := filepath.Join(filepath.Dir(source), fmt.Sprintf(".%s@%s", filepath.Base(source), strengthen.NewSessionID()))
	// Step 1: Rename source to temporary destination
	if err := w.fs.Rename(source, tempDest); err != nil {
		die("zeta rename error: failed to rename to temp file %s: %v", tempDest, err)
		return err
	}
	// Step 2: Rename temporary destination to final destination
	if err := w.fs.Rename(tempDest, destination); err != nil {
		die("zeta rename error: failed to rename to destination %s, file is at temp location %s: %v", destination, tempDest, err)
		return err
	}
	return nil
}

/*
zeta rename [-v] [-f] [-n] [-k] <source> <destination>
*/
func (w *Worktree) Rename(ctx context.Context, source, destination string, opts *RenameOptions) error {
	newSource, newDestination, err := w.validateRenameArgs(source, destination)
	if err != nil {
		return err
	}
	conflict, isDir, err := w.validateRenameable(newSource, newDestination, opts.Force)
	if err != nil {
		return err
	}
	if opts.DryRun {
		_, _ = fmt.Fprintf(os.Stdout, "rename %s %s\n", newSource, newDestination)
		return nil
	}
	if err := w.renameConflict(ctx, newSource, newDestination, conflict); err != nil {
		return err
	}
	idx, err := w.odb.Index()
	if err != nil {
		die("zeta rename error: %v", err)
		return err
	}
	if err := idx.Rename(newSource, newDestination, isDir); err != nil {
		die("zeta rename error: %v", err)
		return err
	}
	if err := w.odb.SetIndex(idx); err != nil {
		die("zeta rename error: %v", err)
		return err
	}
	return nil
}
