//go:build !windows

package strengthen

import (
	"os"
)

func FinalizeObject(oldpath string, newpath string) (err error) {
	if err = os.Link(oldpath, newpath); err == nil {
		_ = os.Remove(oldpath)
		return
	}
	return os.Rename(oldpath, newpath)
}
