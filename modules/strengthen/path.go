package strengthen

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

var (
	ErrDangerousRepoPath = errors.New("dangerous or unreachable repository path")
)

// ExpandPath is a helper function to expand a relative or home-relative path to an absolute path.
//
// eg.
//
//	~/.someconf -> /home/alec/.someconf
//	~alec/.someconf -> /home/alec/.someconf
func ExpandPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if strings.HasPrefix(path, "~") {
		// For Windows systems, please replace the path separator first
		pos := strings.IndexByte(path, '/')
		switch {
		case pos == 1:
			if homeDir, err := os.UserHomeDir(); err == nil {
				return filepath.Join(homeDir, path[2:])
			}
		case pos > 1:
			// https://github.com/golang/go/issues/24383
			// macOS may not produce correct results
			username := path[1:pos]
			if userAccount, err := user.Lookup(username); err == nil {
				return filepath.Join(userAccount.HomeDir, path[pos+1:])
			}
		default:
		}
	}
	abspath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abspath
}

func splitPathInternal(p string) []string {
	sv := make([]string, 0, 8)
	var first, i int
	for ; i < len(p); i++ {
		if p[i] != '/' && p[i] != '\\' {
			continue
		}
		if first != i {
			sv = append(sv, p[first:i])
		}
		first = i + 1
	}
	if first < len(p) {
		sv = append(sv, p[first:])
	}
	return sv
}

// SplitPath skip empty string suggestcap is suggest cap
func SplitPath(p string) []string {
	if len(p) == 0 {
		return nil
	}
	svv := splitPathInternal(p)
	sv := make([]string, 0, len(svv))
	for _, s := range svv {
		if s == "." {
			continue
		}
		if s == ".." {
			if len(sv) == 0 {
				return sv
			}
			sv = sv[0 : len(sv)-1]
			continue
		}
		sv = append(sv, s)
	}
	return sv
}
