//go:build linux
// +build linux

package strengthen

import "golang.org/x/sys/unix"

const (
	FilesystemSuperMagicTmpfs = 0x01021994
	FilesystemSuperMagicExt4  = 0xEF53
	FilesystemSuperMagicXfs   = 0x58465342
	FilesystemSuperMagicNfs   = 0x6969
	FilesystemSuperMagicZfs   = 0x2fc12fc1
	// FilesystemSuperMagicBtrfs is the 64bit magic for Btrfs
	// we not support 32bit system
	FilesystemSuperMagicBtrfs     = 0x9123683E
	FilesystemSuperMagicCGroup    = 0x27e0eb
	FilesystemSuperMagicCGroup2   = 0x63677270
	FilesystemSuperMagicNTFS      = 0x5346544e
	FilesystemSuperMagicEXFAT     = 0x2011BAB0
	FilesystemSuperMagicCEPH      = 0x00c36400
	FilesystemSuperMagicOverlayFS = 0x794c7630
	// https://developer.apple.com/support/downloads/Apple-File-System-Reference.pdf
	FilesystemSuperMagicAPFS = 0x42535041 // BSPA
)

// This map has been collected from `man 2 statfs` and is non-exhaustive
// The values of EXT2, EXT3, and EXT4 have been renamed to a generic EXT as their
// key values were duplicate. This value is now called EXT_2_3_4
// https://github.com/torvalds/linux/blob/master/include/uapi/linux/magic.h
var (
	magicMap = map[int64]string{
		0xadf5:     "adfs",
		0xadff:     "affs",
		0x5346414f: "afs",
		0x0187:     "autofs",
		0x00c36400: "ceph",
		0x73757245: "coda",
		0x28cd3d45: "cramfs", // 0x453dcd28 wroing endianess
		0x64626720: "debugfs",
		0x73636673: "securityfs",
		0xf97cff8c: "selinux",
		0x43415d53: "smack",
		0x858458f6: "ramfs",
		0x01021994: "tmpfs",
		0x958458f6: "hugetlbfs",
		0x73717368: "squashfs",
		0xf15f:     "ecryptfs",
		0x00414a53: "efs",
		0xE0F5E1E2: "erofs",
		0xef53:     "ext_2_3_4",
		0xabba1974: "xenfs",
		0x9123683e: "btrfs",
		0x3434:     "nilfs",
		0xf2f52010: "f2fs",
		0xf995e849: "hpfs",
		0x9660:     "isofs",
		0x72b6:     "jffs2",
		0x58465342: "xfs",
		0x6165676c: "pstorefs",
		0xde5e81e4: "efivarfs",
		0x00c0ffee: "hostfs",
		0x794c7630: "overlayfs",
		0x65735546: "fuse",
		0xca451a4e: "bcachefs",
		// MINIX fs
		0x137f: "minix",
		0x138f: "minix2",
		0x2468: "minix2",
		0x2478: "minix22",
		0x4d5a: "minix3",
		// Others
		0x4d44:     "msdos",
		0x2011bab0: "exFAT",
		0x564c:     "ncp",
		0x6969:     "nfs",
		0x7461636f: "ocfs2",
		0x9fa1:     "openprom",
		0x002f:     "qnx4",
		0x68191122: "qnx6",
		0x6B414653: "afs",
		// used by gcc
		0x52654973: "reiserfs",
		// SMB
		0x517b:     "smb",
		0xff534d42: "smd2", /* the first four bytes of SMB PDUs or SMB2 */
		// CGroup
		0x27e0eb:   "cgroup",
		0x63677270: "cgroup2",
		// tracefs
		0x74726163: "tracefs",
		// next
		0x01021997: "v9fs",
		0x64646178: "daxfs",
		0x42494e4d: "binfmtfs",
		0x1cd1:     "devpts",
		0x6c6f6f70: "binderfs",
		0xbad1dea:  "futexfs",
		0x50495045: "pipefs",
		0x9fa0:     "proc",
		0x534f434b: "sockfs",
		0x62656572: "sysfs",
		0x9fa2:     "usbdevice",
		0x11307854: "mtd_inode_fs",
		0x09041934: "anon_inode_fs",
		0x73727279: "btrfs_test",
		0x6e736673: "nsfs",
		0xcafe4a11: "bpf_fs",
		0x5a3c69f0: "aafs",
		0x5a4f4653: "zonefs",
		0x15013346: "udf",
		0x444d4142: "DMAB",
		0x454d444d: "DMEM",
		0x5345434d: "SECM",
		0x50494446: "PIDF", // PID fs
		// no include
		0x00011954: "ufs",
		0x62646576: "bdevfs",
		0x42465331: "befs",
		0x1badface: "bfs",
		0x012ff7b7: "coh",
		0x1373:     "devfs",
		0x137d:     "ext",
		0xef51:     "ext2_old",
		0x4244:     "hfs",
		0x3153464a: "jfs",
		0x19800202: "mqueue",

		0x7275:     "romfs",
		0x012ff7b6: "sysv2",
		0x012ff7b5: "sysv4",
		0xa501fcf5: "vxfs",
		0x012ff7b4: "xenix",
		// APFS_MAGIC https://github.com/linux-apfs/linux-apfs-rw/blob/master/apfs_raw.h#L1045
		0x42535041: "apfs",
		// NTFS magic
		0x5346544e: "ntfs",
		// ZFS_SUPER_MAGIC https://github.com/openzfs/zfs/blob/6c82951d111bb4c8a426e5f58a87ac80a4996fc1/include/sys/fs/zfs.h#L1374
		0x2fc12fc1: "zfs",
	}
)

func detectFileSystem(stat *unix.Statfs_t) string {
	// This explicit cast to int64 is required for systems where the syscall
	// returns an int32 instead.
	fsType, found := magicMap[int64(stat.Type)] //nolint:unconvert
	if !found {
		return UnknownFS
	}

	return fsType
}

func GetDiskFreeSpaceEx(mountPath string) (*DiskFreeSpace, error) {
	var st unix.Statfs_t
	if err := unix.Statfs(mountPath, &st); err != nil {
		return nil, err
	}
	ds := &DiskFreeSpace{
		Total: st.Blocks * uint64(st.Bsize),
		Avail: uint64(st.Bavail) * uint64(st.Bsize),
		Free:  st.Bfree * uint64(st.Bsize),
	}
	ds.Used = ds.Total - ds.Free
	ds.FS = detectFileSystem(&st)
	return ds, nil
}
