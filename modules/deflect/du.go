package deflect

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	ENV_GIT_QUARANTINE_PATH = "GIT_QUARANTINE_PATH"
)

func ReadDir(name string) ([]os.DirEntry, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close() // nolint

	dirs, err := f.ReadDir(-1)
	return dirs, err
}

func (f *Filter) duObject(p, name string, hugeReject, deltaSUM bool) error {
	ds, err := ReadDir(p)
	if err != nil {
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
		f.counts++
		size := fi.Size()
		f.size += size
		if deltaSUM {
			f.delta += size
		}
		if size > hugeSizeLimit {
			f.hugeSum += size
		}
		if hugeReject && size > f.Limit {
			if err := f.reject(name+d.Name(), size); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *Filter) duPacks(packdir string, hugeReject, deltaSUM bool) error {
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
		f.size += size
		if deltaSUM {
			f.delta += size
		}
		dirName := fi.Name()
		if strings.HasPrefix(dirName, "tmp_") {
			f.tmpPacks++
		}
		if filepath.Ext(dirName) != ".pack" {
			continue
		}
		if !hugeReject {
			continue
		}
		// quarantine environment mode optimization： skip small pack
		if f.QuarantineMode && size < f.Limit {
			continue
		}
		f.packs = append(f.packs, pack{path: filepath.Join(packdir, fi.Name()), size: size})
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

func (f *Filter) duInternal(objectsDir string, hugeReject, deltaSUM bool) error {
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
			if err := f.duObject(p, name, hugeReject, deltaSUM); err != nil {
				return err
			}
			continue
		}
		if name == "pack" {
			if err := f.duPacks(filepath.Join(objectsDir, "pack"), hugeReject, deltaSUM); err != nil {
				return err
			}
		}
	}
	return nil
}

func (f *Filter) Du() error {
	if err := f.duInternal(filepath.Join(f.repoPath, "objects"), !f.QuarantineMode, false); err != nil {
		return err
	}
	if !f.QuarantineMode {
		return nil
	}
	incomingPath := os.Getenv(ENV_GIT_QUARANTINE_PATH)
	if len(incomingPath) == 0 {
		return nil
	}
	if err := f.duInternal(incomingPath, true, true); err != nil {
		return err
	}
	return nil
}
