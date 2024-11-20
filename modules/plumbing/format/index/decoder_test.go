package index

import (
	"fmt"
	"os"
	"testing"
)

func TestDecode(t *testing.T) {
	fd, err := os.Open("/private/tmp/k3/.zeta/index")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open index error: %v\n", err)
		return
	}
	defer fd.Close()
	d := NewDecoder(fd)
	idx := &Index{}
	if err := d.Decode(idx); err != nil {
		fmt.Fprintf(os.Stderr, "decode index error: %v\n", err)
		return
	}
	for _, e := range idx.Entries {
		fmt.Fprintf(os.Stderr, "%v %s\n", e.SkipWorktree, e.Name)
	}
}

func TestDecodeSkip(t *testing.T) {
	fd, err := os.Open("/private/tmp/k4/.zeta/index")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open index error: %v\n", err)
		return
	}
	defer fd.Close()
	d := NewDecoder(fd)
	idx := &Index{}
	if err := d.Decode(idx); err != nil {
		fmt.Fprintf(os.Stderr, "decode index error: %v\n", err)
		return
	}
	checkout := 0
	for _, e := range idx.Entries {
		if e.SkipWorktree {
			continue
		}
		checkout++
	}
	fmt.Fprintf(os.Stderr, "%v total: %d\n", checkout, len(idx.Entries))
}

func TestDecode2(t *testing.T) {
	fd, err := os.Open("/private/tmp/xh5/.zeta/index")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open index error: %v\n", err)
		return
	}
	defer fd.Close()
	d := NewDecoder(fd)
	idx := &Index{}
	if err := d.Decode(idx); err != nil {
		fmt.Fprintf(os.Stderr, "decode index error: %v\n", err)
		return
	}
	for _, e := range idx.Entries {
		if e.Name != "go.pkg" {
			continue
		}
		fmt.Fprintf(os.Stderr, "%v %s\n", e.String(), e.Mode)
	}
}

func TestIndexGlob(t *testing.T) {
	fd, err := os.Open("/private/tmp/k4/.zeta/index")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open index error: %v\n", err)
		return
	}
	defer fd.Close()
	d := NewDecoder(fd)
	idx := &Index{}
	if err := d.Decode(idx); err != nil {
		fmt.Fprintf(os.Stderr, "decode index error: %v\n", err)
		return
	}
	patterns := []string{
		"sigma",
		"sigma/",
		"s*",
		"sigma/*",
	}
	for _, p := range patterns {
		eee, err := idx.Glob(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "glob error: %v\n", err)
			continue
		}
		for _, e := range eee {
			fmt.Fprintf(os.Stderr, "%s: %s\n", p, e.Name)
		}
	}
}
