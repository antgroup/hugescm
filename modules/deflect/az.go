package deflect

import (
	"slices"

	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
)

const (
	MaxLooseObjects = 1000
	MaxPacks        = 3
	MinPackSize     = 4 * strengthen.GiByte
)

type Pack struct {
	Name string
	Size int64
}

type Result struct {
	Size         int64
	LooseObjects int
	Packs        []*Pack
	TmpPacks     uint32
}

func (r *Result) IsUntidy() bool {
	if r.TmpPacks > 0 {
		return true
	}
	if r.LooseObjects > MaxLooseObjects {
		return true
	}
	return len(r.Packs) > MaxPacks && slices.ContainsFunc(r.Packs, func(p *Pack) bool { return p.Size < MinPackSize })
}

func HousekeepingScan(repoPath string) (*Result, error) {
	shaFormat, err := git.HashFormatResult(repoPath)
	if err != nil {
		return nil, err
	}
	filter, err := NewFilter(repoPath, shaFormat, &FilterOption{
		Limit:          strengthen.GiByte,
		QuarantineMode: false,
		Rejector: func(oid string, size int64) error {
			return nil
		},
	})
	if err != nil {
		return nil, err
	}
	if err := filter.Du(); err != nil {
		return nil, err
	}
	result := &Result{
		Size:         filter.size,
		LooseObjects: int(filter.counts),
		Packs:        make([]*Pack, 0, len(filter.packs)),
		TmpPacks:     filter.tmpPacks,
	}
	for _, p := range filter.packs {
		result.Packs = append(result.Packs, &Pack{
			Name: p.path,
			Size: p.size,
		})
	}
	return result, nil
}
