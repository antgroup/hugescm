package vfs

import (
	"io/fs"
	"os"
)

type VFS interface {
	// Create creates the named file with mode 0666 (before umask), truncating
	// it if it already exists. If successful, methods on the returned File can
	// be used for I/O; the associated file descriptor has mode O_RDWR.
	Create(filename string) (*os.File, error)
	// Open opens the named file for reading. If successful, methods on the
	// returned file can be used for reading; the associated file descriptor has
	// mode O_RDONLY.
	Open(filename string) (*os.File, error)
	// OpenFile is the generalized open call; most users will use Open or Create
	// instead. It opens the named file with specified flag (O_RDONLY etc.) and
	// perm, (0666 etc.) if applicable. If successful, methods on the returned
	// File can be used for I/O.
	OpenFile(filename string, flag int, perm os.FileMode) (*os.File, error)
	// Stat returns a FileInfo describing the named file.
	Stat(filename string) (os.FileInfo, error)
	// Rename renames (moves) oldpath to newpath. If newpath already exists and
	// is not a directory, Rename replaces it. OS-specific restrictions may
	// apply when oldpath and newpath are in different directories.
	Rename(oldpath, newpath string) error
	// Remove removes the named file or directory.
	Remove(filename string) error
	// RemoveAll removes path and any children it contains.
	// It removes everything it can but returns the first error
	// it encounters. If the path does not exist, RemoveAll
	// returns nil (no error).
	// If there is an error, it will be of type *PathError.
	RemoveAll(path string) error
	// Join joins any number of path elements into a single path, adding a
	// Separator if necessary. Join calls filepath.Clean on the result; in
	// particular, all empty strings are ignored. On Windows, the result is a
	// UNC path if and only if the first path element is a UNC path.
	Join(elem ...string) string
	// ReadDir reads the directory named by dirname and returns a list of
	// directory entries sorted by filename.
	ReadDir(path string) ([]fs.DirEntry, error)
	// MkdirAll creates a directory named path, along with any necessary
	// parents, and returns nil, or else returns an error. The permission bits
	// perm are used for all directories that MkdirAll creates. If path is/
	// already a directory, MkdirAll does nothing and returns nil.
	MkdirAll(filename string, perm os.FileMode) error

	// Lstat returns a FileInfo describing the named file. If the file is a
	// symbolic link, the returned FileInfo describes the symbolic link. Lstat
	// makes no attempt to follow the link.
	Lstat(filename string) (os.FileInfo, error)
	// Symlink creates a symbolic-link from link to target. target may be an
	// absolute or relative path, and need not refer to an existing node.
	// Parent directories of link are created as necessary.
	Symlink(target, link string) error
	// Readlink returns the target path of link.
	Readlink(link string) (string, error)
}

func NewVFS(root string) VFS {
	return newBoundOS(root, true)
}
