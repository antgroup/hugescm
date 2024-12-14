package diferenco

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

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
	aa := []Algorithm{Histogram, Myers, ONP, Patience}
	for _, a := range aa {
		now := time.Now()
		u, err := DoUnified(context.Background(), &Options{
			From: &File{
				Path: "a.txt",
			},
			To:        nil,
			S1:        textA,
			S2:        textB,
			Algorithm: a,
		})
		if err != nil {
			return
		}
		fmt.Fprintf(os.Stderr, "\x1b[32m%s --> use time: %v\x1b[0m\n%s\n", a, time.Since(now), u)
	}

}

func TestPatchFD(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	fd, err := os.Open(filepath.Join(dir, "testdata/a.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read a error: %v\n", err)
		return
	}
	defer fd.Close()
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
		R1: fd,
		S2: textB,
	})
	if err != nil {
		return
	}
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}

func TestPatch(t *testing.T) {
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
		S1: textA,
		S2: textB,
	})
	if err != nil {
		return
	}
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}

func TestPatchNew(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	bytesB, err := os.ReadFile(filepath.Join(dir, "testdata/b.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read b error: %v\n", err)
		return
	}
	textB := string(bytesB)
	u, err := DoUnified(context.Background(), &Options{
		From: nil,
		To: &File{
			Path: "a.txt",
			Hash: "6547898",
			Mode: 0o10644,
		},
		S1: "",
		S2: textB,
	})
	if err != nil {
		return
	}
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}

func TestPatchDelete(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	bytesA, err := os.ReadFile(filepath.Join(dir, "testdata/a.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read a error: %v\n", err)
		return
	}
	textA := string(bytesA)
	u, err := DoUnified(context.Background(), &Options{
		From: &File{
			Path: "a.txt",
			Hash: "6547898",
			Mode: 0o10644,
		},
		To: nil,
		S1: textA,
		S2: "",
	})
	if err != nil {
		return
	}
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}

func TestDiff2(t *testing.T) {
	textA := `hello
world

foo
c07e640b246c7885cbc3d5c627acbcb2d2ab9c95`
	textB := `hello
novel
world

foo bar
31df1778815171897c907daf454c4419cfaa46f9`
	u, err := DoUnified(context.Background(), &Options{
		From: &File{
			Path: "a.txt",
			Hash: "6547898",
			Mode: 0o10644,
		},
		To: nil,
		S1: textA,
		S2: textB,
	})
	if err != nil {
		return
	}
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*Unified{u})
}
