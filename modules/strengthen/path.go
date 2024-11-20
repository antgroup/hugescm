package strengthen

import (
	"errors"
	"os/user"
	"path/filepath"
	"strings"
)

var (
	ErrDangerousRepoPath = errors.New("dangerous or unreachable repository path")
)

// ExpandPath is a helper function to expand a relative or home-relative path to an absolute path.
//
// eg. ~/.someconf -> /home/alec/.someconf
func ExpandPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		user, err := user.Current()
		if err != nil {
			return path
		}
		return filepath.Join(user.HomeDir, path[2:])
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
