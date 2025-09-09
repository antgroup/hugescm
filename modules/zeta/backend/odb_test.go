package backend

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"testing"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend/pack"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

func TestHashTo(t *testing.T) {
	db, err := NewDatabase("/tmp/blat/.zeta")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database error: %v\n", err)
		return
	}
	defer db.Close() // nolint
	_, filename, _, _ := runtime.Caller(0)
	fd, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open file error: %v\n", err)
		return
	}
	defer fd.Close() // nolint
	si, err := fd.Stat()
	if err != nil {
		fmt.Fprintf(os.Stderr, "stat error: %v\n", err)
		return
	}
	oid, err := db.HashTo(t.Context(), fd, si.Size())
	if err != nil {
		fmt.Fprintf(os.Stderr, "hashTo error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "oid: %s\n", oid)
}

func TestPackDeocde(t *testing.T) {
	sa, err := pack.NewScanner("/tmp/zeta-pack")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read set error: %v\n", err)
		return
	}
	defer sa.Close() // nolint
	oid := plumbing.NewHash("ff07b8065913e8f9b8e4c74ad6d2bd64a8b8f0ef8f025567f79950d0c39fe138")
	sr, err := sa.Open(oid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read object error: %v\n", err)
		return
	}
	br, err := object.NewBlob(sr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve blob error: %v\n", err)
		_ = sr.Close()
		return
	}
	_, _ = io.Copy(os.Stderr, br.Contents)
	var count int
	if err := sa.PackedObjects(func(oid plumbing.Hash, mtime int64) error {
		count++
		return nil
	}); err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "count: %d\n", count)
}

func TestSearchObject(t *testing.T) {
	odb, err := NewDatabase("/tmp/xh5/.zeta")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve blob error: %v\n", err)
		return
	}
	defer odb.Close() // nolint
	oid, err := odb.Search("ff0929c5c92f519f59518666d094c315f")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read set error: %v prefix: %s\n", err, oid.Prefix())
		return
	}
	fmt.Fprintf(os.Stderr, "object %s\n", oid)
}

func TestRemoveNonEmptyDir(t *testing.T) {
	err := os.Remove("/tmp/b2")
	fmt.Fprintf(os.Stderr, "%s %v\n", err, os.IsExist(err))
}

func TestSleep(t *testing.T) {
	time.Sleep(0)
	fmt.Fprintf(os.Stderr, "%s\n", os.Getenv("LANG"))
}
