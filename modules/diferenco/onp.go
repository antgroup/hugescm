//	Copyright (c) 2014-2021 Akinori Hattori <hattya@gmail.com>
//
//	SPDX-License-Identifier: MIT
//
//	SOURCE: https://github.com/hattya/go.diff
//
// Package diff implements the difference algorithm, which is based upon
// S. Wu, U. Manber, G. Myers, and W. Miller,
// "An O(NP) Sequence Comparison Algorithm" August 1989.
package diferenco

import "context"

func onpDiff[E comparable](ctx context.Context, L1 []E, P1 int, L2 []E, P2 int) ([]Change, error) {
	m, n := len(L1), len(L2)
	c := &onpCtx[E]{L1: L1, L2: L2, P1: P1, P2: P2}
	if n >= m {
		c.M = m
		c.N = n
	} else {
		c.M = n
		c.N = m
		c.xchg = true
	}
	c.Δ = c.N - c.M
	return c.compare(ctx)
}

type onpCtx[E comparable] struct {
	L1, L2 []E
	P1, P2 int
	M, N   int
	Δ      int
	fp     []point
	xchg   bool
}

func (c *onpCtx[E]) compare(ctx context.Context) ([]Change, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}
	c.fp = make([]point, (c.M+1)+(c.N+1)+1)
	for i := range c.fp {
		c.fp[i].y = -1
	}

	Δ := c.Δ + (c.M + 1)
	for p := 0; c.fp[Δ].y != c.N; p++ {
		for k := -p; k < c.Δ; k++ {
			c.snake(k)
		}
		for k := c.Δ + p; k > c.Δ; k-- {
			c.snake(k)
		}
		c.snake(c.Δ)
	}

	lcs, n := c.reverse(c.fp[Δ].lcs)
	changes := make([]Change, 0, n+1)
	var x, y int
	for ; lcs != nil; lcs = lcs.next {
		if x < lcs.x || y < lcs.y {
			if !c.xchg {
				changes = append(changes, Change{x + c.P1, y + c.P2, lcs.x - x, lcs.y - y})
			} else {
				changes = append(changes, Change{y + c.P1, x + c.P2, lcs.y - y, lcs.x - x})
			}
		}
		x = lcs.x + lcs.n
		y = lcs.y + lcs.n
	}
	if x < c.M || y < c.N {
		if !c.xchg {
			changes = append(changes, Change{x + c.P1, y + c.P2, c.M - x, c.N - y})
		} else {
			changes = append(changes, Change{y + c.P1, x + c.P2, c.N - y, c.M - x})
		}
	}
	return changes, nil
}

func (c *onpCtx[E]) snake(k int) {
	var y int
	var prev *onpLcs
	kk := k + (c.M + 1)

	h := &c.fp[kk-1]
	v := &c.fp[kk+1]
	if h.y+1 >= v.y {
		y = h.y + 1
		prev = h.lcs
	} else {
		y = v.y
		prev = v.lcs
	}

	x := y - k
	n := 0
	for x < c.M && y < c.N {
		var eq bool
		if !c.xchg {
			eq = c.L1[x] == c.L2[y]
		} else {
			eq = c.L1[y] == c.L2[x]
		}
		if !eq {
			break
		}
		x++
		y++
		n++
	}

	p := &c.fp[kk]
	p.y = y
	if n == 0 {
		p.lcs = prev
	} else {
		p.lcs = &onpLcs{
			x:    x - n,
			y:    y - n,
			n:    n,
			next: prev,
		}
	}
}

func (c *onpCtx[E]) reverse(curr *onpLcs) (next *onpLcs, n int) {
	for ; curr != nil; n++ {
		curr.next, next, curr = next, curr, curr.next
	}
	return
}

type point struct {
	y   int
	lcs *onpLcs
}

type onpLcs struct {
	x, y int
	n    int
	next *onpLcs
}

// OnpDiff returns the differences between data.
// It makes O(NP) (the worst case) calls to data.Equal.
func OnpDiff[E comparable](ctx context.Context, L1, L2 []E) ([]Change, error) {
	//return myersDiff(L1, 0, L2, 0)
	prefix := commonPrefixLength(L1, L2)
	L1 = L1[prefix:]
	L2 = L2[prefix:]
	suffix := commonSuffixLength(L1, L2)
	L1 = L1[:len(L1)-suffix]
	L2 = L2[:len(L2)-suffix]
	return onpDiff(ctx, L1, prefix, L2, prefix)
}
