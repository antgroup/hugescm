//go:build windows

package strengthen

import (
	"os"
	"path/filepath"
)

func Du(path string) (int64, error) {
	dirs, err := os.ReadDir(path)
	if err != nil {
		return 0, err
	}
	var size int64
	for _, d := range dirs {
		di, err := d.Info()
		if err != nil {
			return size, nil
		}
		size += di.Size()
		if !d.IsDir() {
			continue
		}
		dirPath := filepath.Join(path, d.Name())
		if sz, err := Du(dirPath); err == nil {
			size += sz
		}
	}
	return size, nil
}
