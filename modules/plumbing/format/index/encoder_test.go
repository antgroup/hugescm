package index

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func TestIndex(t *testing.T) {
	fd, err := os.Create("/tmp/abc.index")
	if err != nil {
		return
	}
	defer fd.Close() // nolint
	treeEntries := make([]TreeEntry, 0, 100)
	e := NewEncoder(fd)
	_ = e.Encode(&Index{
		Version: EncodeVersionSupported,
		Cache: &Tree{
			Entries: treeEntries,
		},
	})

}

func TestEncodeV4(t *testing.T) {
	idx := &Index{
		Version: 4,
		Entries: []*Entry{{
			CreatedAt:  time.Now(),
			ModifiedAt: time.Now(),
			Dev:        4242,
			Inode:      424242,
			UID:        84,
			GID:        8484,
			Size:       42,
			Stage:      TheirMode,
			Hash:       plumbing.NewHash("e25b29c8946e0e192fae2edc1dabf7be71e8ecf3"),
			Name:       "foo",
		}, {
			CreatedAt:  time.Now(),
			ModifiedAt: time.Now(),
			Name:       "bar",
			Size:       82,
		}, {
			CreatedAt:  time.Now(),
			ModifiedAt: time.Now(),
			Name:       strings.Repeat(" ", 20),
			Size:       82,
		}, {
			CreatedAt:  time.Now(),
			ModifiedAt: time.Now(),
			Name:       "baz/bar",
			Size:       82,
		}, {
			CreatedAt:  time.Now(),
			ModifiedAt: time.Now(),
			Name:       "baz/bar/bar",
			Size:       82,
		}},
	}

	buf := bytes.NewBuffer(nil)
	e := NewEncoder(buf)
	if err := e.Encode(idx); err != nil {
		fmt.Fprintf(os.Stderr, "error %v\n", err)
		return
	}

	output := &Index{}
	d := NewDecoder(buf)
	if err := d.Decode(output); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return
	}
	for _, e := range output.Entries {
		fmt.Fprintf(os.Stderr, "%s\n", e.Name)
	}

}
