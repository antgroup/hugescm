package deflect

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	// ENV_GIT_QUARANTINE_PATH is the environment variable used by Git for incoming objects
	ENV_GIT_QUARANTINE_PATH = "GIT_QUARANTINE_PATH"
)

// ReadDir reads directory entries from the specified path
// Returns a slice of directory entries or an error
func ReadDir(name string) ([]os.DirEntry, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() // nolint

	dirs, err := f.ReadDir(-1)
	return dirs, err
}

// duObject analyzes loose Git objects in a single hash prefix directory (e.g., objects/ab/)
// Parameters:
//   - p: path to the hash prefix directory
//   - name: hash prefix (2 characters) for constructing object IDs
//   - hugeReject: whether to reject files exceeding size limit
//   - deltaSUM: whether to accumulate sizes for quarantine mode
//
// Note: This function silently skips directories that cannot be read because:
// 1. Git objects directory may not contain all 256 hash prefix directories
// 2. Some prefix directories may not exist or be temporarily inaccessible
// 3. Partial statistics are preferable to complete failure in this context
func (a *Auditor) duObject(p, name string, hugeReject, deltaSUM bool) error {
	ds, err := ReadDir(p)
	if err != nil {
		// Silently skip directories that cannot be read - this is intentional design
		return nil
	}
	for _, d := range ds {
		if d.IsDir() {
			continue
		}
		fi, err := d.Info()
		if err != nil {
			continue
		}
		a.counts++
		size := fi.Size()
		a.size += size
		if deltaSUM {
			a.delta += size
		}
		if size > hugeSizeLimit {
			a.hugeSum += size
		}
		if hugeReject && size > a.Limit {
			if err := a.onOversized(name+d.Name(), size); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *Auditor) duPacks(packdir string, hugeReject, deltaSUM bool) error {
	ds, err := ReadDir(packdir)
	if err != nil {
		return err
	}
	for _, d := range ds {
		if d.IsDir() {
			continue
		}
		fi, err := d.Info()
		if err != nil {
			return err
		}
		size := fi.Size()
		a.size += size
		if deltaSUM {
			a.delta += size
		}
		dirName := fi.Name()
		if strings.HasPrefix(dirName, "tmp_") {
			a.tmpPacks++
		}
		if filepath.Ext(dirName) != ".pack" {
			continue
		}
		if !hugeReject {
			continue
		}
		// quarantine environment mode optimization： skip small pack
		if a.QuarantineMode && size < a.Limit {
			continue
		}
		a.packs = append(a.packs, pack{path: filepath.Join(packdir, fi.Name()), size: size})
	}
	return nil
}

// objects/
//        |-00/
//            | - hash
//        |-01
//        |-pack
//              |- pack-$hash.pack
//              |- pack-$hash.idx
//              |- pack-$hash.bitmap
//        |-info

// duInternal analyzes the Git objects directory structure
// Parameters:
//   - objectsDir: path to the objects directory (main or quarantine)
//   - hugeReject: whether to analyze for large objects
//   - deltaSUM: whether to accumulate sizes (for quarantine mode)
//
// This function traverses both loose object directories (00-ff) and pack files
func (a *Auditor) duInternal(objectsDir string, hugeReject, deltaSUM bool) error {
	ds, err := ReadDir(objectsDir)
	if err != nil {
		return err
	}
	for _, d := range ds {
		if !d.IsDir() {
			continue
		}
		name := d.Name()
		if len(name) == 2 {
			p := filepath.Join(objectsDir, name)
			if err := a.duObject(p, name, hugeReject, deltaSUM); err != nil {
				return err
			}
			continue
		}
		if name == "pack" {
			if err := a.duPacks(filepath.Join(objectsDir, "pack"), hugeReject, deltaSUM); err != nil {
				return err
			}
		}
	}
	return nil
}

// Du performs disk usage analysis of the Git repository
// In quarantine mode, also analyzes incoming objects in GIT_QUARANTINE_PATH
func (a *Auditor) Du() error {
	if err := a.duInternal(filepath.Join(a.repoPath, "objects"), !a.QuarantineMode, false); err != nil {
		return err
	}
	if !a.QuarantineMode {
		return nil
	}
	incomingPath := os.Getenv(ENV_GIT_QUARANTINE_PATH)
	if len(incomingPath) == 0 {
		return nil
	}
	if err := a.duInternal(incomingPath, true, true); err != nil {
		return err
	}
	return nil
}
