package deflect

import (
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
)

// [core]
// 	repositoryformatversion = 1
// 	filemode = true
// 	bare = false
// 	logallrefupdates = true
// 	ignorecase = true
// 	precomposeunicode = true
// [extensions]
// 	objectformat = sha256

const (
	DefaultFileSizeLimit = strengthen.MiByte * 50
	hugeSizeLimit        = strengthen.MiByte * 15
)

type FilterOption struct {
	Limit          int64 // file size limit: > Limit --> reject
	Rejector       func(oid string, size int64) error
	QuarantineMode bool // If  Quarantine is enable
}

type SizeReceiver func(size int64) error

type pack struct {
	path string
	size int64
}

type Filter struct {
	*FilterOption
	repoPath string
	size     int64
	delta    int64
	hugeSum  int64
	rawsz    int64 //
	counts   uint32
	packs    []pack
	tmpPacks uint32
}

func NewFilter(repoPath string, shaFormat git.HashFormat, opts *FilterOption) (*Filter, error) {
	f := &Filter{
		repoPath: repoPath,
		rawsz:    int64(shaFormat.RawSize()),
	}
	if opts == nil {
		f.FilterOption = &FilterOption{
			Limit: DefaultFileSizeLimit,
		}
		return f, nil
	}
	f.FilterOption = &FilterOption{
		Limit:          opts.Limit,
		Rejector:       opts.Rejector,
		QuarantineMode: opts.QuarantineMode,
	}
	if f.Limit <= 0 {
		f.Limit = DefaultFileSizeLimit // avoid --> f.Limit <= 0
	}
	return f, nil
}

func (f *Filter) HashLen() int64 {
	return f.rawsz
}

func (f *Filter) Counts() uint32 {
	return f.counts
}

func (f *Filter) Size() int64 {
	return f.size
}

func (f *Filter) Delta() int64 {
	return f.delta
}

func (f *Filter) HugeSUM() int64 {
	return f.hugeSum
}

func (f *Filter) Execute(sr SizeReceiver) error {
	if err := f.Du(); err != nil {
		return err
	}
	if sr != nil {
		if err := sr(f.size); err != nil {
			return err
		}
	}
	for _, p := range f.packs {
		if err := f.FilterPack(&p); err != nil {
			return err
		}
	}
	return nil
}

func (f *Filter) reject(oid string, size int64) error {
	if f.Rejector == nil {
		fmt.Fprintf(os.Stderr, "blob: %s compressed size: %s\n", oid, strengthen.FormatSize(size))
		return nil
	}
	return f.Rejector(oid, size)
}

func Du(repoPath string) (int64, error) {
	shaFormat, err := git.HashFormatResult(repoPath)
	if err != nil {
		return 0, err
	}
	f, err := NewFilter(repoPath, shaFormat, &FilterOption{
		Limit:          strengthen.GiByte,
		QuarantineMode: false,
		Rejector: func(oid string, size int64) error {
			return nil
		},
	})
	if err != nil {
		return 0, err
	}
	if err := f.Du(); err != nil {
		return 0, err
	}
	return f.Size(), nil
}
