package ignore

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/modules/vfs"
)

const (
	commentPrefix   = "#"
	zetaDir         = ".zeta"
	gitDir          = ".git"
	gitignoreFile   = ".gitignore"
	zetaignoreFile  = ".zetaignore"
	infoExcludeFile = zetaDir + "/info/exclude"
)

// readIgnoreFile reads a specific git ignore file via the VFS so that the
// path is resolved relative to the filesystem's base directory, not the
// process working directory.  This is critical for nested ignore files
// (e.g. sub/.gitignore) which would otherwise be looked up relative to CWD.
func readIgnoreFile(fs vfs.VFS, path []string, ignoreFile string) (ps []Pattern, err error) {
	f, err := fs.Open(fs.Join(append(path, ignoreFile)...))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close() // nolint

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		s := scanner.Text()
		if !strings.HasPrefix(s, commentPrefix) && len(strings.TrimSpace(s)) > 0 {
			ps = append(ps, ParsePattern(s, path))
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return
}

// ReadPatterns reads the .zeta/info/exclude and then the zetaignore patterns
// recursively traversing through the directory structure. The result is in
// the ascending order of priority (last higher).
//
// .zeta/info/exclude is only consulted at the root of the given filesystem,
// matching reference git which reads $GIT_DIR/info/exclude of the repository
// being walked. Ignore files are opened only when present in the directory
// listing, so directories without them cost a single ReadDir.
func ReadPatterns(fs vfs.VFS, path []string) (ps []Pattern, err error) {
	fis, err := fs.ReadDir(filepath.Join(path...))
	if err != nil {
		return nil, err
	}

	var hasZetaDir, hasZetaignore, hasGitignore bool
	for _, fi := range fis {
		switch fi.Name() {
		case zetaDir:
			hasZetaDir = true
		case zetaignoreFile:
			hasZetaignore = true
		case gitignoreFile:
			hasGitignore = true
		}
	}

	// .zeta/info/exclude is only read at the repository root; a nested
	// .zeta/info/exclude belongs to a different repository and must not
	// contribute patterns, matching reference git semantics.
	if len(path) == 0 && hasZetaDir {
		rootPs, _ := readIgnoreFile(fs, path, infoExcludeFile)
		ps = append(ps, rootPs...)
	}

	if hasZetaignore {
		subps, _ := readIgnoreFile(fs, path, zetaignoreFile)
		ps = append(ps, subps...)
	}

	if hasGitignore {
		subps, _ := readIgnoreFile(fs, path, gitignoreFile)
		ps = append(ps, subps...)
	}

	for _, fi := range fis {
		if !fi.IsDir() || fi.Name() == zetaDir || fi.Name() == gitDir {
			continue
		}
		if NewMatcher(ps).Match(append(path, fi.Name()), true) {
			continue
		}
		subps, err := ReadPatterns(fs, append(path, fi.Name()))
		if err != nil {
			return nil, err
		}
		if len(subps) > 0 {
			ps = append(ps, subps...)
		}
	}

	return ps, nil
}
