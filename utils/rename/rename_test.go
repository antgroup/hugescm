//go:build windows

package rename

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func TestPosixRename(t *testing.T) {
	a := filepath.Join(os.TempDir(), "ed34a14b-0b09-4078-ac36-71745e4c4084.tmp")
	b := filepath.Join(os.TempDir(), "b.txt")
	fmt.Fprintf(os.Stderr, "rename %s to %s\n", a, b)
	if err := PosixRename(a, b); err != nil {
		fmt.Fprintf(os.Stderr, "rename %s to %s error: %v\n", a, b, err)
		return
	}
	fmt.Fprintf(os.Stderr, "rename success\n")
}

func TestRename(t *testing.T) {
	a := filepath.Join(os.TempDir(), "a.txt")
	b := filepath.Join(os.TempDir(), "b.txt")
	fmt.Fprintf(os.Stderr, "rename %s to %s\n", a, b)
	if err := os.Rename(a, b); err != nil {
		fmt.Fprintf(os.Stderr, "rename %s to %s error: %v\n", a, b, err)
		return
	}
	fmt.Fprintf(os.Stderr, "rename success\n")
}

func TestPosixRemove(t *testing.T) {
	a := filepath.Join(os.TempDir(), "a.txt")
	fmt.Fprintf(os.Stderr, "remove  %s\n", a)
	if err := Remove(a); err != nil {
		fmt.Fprintf(os.Stderr, "remove %s error: %v\n", a, err)
		return
	}
	fmt.Fprintf(os.Stderr, "remove success\n")
}

func TestRemove(t *testing.T) {
	a := filepath.Join(os.TempDir(), "a.txt")
	fmt.Fprintf(os.Stderr, "remove  %s\n", a)
	if err := os.Remove(a); err != nil {
		fmt.Fprintf(os.Stderr, "remove %s error: %v\n", a, err)
		return
	}
	fmt.Fprintf(os.Stderr, "remove success\n")
}

func TestLink(t *testing.T) {
	a := filepath.Join(os.TempDir(), "a.txt")
	os.Link(a, filepath.Join(os.TempDir(), "cc/b.txt"))
	fmt.Fprintf(os.Stderr, "remove  %s\n", a)
	if err := os.Remove(a); err != nil {
		fmt.Fprintf(os.Stderr, "remove %s error: %v\n", a, err)
		return
	}
	fmt.Fprintf(os.Stderr, "remove success\n")
}

func windowsLink(oldpath, newpath string) (err error) {
	for i := 0; i < 2; i++ {
		if err = os.Link(oldpath, newpath); err == nil {
			_ = os.Remove(oldpath)
			return nil
		}
		if !errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			break
		}
		if err = os.Remove(newpath); err != nil {
			break
		}
	}
	return err
}

func TestReFsLink(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		return
	}
	a := filepath.Join(cwd, "a.txt")
	_ = os.WriteFile(a, []byte("hello world\n"), 0644)
	if err := windowsLink(a, filepath.Join(cwd, "b.txt")); err != nil {
		fmt.Fprintf(os.Stderr, "Link %s error: %v\n", a, err)
	}
	// fmt.Fprintf(os.Stderr, "remove  %s\n", a)
	// if err := os.Remove(a); err != nil {
	// 	fmt.Fprintf(os.Stderr, "remove %s error: %v\n", a, err)
	// 	return
	// }
	// fmt.Fprintf(os.Stderr, "remove success\n")
}
