//go:build !windows

/*
   Copyright The containerd Authors.

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package strengthen

import (
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

// ResolveSymbolicLink will follow any symbolic links
func ResolveSymbolicLink(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if info.Mode()&os.ModeSymlink != os.ModeSymlink {
		return path, nil
	}
	return filepath.EvalSymlinks(path)
}

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

func PathResolve(p string) (string, error) {
	pp := ExpandPath(p)
	_, err := os.Stat(pp)
	return pp, err
}
