package diffbug

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/antgroup/hugescm/modules/diff"
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
	diffs := diff.Do(string(bytesA), string(bytesB))
	for _, d := range diffs {
		fmt.Fprintf(os.Stderr, "%v %s\n", d.Type, d.Text)
	}
}
