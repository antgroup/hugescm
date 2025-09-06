package strengthen

type DiskFreeSpace struct {
	Total uint64
	Used  uint64
	Free  uint64
	Avail uint64
	FS    string
}

const UnknownFS = "unknown"
