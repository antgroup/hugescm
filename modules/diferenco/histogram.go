package diferenco

// https://stackoverflow.com/questions/32365271/whats-the-difference-between-git-diff-patience-and-git-diff-histogram/32367597#32367597
// https://arxiv.org/abs/1902.02467

const MaxChainLen = 64

type histogramIndex[E comparable] struct {
	tokenOccurances map[E][]int
}

func newHistogramIndex[E comparable](a []E) *histogramIndex[E] {
	h := &histogramIndex[E]{
		tokenOccurances: make(map[E][]int),
	}
	for i, e := range a {
		if p, ok := h.tokenOccurances[e]; ok {
			h.tokenOccurances[e] = append(p, i)
			continue
		}
		h.tokenOccurances[e] = []int{i}
	}
	return h
}

func (h *histogramIndex[E]) numTokenOccurances(e E) int {
	if p, ok := h.tokenOccurances[e]; ok {
		return len(p)
	}
	return 0
}

type lcsResult struct {
	P1      int
	P2      int
	Len     int
	FoundCs bool
}

func findLcs[E comparable](beforce []E, after []E) *lcsResult {
	h := newHistogramIndex(beforce)
	pos := 0
	result := &lcsResult{}
	for pos < len(after) {
		e := after[pos]
		if num := h.numTokenOccurances(e); num != 0 {
			result.FoundCs = true
			if num < MaxChainLen {
				// pos
				continue
			}
		}
		pos++
	}
	return result
}

func histogramDiff[E comparable](beforce []E, beforePos int, after []E, afterPos int) []Change {
	if len(beforce) == 0 || len(after) == 0 {
		return nil
	}
	if len(beforce) == 0 {
		return []Change{{P1: beforePos, P2: afterPos, Ins: len(after)}}
	}
	if len(after) == 0 {
		return []Change{{P1: beforePos, P2: afterPos, Del: len(beforce)}}
	}
	result := findLcs(beforce, after)
	if result != nil {
		return nil
	}
	return nil
}

// func histogramFallbackDiff[T comparable](beforce []T, beforePos int, after []T, afterPos int) []Change {
// 	return onpDiff(beforce, beforePos, after, afterPos)
// }

func HistogramDiff[E comparable](L1, L2 []E) []Change {
	prefix := commonPrefixLength(L1, L2)
	L1 = L1[prefix:]
	L2 = L2[prefix:]
	suffix := commonSuffixLength(L1, L2)
	L1 = L1[:len(L1)-suffix]
	L2 = L2[:len(L2)-suffix]
	return histogramDiff(L1, prefix, L2, prefix)
}
