package pack

import (
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/binary"
	"github.com/antgroup/hugescm/modules/plumbing"
)

func TestPackDecode(t *testing.T) {
	fd, err := os.Open("/tmp/git-pack.idx")
	if err != nil {
		fmt.Fprintf(os.Stderr, "open index error: %v\n", err)
		return
	}
	defer fd.Close()
	_, _ = fd.Seek(4+4, io.SeekStart)
	for i := range 256 {
		n, err := binary.ReadUint32(fd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open index error: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "Fanout: %d - %d\n", i, n)
	}
	_, _ = fd.Seek(4+4+4*256, io.SeekStart)
	for i := 0; i < 260; i++ {
		var oid [20]byte
		if _, err := io.ReadFull(fd, oid[:]); err != nil {
			fmt.Fprintf(os.Stderr, "read oid index error: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "%s\n", hex.EncodeToString(oid[:]))
	}
}

func TestLastIndexByte(t *testing.T) {
	ss := []string{
		"00",
		"12",
		"123456",
		"abcd000000123455",
		"abcdefdd",
	}
	for _, s := range ss {
		o := plumbing.NewHash(s)
		fmt.Fprintf(os.Stderr, "prefix: %s\n", o.Prefix())
	}
}
