package strengthen

import (
	"fmt"
	"os"
	"testing"
)

func TestGetDiskFreeSpaceEx(t *testing.T) {
	gb := float64(1024 * 1024 * 1024)
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	ds, err := GetDiskFreeSpaceEx(cwd)
	if err != nil {
		fmt.Fprintf(os.Stderr, "usage: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "disk space total: %0.2f GB. used: %0.2f GB. available: %0.2f GB FS: %s\n",
		float64(ds.Total)/gb, float64(ds.Used)/gb, float64(ds.Avail)/gb, ds.FS)
}

func TestGetDiskFreeSpaceExTemp(t *testing.T) {
	gb := float64(1024 * 1024 * 1024)
	ds, err := GetDiskFreeSpaceEx(os.TempDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "usage: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "disk space total: %0.2f GB. used: %0.2f GB. available: %0.2f GB FS: %s\n",
		float64(ds.Total)/gb, float64(ds.Used)/gb, float64(ds.Avail)/gb, ds.FS)
}
