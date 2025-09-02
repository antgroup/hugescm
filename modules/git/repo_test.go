package git

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsBareRepository(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoPath := RevParseRepoPath(t.Context(), filepath.Dir(filename))
	fmt.Fprintf(os.Stderr, "IsBareRepository %v\n", IsBareRepository(t.Context(), repoPath))
}
