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
func ExpandPath(rawPath string) string {
	if filepath.IsAbs(rawPath) {
		return rawPath
	}
	p, ok := strings.CutPrefix(rawPath, "~")
	if !ok {
		abspath, err := filepath.Abs(rawPath)
		if err != nil {
			return rawPath
		}
		return abspath
	}
	username, suffix, ok := strings.Cut(p, "/")
	var homeDir string
	var err error
	if username == "" {
		if homeDir, err = os.UserHomeDir(); err != nil {
			return rawPath
		}
	} else {
		userAccount, err := user.Lookup(username)
		if err != nil {
			return rawPath
		}
		homeDir = userAccount.HomeDir
	}
	if ok {
		return filepath.Join(homeDir, suffix)
	}
	return homeDir
}

func PathResolve(p string) (string, error) {
	pp := ExpandPath(p)
	_, err := os.Stat(pp)
	return pp, err
}
