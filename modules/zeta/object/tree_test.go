package object

import (
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/plumbing/filemode"
)

func ShowType(mode filemode.FileMode) ObjectType {
	switch mode & sIFMT {
	case sIFREG:
		return BlobObject
	case sIFDIR:
		return TreeObject
	case sIFLNK:
		return BlobObject
	case sIFGITLINK:
		return CommitObject
	default:
	}
	return 0
}

func TestFragments(t *testing.T) {
	ee := []*TreeEntry{
		{
			Mode: filemode.Dir,
		},
		{
			Mode: filemode.Executable,
		},
		{
			Mode: filemode.Executable | filemode.Fragments,
		},
		{
			Mode: filemode.Regular | filemode.Fragments,
		},
	}
	for _, e := range ee {
		fmt.Fprintf(os.Stderr, "%s %s\n", e.Type(), ShowType(e.Mode))
	}

}

func TestNotFragments(t *testing.T) {
	e := &TreeEntry{
		Mode: filemode.Executable,
	}
	fmt.Fprintf(os.Stderr, "%s\n", e.Type())
}
