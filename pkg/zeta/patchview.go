package zeta

import (
	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/patchview"
)

func runPatchView(patches []*diferenco.Patch, wordDiff bool) error {
	if len(patches) == 0 {
		return nil
	}
	formatter := newDiffFormatter(wordDiff)
	entries := make([]patchview.Entry, 0, len(patches))
	for _, p := range patches {
		if p == nil {
			continue
		}
		stat := p.Stat()
		entries = append(entries, patchview.Entry{
			Name:     patchName(p),
			Status:   patchStatus(p),
			Addition: stat.Addition,
			Deletion: stat.Deletion,
			Content:  formatter.formatPatch(p),
		})
	}
	return patchview.Run(entries)
}

func patchName(p *diferenco.Patch) string {
	if p == nil {
		return ""
	}
	switch {
	case p.From == nil && p.To != nil:
		return p.To.Name
	case p.From != nil && p.To == nil:
		return p.From.Name
	case p.From != nil && p.To != nil && p.From.Name != p.To.Name:
		return p.From.Name + " → " + p.To.Name
	case p.To != nil:
		return p.To.Name
	case p.From != nil:
		return p.From.Name
	default:
		return ""
	}
}

func patchStatus(p *diferenco.Patch) string {
	if p == nil {
		return "M"
	}
	switch {
	case p.From == nil:
		return "A"
	case p.To == nil:
		return "D"
	case p.From != nil && p.To != nil && p.From.Name != p.To.Name:
		return "R"
	default:
		return "M"
	}
}
