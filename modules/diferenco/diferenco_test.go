package diferenco

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/antgroup/hugescm/modules/diferenco/color"
)

func TestDiff(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	bytesA, err := os.ReadFile(filepath.Join(dir, "testdata/a.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read a error: %v\n", err)
		return
	}
	textA := string(bytesA)
	bytesB, err := os.ReadFile(filepath.Join(dir, "testdata/b.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read b error: %v\n", err)
		return
	}
	textB := string(bytesB)
	u, err := DoUnified(context.Background(), &Options{
		From: &File{
			Path: "a.txt",
		},
		To: nil,
		A:  textA,
		B:  textB,
	})
	if err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "%s\n", u)
}

func TestDiff2(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	bytesA, err := os.ReadFile(filepath.Join(dir, "testdata/a.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read a error: %v\n", err)
		return
	}
	textA := string(bytesA)
	bytesB, err := os.ReadFile(filepath.Join(dir, "testdata/b.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read b error: %v\n", err)
		return
	}
	textB := string(bytesB)
	u, err := DoUnified(context.Background(), &Options{
		From: &File{
			Path: "a.txt",
			Hash: "4789568",
			Mode: 0o10644,
		},
		To: &File{
			Path: "b.txt",
			Hash: "6547898",
			Mode: 0o10644,
		},
		A: textA,
		B: textB,
	})
	if err != nil {
		return
	}
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}
