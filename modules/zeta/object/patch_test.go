package object

import (
	"fmt"
	"os"
	"testing"
)

func TestPathRenameCombine(t *testing.T) {
	pp := []struct {
		A, B string
	}{
		{
			"a.txt",
			"b.txt",
		},
		{
			"pkg/command/command_merge_file.go",
			"pkg/command/merge.go",
		},
		{
			"pkg/command/command_merge_file.go",
			"pkg/merge.go",
		},
	}
	for _, i := range pp {
		d := PathRenameCombine(i.A, i.B)
		fmt.Fprintf(os.Stderr, "%s => %s|%s\n", i.A, i.B, d)
	}
}
