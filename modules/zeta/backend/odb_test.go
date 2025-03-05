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

func TestODB(t *testing.T) {
	db, err := NewDatabase("/tmp/xh3/.zeta")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database error: %v\n", err)
		return
	}
	defer db.Close()
	cc, err := db.Commit(t.Context(), plumbing.NewHash("498afa6582e4b15dea40f8c355ac01dbbca98c5e4013b552c0a9e3c0ea1872a2"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database error: %v\n", err)
		return
	}
	_ = cc.Pretty(os.Stderr)
}

func TestHashTo(t *testing.T) {
	db, err := NewDatabase("/tmp/blat/.zeta")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database error: %v\n", err)
		return
	}
	defer db.Close()
	_, filename, _, _ := runtime.Caller(0)
	fd, err := os.Open(filename)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open file error: %v\n", err)
		return
	}
	defer fd.Close()
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

func TestLookLooseObjects(t *testing.T) {
	db, err := NewDatabase("/tmp/xh-test/.zeta")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database error: %v\n", err)
		return
	}
	defer db.Close()
	cc, err := db.Commit(t.Context(), plumbing.NewHash("0942fdefc71cd54066e99b56dd47570ae2f18f41eb2406d65b0092e9c9d2efaf"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database error: %v\n", err)
		return
	}
	_ = cc.Pretty(os.Stderr)

	objects, err := db.rw.LooseObjects()
	if err != nil {
		fmt.Fprintf(os.Stderr, "open database error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "\nLoose objects: %d\n", len(objects))
	_ = os.MkdirAll("/tmp/zeta-pack", 0755)
	fd, err := os.CreateTemp("/tmp/zeta-pack", "pack")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open file error: %v\n", err)
		return
	}
	enc, err := pack.NewEncoder(fd, uint32(len(objects)))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open encoder error: %v\n", err)
		_ = fd.Close()
		return
	}
	for _, o := range objects {
		sr, err := db.SizeReader(o, false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open reader error: %v\n", err)
			_ = fd.Close()
			return
		}
		err = enc.Write(o, uint32(sr.Size()), sr, 0)
		sr.Close()
		if err != nil {
			fmt.Fprintf(os.Stderr, "write item error: %v\n", err)
			_ = fd.Close()
			return
		}
	}
	if err := enc.WriteTrailer(); err != nil {
		fmt.Fprintf(os.Stderr, "write trailer error: %v\n", err)
		_ = fd.Close()
		return
	}
	packName := fd.Name()
	_ = fd.Close()
	idx, err := os.CreateTemp("/tmp/zeta-pack", "idx")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open file error: %v\n", err)
		return
	}
	idxName := idx.Name()
	err = enc.WriteIndex(idx)
	idx.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "write index error: %v\n", err)
		return
	}
	name := enc.Name()
	_ = os.MkdirAll("/tmp/zeta-pack/pack", 0755)
	_ = os.Rename(packName, fmt.Sprintf("/tmp/zeta-pack/pack/pack-%s.pack", name))
	_ = os.Rename(idxName, fmt.Sprintf("/tmp/zeta-pack/pack/pack-%s.idx", name))

	mtime, err := os.CreateTemp("/tmp/zeta-pack", "mtimes")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open file error: %v\n", err)
		return
	}
	mtimeName := mtime.Name()
	err = enc.WriteModification(mtime)
	mtime.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "write mtimes error: %v\n", err)
		return
	}
	_ = os.Rename(mtimeName, fmt.Sprintf("/tmp/zeta-pack/pack/pack-%s.mtimes", name))
}

func TestPackDeocde(t *testing.T) {
	sa, err := pack.NewScanner("/tmp/zeta-pack")
	if err != nil {
		fmt.Fprintf(os.Stderr, "read set error: %v\n", err)
		return
	}
	defer sa.Close()
	oid := plumbing.NewHash("ff07b8065913e8f9b8e4c74ad6d2bd64a8b8f0ef8f025567f79950d0c39fe138")
	sr, err := sa.Open(oid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read object error: %v\n", err)
		return
	}
	br, err := object.NewBlob(sr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve blob error: %v\n", err)
		sr.Close()
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
	defer odb.Close()
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
