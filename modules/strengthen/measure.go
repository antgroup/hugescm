package strengthen

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/pprof"
)

type Measurer struct {
	closeFn func()
}

func NewMeasurer(name string, debugMode bool) *Measurer {
	m := &Measurer{}
	if !debugMode {
		return m
	}
	pprofName := filepath.Join(os.TempDir(), fmt.Sprintf("%s-%d.pprof", name, os.Getpid()))
	fd, err := os.Create(pprofName)
	if err != nil {
		return m
	}
	if err = pprof.StartCPUProfile(fd); err != nil {
		_ = fd.Close()
		return m
	}
	m.closeFn = func() {
		pprof.StopCPUProfile()
		_ = fd.Close()
		fmt.Fprintf(os.Stderr, "Task operation completed\ngo tool pprof -http=\":8080\" %s\n", pprofName)
	}
	return m
}

func (d *Measurer) Close() {
	if d.closeFn != nil {
		d.closeFn()
	}
}
