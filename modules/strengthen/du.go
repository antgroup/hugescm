//go:build !windows

package strengthen

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"

	"golang.org/x/sys/unix"
)

const (
	SystemBlockSize int64 = 512
)

type duWalker struct {
	size      int64
	dirSize   int64
	ignoreErr bool
}

func isReg(si *unix.Stat_t) bool {
	return si.Mode&unix.S_IFMT == syscall.S_IFREG
}

func isDir(si *unix.Stat_t) bool {
	return si.Mode&unix.S_IFMT == syscall.S_IFDIR
}

func (d *duWalker) unixStat(p string) error {
	var si unix.Stat_t
	if err := unix.Stat(p, &si); err != nil {
		if !d.ignoreErr {
			return err
		}
		return nil
	}
	if !isReg(&si) {
		return nil
	}
	// number of 512B blocks allocated
	d.size += si.Blocks * SystemBlockSize
	return nil
}

func (d *duWalker) du(path string) error {
	d.size += d.dirSize
	dirs, err := os.ReadDir(path)
	if err != nil {
		return err
	}
	for _, it := range dirs {
		if !it.IsDir() {
			if err := d.unixStat(filepath.Join(path, it.Name())); err != nil {
				return err
			}
			continue
		}
		if err := d.du(filepath.Join(path, it.Name())); err != nil {
			if !d.ignoreErr {
				return err
			}
		}
	}
	return nil
}

func Du(path string) (int64, error) {
	var si unix.Stat_t
	if err := unix.Stat(path, &si); err != nil {
		return 0, err
	}
	if !isDir(&si) {
		if !isReg(&si) {
			return 0, nil
		}
		return si.Blocks * SystemBlockSize, nil
	}
	dw := &duWalker{ignoreErr: true} // skip broken symlink
	// Windows and macOS directory self size is zero not like Linux. Linux 4K (blocks)
	if runtime.GOOS != "darwin" {
		dw.dirSize = si.Blocks
	}
	if err := dw.du(path); err != nil {
		return dw.size, err
	}
	return dw.size, nil
}
