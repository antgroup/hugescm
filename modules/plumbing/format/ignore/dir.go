package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/vfs"
)

const (
	commentPrefix   = "#"
	zetaDir         = ".zeta"
	gitignoreFile   = ".gitignore"
	zetaignoreFile  = ".zetaignore"
	infoExcludeFile = zetaDir + "/info/exclude"
)

// readIgnoreFile reads a specific git ignore file.
func readIgnoreFile(fs vfs.VFS, path []string, ignoreFile string) (ps []Pattern, err error) {
	ignoreFile = strengthen.ExpandPath(ignoreFile)
	f, err := os.Open(fs.Join(append(path, ignoreFile)...))
	if err == nil {
		defer f.Close()

		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			s := scanner.Text()
			if !strings.HasPrefix(s, commentPrefix) && len(strings.TrimSpace(s)) > 0 {
				ps = append(ps, ParsePattern(s, path))
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}

	return
}

// ReadPatterns reads the .zeta/info/exclude and then the zetaignore patterns
// recursively traversing through the directory structure. The result is in
// the ascending order of priority (last higher).
func ReadPatterns(fs vfs.VFS, path []string) (ps []Pattern, err error) {
	ps, _ = readIgnoreFile(fs, path, infoExcludeFile)

	subps, _ := readIgnoreFile(fs, path, zetaignoreFile)
	ps = append(ps, subps...)
	subps, _ = readIgnoreFile(fs, path, gitignoreFile)
	ps = append(ps, subps...)

	dirs, err := fs.ReadDir(filepath.Join(path...))
	if err != nil {
		return
	}

	for _, d := range dirs {
		if d.IsDir() && d.Name() != zetaDir {
			if NewMatcher(ps).Match(append(path, d.Name()), true) {
				continue
			}
			var subps []Pattern
			subps, err = ReadPatterns(fs, append(path, d.Name()))
			if err != nil {
				return
			}

			if len(subps) > 0 {
				ps = append(ps, subps...)
			}
		}
	}

	return
}
