package diffbug

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"unicode/utf8"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/diferenco/color"
)

func TestDiffText(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	dir := filepath.Dir(filename)
	bytesA, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read a error: %v\n", err)
		return
	}
	bytesB, err := os.ReadFile(filepath.Join(dir, "b.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "read b error: %v\n", err)
		return
	}
	u, err := diferenco.DoUnified(t.Context(), &diferenco.Options{
		From: &diferenco.File{
			Name: "a.go",
		},
		To: &diferenco.File{
			Name: "a.go",
		},
		S1: string(bytesA),
		S2: string(bytesB),
	})
	if err != nil {
		return
	}
	e := diferenco.NewUnifiedEncoder(os.Stderr)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*diferenco.Unified{u})
}

func TestRuneToString(t *testing.T) {
	rs := []rune{0xD800 + 1, 0xDFFF - 1, 1, 2, 3, utf8.MaxRune, math.MaxInt32}
	s := string(rs)
	rs2 := []rune(s)
	for _, c := range s {
		fmt.Fprintf(os.Stderr, "%04X\n", c)
	}
	for i, c := range rs2 {
		fmt.Fprintf(os.Stderr, "%d %04X\n", i, c)
	}
}
