package git

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestRepoIsBare(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoPath := RevParseRepoPath(context.Background(), filepath.Dir(filename))
	fmt.Fprintf(os.Stderr, "RepoIsBare %v\n", RepoIsBare(context.Background(), repoPath))
}

func TestRepoIsBare2(t *testing.T) {
	fmt.Fprintf(os.Stderr, "RepoIsBare %v\n", RepoIsBare(context.Background(), "/tmp/batman.git"))
}
