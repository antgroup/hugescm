package vfs

import (
	"fmt"
	"os"
	"testing"
)

func TestInsideBaseDir(t *testing.T) {
	b := &BoundOS{baseDir: "/tmp/zeta-1", deduplicatePath: true}
	_, _ = b.insideBaseDir("D:////")
}

func TestInsideBaseDirEval(t *testing.T) {
	b := &BoundOS{baseDir: "/tmp/zeta-1", deduplicatePath: true}
	ok, err := b.Lstat("jack")
	fmt.Fprintf(os.Stderr, "%v %v\n", ok, err)
}

func TestInsideBaseDirEval2(t *testing.T) {
	b := &BoundOS{baseDir: "/", deduplicatePath: true}
	ok, err := b.insideBaseDirEval("abc")
	fmt.Fprintf(os.Stderr, "%v %v\n", ok, err)
}
