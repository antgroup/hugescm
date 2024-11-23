//go:build !windows

package env

import (
	"os"
	"path/filepath"
	"strings"
)

func InitializeEnv() error {
	pathEnv := os.Getenv("PATH")
	pathList := strings.Split(pathEnv, string(os.PathListSeparator))
	pathNewList := make([]string, 0, len(pathList))
	seen := make(map[string]bool)
	for _, p := range pathList {
		cleanedPath := filepath.Clean(p)
		if cleanedPath == "." {
			continue
		}
		u := strings.ToLower(cleanedPath)
		if seen[u] {
			continue
		}
		seen[u] = true
		pathNewList = append(pathNewList, cleanedPath)
	}
	os.Setenv("PATH", strings.Join(pathNewList, string(os.PathListSeparator)))
	return nil
}
