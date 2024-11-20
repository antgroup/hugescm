package vfs

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/securejoin"
)

const (
	defaultDirectoryMode = 0o755
	defaultCreateMode    = 0o666
)

// BoundOS is a fs implementation based on the OS filesystem which is bound to
// a base dir.
// Prefer this fs implementation over ChrootOS.
//
// Behaviours of note:
//  1. Read and write operations can only be directed to files which descends
//     from the base dir.
//  2. Symlinks don't have their targets modified, and therefore can point
//     to locations outside the base dir or to non-existent paths.
//  3. Readlink and Lstat ensures that the link file is located within the base
//     dir, evaluating any symlinks that file or base dir may contain.
type BoundOS struct {
	baseDir         string
	walkBaseDir     string
	deduplicatePath bool
}

func newBoundOS(d string, deduplicatePath bool) VFS {
	walkBaseDir := d
	if wd, err := filepath.EvalSymlinks(d); err == nil && wd != "" {
		walkBaseDir = wd
	}
	return &BoundOS{baseDir: d, walkBaseDir: walkBaseDir, deduplicatePath: deduplicatePath}
}

func (fs *BoundOS) Create(filename string) (*os.File, error) {
	return fs.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, defaultCreateMode)
}

func openFile(fn string, flag int, perm os.FileMode, createDir func(string) error) (*os.File, error) {
	if flag&os.O_CREATE != 0 {
		if createDir == nil {
			return nil, fmt.Errorf("createDir func cannot be nil if file needs to be opened in create mode")
		}
		if err := createDir(fn); err != nil {
			return nil, err
		}
	}
	return os.OpenFile(fn, flag, perm)
}

func (fs *BoundOS) OpenFile(filename string, flag int, perm os.FileMode) (*os.File, error) {
	fn, err := fs.abs(filename)
	if err != nil {
		return nil, err
	}
	return openFile(fn, flag, perm, fs.createDir)
}

func (fs *BoundOS) ReadDir(path string) ([]os.DirEntry, error) {
	dir, err := fs.abs(path)
	if err != nil {
		return nil, err
	}

	return os.ReadDir(dir)
}

func (fs *BoundOS) Rename(from, to string) error {
	f, err := fs.abs(from)
	if err != nil {
		return err
	}
	t, err := fs.abs(to)
	if err != nil {
		return err
	}

	// MkdirAll for target name.
	if err := fs.createDir(t); err != nil {
		return err
	}

	return os.Rename(f, t)
}

func (fs *BoundOS) MkdirAll(path string, perm os.FileMode) error {
	dir, err := fs.abs(path)
	if err != nil {
		return err
	}
	return os.MkdirAll(dir, perm)
}

func (fs *BoundOS) Open(filename string) (*os.File, error) {
	return fs.OpenFile(filename, os.O_RDONLY, 0)
}

func (fs *BoundOS) Stat(filename string) (os.FileInfo, error) {
	filename, err := fs.abs(filename)
	if err != nil {
		return nil, err
	}
	return os.Stat(filename)
}

func (fs *BoundOS) Remove(filename string) error {
	fn, err := fs.abs(filename)
	if err != nil {
		return err
	}
	return os.Remove(fn)
}

func (fs *BoundOS) Join(elem ...string) string {
	return filepath.Join(elem...)
}

func (fs *BoundOS) RemoveAll(path string) error {
	dir, err := fs.abs(path)
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}

func (fs *BoundOS) Symlink(target, link string) error {
	ln, err := fs.abs(link)
	if err != nil {
		return err
	}
	// MkdirAll for containing dir.
	if err := fs.createDir(ln); err != nil {
		return err
	}
	return os.Symlink(target, ln)
}

func (fs *BoundOS) Lstat(filename string) (os.FileInfo, error) {
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(fs.baseDir, filename)
	}
	filename = filepath.Clean(filename)
	if ok, err := fs.insideBaseDirEval(filename); !ok {
		return nil, err
	}
	return os.Lstat(filename)
}

func (fs *BoundOS) Readlink(link string) (string, error) {
	if !filepath.IsAbs(link) {
		link = filepath.Join(fs.baseDir, link)
	}
	link = filepath.Clean(link)
	if ok, err := fs.insideBaseDirEval(link); !ok {
		return "", err
	}
	return os.Readlink(link)
}

// Root returns the current base dir of the billy.Filesystem.
// This is required in order for this implementation to be a drop-in
// replacement for other upstream implementations (e.g. memory and osfs).
func (fs *BoundOS) Root() string {
	return fs.baseDir
}

func (fs *BoundOS) createDir(fullpath string) error {
	dir := filepath.Dir(fullpath)
	if dir != "." {
		if err := os.MkdirAll(dir, defaultDirectoryMode); err != nil {
			return err
		}
	}

	return nil
}

// abs transforms filename to an absolute path, taking into account the base dir.
// Relative paths won't be allowed to ascend the base dir, so `../file` will become
// `/working-dir/file`.
//
// Note that if filename is a symlink, the returned address will be the target of the
// symlink.
func (fs *BoundOS) abs(filename string) (string, error) {
	if filename == fs.baseDir {
		filename = string(filepath.Separator)
	}

	path, err := securejoin.SecureJoin(fs.baseDir, filename)
	if err != nil {
		return "", nil
	}

	if fs.deduplicatePath {
		vol := filepath.VolumeName(fs.baseDir)
		dup := filepath.Join(fs.baseDir, fs.baseDir[len(vol):])
		if strings.HasPrefix(path, dup+string(filepath.Separator)) {
			return fs.abs(path[len(dup):])
		}
	}
	return path, nil
}

var (
	ErrPathOutsideBase = errors.New("path outside base dir")
)

// insideBaseDir checks whether filename is located within
// the fs.baseDir.
func (fs *BoundOS) insideBaseDir(filename string) (bool, error) {
	if filename == fs.baseDir {
		return true, nil
	}
	if !strings.HasPrefix(filename, fs.baseDir+string(filepath.Separator)) {
		return false, ErrPathOutsideBase
	}
	return true, nil
}

type ErrNotInsideBaseDir struct {
	BaseDir string
	Path    string
}

func (e *ErrNotInsideBaseDir) Error() string {
	return fmt.Sprintf("path '%s' outside base dir: %s", e.Path, e.BaseDir)
}

func IsErrNotInsideBaseDir(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrNotInsideBaseDir)
	return ok
}

func insidePathOf(c, p string) bool {
	return strings.HasPrefix(c, p) && len(p) < len(c) && c[len(p)] == filepath.Separator
}

// insideBaseDirEval checks whether filename is contained within
// a dir that is within the fs.baseDir, by first evaluating any symlinks
// that either filename or fs.baseDir may contain.
func (fs *BoundOS) insideBaseDirEval(filename string) (bool, error) {
	if filename == fs.baseDir {
		return true, nil
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(filename))
	if os.IsNotExist(err) {
		if insidePathOf(filename, fs.baseDir) {
			return true, nil
		}
		return false, &ErrNotInsideBaseDir{BaseDir: fs.baseDir, Path: filename}
	}
	if dir != fs.walkBaseDir && dir != fs.baseDir && !insidePathOf(dir, fs.walkBaseDir) {
		return false, &ErrNotInsideBaseDir{BaseDir: fs.baseDir, Path: filename}
	}
	return true, nil
}
