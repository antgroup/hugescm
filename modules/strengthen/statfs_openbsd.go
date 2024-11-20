//go:build openbsd && !386

package strengthen

import "golang.org/x/sys/unix"

func detectFileSystem(stat *unix.Statfs_t) string {
	var buf []byte
	for _, c := range stat.F_fstypename {
		if c == 0 {
			break
		}
		buf = append(buf, byte(c))
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
		Total: st.F_blocks * uint64(st.F_bsize),
		Avail: uint64(st.F_favail) * uint64(st.F_bsize),
		Free:  st.F_ffree * uint64(st.F_bsize),
	}
	ds.Used = ds.Total - ds.Free
	ds.FS = detectFileSystem(&st)
	return ds, nil
}
