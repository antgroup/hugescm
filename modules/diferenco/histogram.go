// Refer to https://github.com/pascalkuthe/imara-diff reimplemented in Golang.
package diferenco

import "context"

// https://stackoverflow.com/questions/32365271/whats-the-difference-between-git-diff-patience-and-git-diff-histogram/32367597#32367597
// https://arxiv.org/abs/1902.02467

const MaxChainLen = 63

type histogram[E comparable] struct {
	tokenOccurances map[E][]int
}

func (h *histogram[E]) populate(a []E) {
	for i, e := range a {
		if p, ok := h.tokenOccurances[e]; ok {
			h.tokenOccurances[e] = append(p, i)
			continue
		}
		h.tokenOccurances[e] = []int{i}
	}
}

func (h *histogram[E]) numTokenOccurances(e E) int {
	if p, ok := h.tokenOccurances[e]; ok {
		return len(p)
	}
	return 0
}

func (h *histogram[E]) clear() {
	// runtime: clear() is slow for maps with big capacity and small number of items
	// https://github.com/golang/go/issues/70617
	h.tokenOccurances = make(map[E][]int)
}

type Lcs struct {
	beforeStart int
	afterStart  int
	length      int
}

type LcsSearch[E comparable] struct {
	lcs            Lcs
	minOccurrences int
	foundCS        bool
}

func (s *LcsSearch[E]) run(before, after []E, h *histogram[E]) {
	pos := 0
	for pos < len(after) {
		e := after[pos]
		if num := h.numTokenOccurances(e); num != 0 {
			s.foundCS = true
			if num <= s.minOccurrences {
				pos = s.updateLcs(before, after, pos, e, h)
				continue
			}
		}
		pos++
	}
	h.clear()
}

func (s *LcsSearch[E]) updateLcs(before, after []E, afterPos int, token E, h *histogram[E]) int {
	nextTokenIndex2 := afterPos + 1
	tokenOccurances := h.tokenOccurances[token]
	tokenIndex1 := tokenOccurances[0]
	pos := 1
occurancesIter:
	for {
		occurances := h.numTokenOccurances(token)
		s1, s2 := tokenIndex1, afterPos
		for {
			if s1 == 0 || s2 == 0 {
				break
			}
			t1, t2 := before[s1-1], after[s2-1]
			if t1 != t2 {
				break
			}
			s1--
			s2--
			newOcurances := h.numTokenOccurances(t1)
			occurances = min(newOcurances, occurances)
		}
		e1, e2 := tokenIndex1+1, afterPos+1
		for {
			if e1 >= len(before) || e2 >= len(after) {
				break
			}
			t1, t2 := before[e1], after[e2]
			if t1 != t2 {
				break
			}
			newOccuraces := h.numTokenOccurances(t1)
			occurances = min(occurances, newOccuraces)
			e1++
			e2++
		}
		if nextTokenIndex2 < e2 {
			nextTokenIndex2 = e2
		}
		length := e2 - s2
		if s.lcs.length < length || s.minOccurrences > occurances {
			s.minOccurrences = occurances
			s.lcs = Lcs{
				beforeStart: s1,
				afterStart:  s2,
				length:      length,
			}
		}
		for {
			if pos >= len(tokenOccurances) {
				break occurancesIter
			}
			nextTokenIndex := tokenOccurances[pos]
			pos++
			if nextTokenIndex > e2 {
				tokenIndex1 = nextTokenIndex
				break
			}
		}
	}
	return nextTokenIndex2
}

func (s *LcsSearch[E]) ok() bool {
	return !s.foundCS || s.minOccurrences <= MaxChainLen
}

func findLcs[E comparable](before, after []E, index *histogram[E]) *Lcs {
	s := LcsSearch[E]{
		minOccurrences: MaxChainLen + 1,
	}
	s.run(before, after, index)
	if s.ok() {
		return &s.lcs
	}
	return nil
}

type changesOut struct {
	changes []Change
}

func (h *histogram[E]) run(ctx context.Context, beforce []E, beforePos int, after []E, afterPos int, o *changesOut) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if len(beforce) == 0 {
			if len(after) != 0 {
				o.changes = append(o.changes, Change{P1: beforePos, P2: afterPos, Ins: len(after)})
			}
			return nil
		}
		if len(after) == 0 {
			o.changes = append(o.changes, Change{P1: beforePos, P2: afterPos, Del: len(beforce)})
			return nil
		}
		h.populate(beforce)
		lcs := findLcs(beforce, after, h)
		if lcs == nil {
			changes, err := onpCompute(ctx, beforce, beforePos, after, afterPos)
			if err != nil {
				return err
			}
			o.changes = append(o.changes, changes...)
			return nil
		}
		if lcs.length == 0 {
			o.changes = append(o.changes, Change{P1: beforePos, P2: afterPos, Del: len(beforce), Ins: len(after)})
			return nil
		}
		if err := h.run(ctx, beforce[:lcs.beforeStart], beforePos, after[:lcs.afterStart], afterPos, o); err != nil {
			return err
		}
		e1 := lcs.beforeStart + lcs.length
		beforce = beforce[e1:]
		beforePos += e1
		e2 := lcs.afterStart + lcs.length
		after = after[e2:]
		afterPos += e2
	}
}

// HistogramDiff: calculates the difference using the histogram algorithm
func HistogramDiff[E comparable](ctx context.Context, L1, L2 []E) ([]Change, error) {
	prefix := commonPrefixLength(L1, L2)
	L1 = L1[prefix:]
	L2 = L2[prefix:]
	suffix := commonSuffixLength(L1, L2)
	L1 = L1[:len(L1)-suffix]
	L2 = L2[:len(L2)-suffix]
	h := &histogram[E]{
		tokenOccurances: make(map[E][]int, len(L1)),
	}
	o := &changesOut{changes: make([]Change, 0, 100)}
	if err := h.run(ctx, L1, prefix, L2, prefix, o); err != nil {
		return nil, err
	}
	return o.changes, nil
}
