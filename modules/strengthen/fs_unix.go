//go:build !windows

package strengthen

import "os"

func Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

func Remove(name string) error {
	return os.Remove(name)
}
