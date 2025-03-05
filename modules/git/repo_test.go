package git

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRepoIsBare(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoPath := RevParseRepoPath(t.Context(), filepath.Dir(filename))
	fmt.Fprintf(os.Stderr, "RepoIsBare %v\n", RepoIsBare(t.Context(), repoPath))
}

func TestRepoIsBare2(t *testing.T) {
	fmt.Fprintf(os.Stderr, "RepoIsBare %v\n", RepoIsBare(t.Context(), "/tmp/batman.git"))
}
