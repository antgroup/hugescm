package deflect_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/antgroup/hugescm/modules/deflect"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/stretchr/testify/require"
)

func TestDeflectFilter(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoPath := git.RevParseRepoPath(t.Context(), filepath.Dir(filename))
	shaFormat, err := git.HashFormatResult(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo: %v", err)
		return
	}
	fe, err := deflect.NewFilter(repoPath, shaFormat, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error %v", err)
		return
	}
	if err := fe.Execute(nil); err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "RepoSize: %d, Q: %d hashLen: %d\n", fe.Size(), fe.Delta(), fe.HashLen())
}

func TestDeflectFilter2(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoPath := git.RevParseRepoPath(t.Context(), filepath.Dir(filename))
	shaFormat, err := git.HashFormatResult(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "repo: %v", err)
		return
	}
	t.Setenv(deflect.ENV_GIT_QUARANTINE_PATH, filepath.Join(repoPath, "objects"))
	fe, err := deflect.NewFilter(repoPath, shaFormat, &deflect.FilterOption{
		Limit:          10 << 20,
		Rejector:       nil,
		QuarantineMode: true,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error %v", err)
		return
	}
	if err := fe.Execute(nil); err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "RepoSize: %d, Q: %d hashLen: %d\n", fe.Size(), fe.Delta(), fe.HashLen())
}

func TestRepoSize(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoPath := git.RevParseRepoPath(t.Context(), filepath.Dir(filename))
	size, err := deflect.Du(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error %v", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s repo size: %s\n", repoPath, strengthen.FormatSize(size))
	require.True(t, size > 0)
}

func TestHousekeepingScan(t *testing.T) {
	_, filename, _, _ := runtime.Caller(0)
	repoPath := git.RevParseRepoPath(t.Context(), filepath.Dir(filename))
	result, err := deflect.HousekeepingScan(repoPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "repo %s needs maintenance: %v packs: %d loose objects: %d size: %s\n",
		repoPath, result.IsUntidy(), len(result.Packs), result.LooseObjects, strengthen.FormatSize(result.Size))
}
