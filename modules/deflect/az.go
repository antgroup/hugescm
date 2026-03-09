package deflect

import (
	"slices"

	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/strengthen"
)

const (
	// MaxLooseObjects is the threshold for "too many" loose objects (1000)
	MaxLooseObjects = 1000
	// MaxPacks is the threshold for "too many" pack files (3)
	MaxPacks = 3
	// MinPackSize is the minimum size (4GB) for considering packs as "small" in housekeeping
	MinPackSize = 4 * strengthen.GiByte
)

// Pack represents a Git pack file with its name and size
type Pack struct {
	Name string // Full path to pack file
	Size int64  // Size in bytes
}

// Result contains the results of repository housekeeping scan
type Result struct {
	Size         int64   // Total repository size in bytes
	LooseObjects int     // Number of loose objects
	Packs        []*Pack // List of pack files
	TmpPacks     uint32  // Count of temporary pack files
}

// IsUntidy determines if the repository needs housekeeping/maintenance
// Returns true if any of these conditions are met:
// - Has temporary pack files
// - Has too many loose objects (> 1000)
// - Has many pack files (> 3) and at least one is small (< 4GB)
func (r *Result) IsUntidy() bool {
	if r.TmpPacks > 0 {
		return true
	}
	if r.LooseObjects > MaxLooseObjects {
		return true
	}
	return len(r.Packs) > MaxPacks && slices.ContainsFunc(r.Packs, func(p *Pack) bool { return p.Size < MinPackSize })
}

// HousekeepingScan performs a repository housekeeping analysis
// Returns Result struct with repository statistics and maintenance status
// This function is useful for determining if a repository needs git gc/repack
func HousekeepingScan(repoPath string) (*Result, error) {
	shaFormat, err := git.HashFormatResult(repoPath)
	if err != nil {
		return nil, err
	}
	au := NewAuditor(repoPath, shaFormat, &Option{
		Limit:          strengthen.GiByte,
		QuarantineMode: false,
		OnOversized: func(oid string, size int64) error {
			return nil
		},
	})
	if err := au.Du(); err != nil {
		return nil, err
	}
	result := &Result{
		Size:         au.size,
		LooseObjects: int(au.counts),
		Packs:        make([]*Pack, 0, len(au.packs)),
		TmpPacks:     au.tmpPacks,
	}
	for _, p := range au.packs {
		result.Packs = append(result.Packs, &Pack{
			Name: p.path,
			Size: p.size,
		})
	}
	return result, nil
}
