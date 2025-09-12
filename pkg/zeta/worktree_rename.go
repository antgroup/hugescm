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

func (w *Worktree) validateRenameArgs(source, destination string) (string, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		die("zeta rename error: %v", err)
		return "", "", err
	}
	var sourceRel, destinationRel string
	if sourceRel, err = filepath.Rel(w.baseDir, filepath.Join(cwd, source)); err != nil {
		die("zeta rename error: %v", err)
		return "", "", err
	}
	sourceRel = filepath.ToSlash(sourceRel)
	if hasDotDot(sourceRel) {
		die("'%s' is outside repository at '%s'", source, w.baseDir)
		return "", "", ErrAborting
	}
	if destinationRel, err = filepath.Rel(w.baseDir, filepath.Join(cwd, destination)); err != nil {
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
	if si.IsDir() {
		// source is DIR
		di, err := w.fs.Lstat(destination)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return false, true, nil
			}
			die("zeta rename error: %v", err)
			return false, false, err
		}
		if !di.IsDir() || source == destination {
			die("destination already exists, source=%s, destination=%s", source, destination)
			return false, false, ErrAborting
		}
		// same file
		if caseInsensitive && strings.EqualFold(source, destination) {
			return true, true, nil
		}
		die("destination already exists, source=%s, destination=%s", source, destination)
		return false, false, ErrAborting
	}
	di, err := w.fs.Lstat(destination)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, false, nil
		}
		die("zeta rename error: %v", err)
		return false, false, err
	}
	if di.IsDir() || source == destination {
		die("destination already exists, source=%s, destination=%s", source, destination)
		return false, false, ErrAborting
	}
	// same file
	if caseInsensitive && strings.EqualFold(source, destination) {
		return true, false, nil
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
		if err := w.fs.Rename(source, destination); err != nil {
			die("zeta rename error: %v", err)
			return err
		}
		return nil
	}
	tempDest := filepath.Join(filepath.Dir(source), fmt.Sprintf(".%s@%s", filepath.Base(source), strengthen.NewSessionID()))
	if err := w.fs.Rename(source, tempDest); err != nil {
		die("zeta rename error: %v", err)
		return err
	}
	if err := w.fs.Rename(tempDest, destination); err != nil {
		die("zeta rename error: %v", err)
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
