//go:build darwin || dragonfly || freebsd

package strengthen

import "golang.org/x/sys/unix"

func detectFileSystem(stat *unix.Statfs_t) string {
	var buf []byte
	for _, c := range stat.Fstypename {
		if c == 0 {
			break
		}
		buf = append(buf, c)
	}

	if len(buf) == 0 {
		return unknownFS
	}

	return string(buf)
}

func GetDiskFreeSpaceEx(mountPath string) (*DiskFreeSpace, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(mountPath, &st); err != nil {
		return nil, err
	}
	ds := &DiskFreeSpace{
		Total: uint64(st.Blocks) * uint64(st.Bsize),
		Avail: uint64(st.Bavail) * uint64(st.Bsize),
		Free:  uint64(st.Bfree) * uint64(st.Bsize),
	}
	ds.Used = ds.Total - ds.Free
	ds.FS = detectFileSystem(&st)
	return ds, nil
}
