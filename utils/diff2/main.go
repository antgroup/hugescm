package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
	"time"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/diferenco/color"
	"github.com/antgroup/hugescm/pkg/zeta"
)

type Debuger struct {
	closeFn func()
}

func NewDebuger(debugMode bool) *Debuger {
	d := &Debuger{}
	if !debugMode {
		return d
	}
	pprofName := filepath.Join(os.TempDir(), fmt.Sprintf("zeta-%d.pprof", os.Getpid()))
	fd, err := os.Create(pprofName)
	if err != nil {
		return d
	}
	if err = pprof.StartCPUProfile(fd); err != nil {
		_ = fd.Close()
		return d
	}
	d.closeFn = func() {
		pprof.StopCPUProfile()
		fd.Close()
		fmt.Fprintf(os.Stderr, "Task operation completed\ngo tool pprof -http=\":8080\" %s\n", pprofName)
	}
	return d
}

func (d *Debuger) Close() {
	if d.closeFn != nil {
		d.closeFn()
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "usage: %s file1 file2\n", os.Args[0])
		return
	}
	fd1, err := os.Open(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "open file %s error: %v\n", os.Args[1], err)
		return
	}
	defer fd1.Close()
	fd2, err := os.Open(os.Args[2])
	if err != nil {
		fmt.Fprintf(os.Stderr, "open file %s error: %v\n", os.Args[2], err)
		return
	}
	defer fd2.Close()
	d := NewDebuger(true)
	now := time.Now()
	u, err := diferenco.DoUnified(context.Background(), &diferenco.Options{
		A: diferenco.Histogram,
		From: &diferenco.File{
			Name: os.Args[1],
		},
		To: &diferenco.File{
			Name: os.Args[2],
		},
		R1: fd1,
		R2: fd2,
	})
	if err != nil {
		return
	}
	d.Close()
	fmt.Fprintf(os.Stderr, "use time: %v\n", time.Since(now))
	p := zeta.NewPrinter(context.Background())
	defer p.Close()
	e := diferenco.NewUnifiedEncoder(p)
	e.SetColor(color.NewColorConfig())
	_ = e.Encode([]*diferenco.Unified{u})

}
