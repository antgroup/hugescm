package odb

import (
	"bufio"
	"os"
	"path/filepath"

	"github.com/antgroup/hugescm/modules/plumbing/format/index"
)

const (
	indexPath = "index"
)

func (d *ODB) SetIndex(idx *index.Index) (err error) {
	fd, err := os.Create(filepath.Join(d.root, indexPath))
	if err != nil {
		return err
	}
	defer fd.Close() // nolint

	bw := bufio.NewWriter(fd)
	defer func() {
		if e := bw.Flush(); err == nil && e != nil {
			err = e
		}
	}()

	e := index.NewEncoder(bw)
	err = e.Encode(idx)
	return err
}

func (d *ODB) Index() (i *index.Index, err error) {
	idx := &index.Index{
		Version: index.EncodeVersionSupported,
	}

	fd, err := os.Open(filepath.Join(d.root, indexPath))
	if err != nil {
		if os.IsNotExist(err) {
			return idx, nil
		}
		return nil, err
	}
	defer fd.Close() // nolint
	dec := index.NewDecoder(fd)
	err = dec.Decode(idx)
	return idx, err
}
