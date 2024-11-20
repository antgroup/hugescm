package index

import (
	"os"
	"testing"
)

func TestIndex(t *testing.T) {
	fd, err := os.Create("/tmp/abc.index")
	if err != nil {
		return
	}
	defer fd.Close()
	treeEntries := make([]TreeEntry, 0, 100)
	e := NewEncoder(fd)
	_ = e.Encode(&Index{
		Version: EncodeVersionSupported,
		Cache: &Tree{
			Entries: treeEntries,
		},
	})

}
