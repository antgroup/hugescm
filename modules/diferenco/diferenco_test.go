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
				Name: "a.txt",
			},
			To: nil,
			S1: textA,
			S2: textB,
			A:  a,
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
			Name: "a.txt",
			Hash: "4789568",
			Mode: 0o10644,
		},
		To: &File{
			Name: "b.txt",
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
			Name: "a.txt",
			Hash: "4789568",
			Mode: 0o10644,
		},
		To: &File{
			Name: "b.txt",
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
			Name: "a.txt",
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
			Name: "a.txt",
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
			Name: "a.txt",
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

func TestPatchScss(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	bytesA, err := os.ReadFile(filepath.Join(dir, "testdata/simple_1.scss"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read a error: %v\n", err)
		return
	}
	textA := string(bytesA)
	bytesB, err := os.ReadFile(filepath.Join(dir, "testdata/simple_2.scss"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read b error: %v\n", err)
		return
	}
	textB := string(bytesB)
	u, err := DoUnified(context.Background(), &Options{
		From: &File{
			Name: "a.txt",
			Hash: "4789568",
			Mode: 0o10644,
		},
		To: &File{
			Name: "b.txt",
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

func TestPatchCss(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	bytesA, err := os.ReadFile(filepath.Join(dir, "testdata/css_1.css"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read a error: %v\n", err)
		return
	}
	textA := string(bytesA)
	bytesB, err := os.ReadFile(filepath.Join(dir, "testdata/css_2.css"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read b error: %v\n", err)
		return
	}
	textB := string(bytesB)
	u, err := DoUnified(context.Background(), &Options{
		From: &File{
			Name: "a.txt",
			Hash: "4789568",
			Mode: 0o10644,
		},
		To: &File{
			Name: "b.txt",
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

func TestShowPatch(t *testing.T) {
	patch := []*Unified{
		{
			From: &File{
				Name: "docs/a.png",
				Hash: "1ab12893fc666524ed79caae503e12c20a748e2f92db7730c8be09d981970f96",
				Mode: 33188,
			},
			IsBinary: true,
		},
		{
			To: &File{
				Name: "images/windows7.iso",
				Hash: "adba50d9794b9ef3f7ec8cbc680f7f1fa3fbf9df0ac8d1f9b9ccab6d941bc11b",
				Mode: 33188,
			},
			IsFragments: true,
		},
	}
	e := NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode(patch)
}

func TestDiffRunes(t *testing.T) {
	a := "The quick brown fox jumps over the lazy dog"
	b := "The quick brown dog leaps over the lazy cat"
	sd, err := DiffRunes(context.Background(), a, b, ONP)
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff error: %v\n", err)
		return
	}
	for _, d := range sd {
		switch d.Type {
		case Equal:
			fmt.Fprintf(os.Stderr, "%s", d.Text)
		case Insert:
			fmt.Fprintf(os.Stderr, "\x1b[32m%s\x1b[0m", d.Text)
		case Delete:
			fmt.Fprintf(os.Stderr, "\x1b[31m%s\x1b[0m", d.Text)
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
}

func TestDiffWords(t *testing.T) {
	a := "The quick brown fox jumps over the lazy dog"
	b := "The quick brown dog leaps over the lazy cat"
	sd, err := DiffWords(context.Background(), a, b, Histogram, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff error: %v\n", err)
		return
	}
	for _, d := range sd {
		switch d.Type {
		case Equal:
			fmt.Fprintf(os.Stderr, "%s", d.Text)
		case Insert:
			fmt.Fprintf(os.Stderr, "\x1b[32m%s\x1b[0m", d.Text)
		case Delete:
			fmt.Fprintf(os.Stderr, "\x1b[31m%s\x1b[0m", d.Text)
		}
	}
	fmt.Fprintf(os.Stderr, "\n")

}

func TestDiffWords2(t *testing.T) {
	a := "The quick 你好brown fox jumps over the lazy dog"
	b := "The quick 你好 brown dog  leaps over the lazy cat"
	sd, err := DiffWords(context.Background(), a, b, Histogram, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "diff error: %v\n", err)
		return
	}
	for _, d := range sd {
		switch d.Type {
		case Equal:
			fmt.Fprintf(os.Stderr, "%s", d.Text)
		case Insert:
			fmt.Fprintf(os.Stderr, "\x1b[32m%s\x1b[0m", d.Text)
		case Delete:
			fmt.Fprintf(os.Stderr, "\x1b[31m%s\x1b[0m", d.Text)
		}
	}
	fmt.Fprintf(os.Stderr, "\n")
}
