package diferenco

import (
	"context"
	"math/rand"
	"testing"
)

// myersFast is a GPT implementation for comparison
func myersFast[E comparable](ctx context.Context, a []E, P1 int, b []E, P2 int) ([]Change, error) {
	n := len(a)
	m := len(b)

	if n == 0 && m == 0 {
		return nil, nil
	}
	if n == 0 {
		return []Change{{P1: P1, P2: P2, Ins: m}}, nil
	}
	if m == 0 {
		return []Change{{P1: P1, P2: P2, Del: n}}, nil
	}

	max := n + m
	offset := max

	V := make([]int, 2*max+1)
	trace := make([][]int, 0, max+1)

	V[offset] = 0

	for d := 0; d <= max; d++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		Vcopy := make([]int, len(V))
		copy(Vcopy, V)
		trace = append(trace, Vcopy)

		for k := -d; k <= d; k += 2 {
			var x int

			if k == -d || (k != d && V[offset+k-1] < V[offset+k+1]) {
				x = V[offset+k+1]
			} else {
				x = V[offset+k-1] + 1
			}

			y := x - k

			for x < n && y < m && a[x] == b[y] {
				x++
				y++
			}

			V[offset+k] = x

			if x >= n && y >= m {
				return buildScriptFast(trace, a, b, P1, P2)
			}
		}
	}

	return nil, nil
}

func buildScriptFast[E comparable](trace [][]int, a, b []E, P1, P2 int) ([]Change, error) {
	x := len(a)
	y := len(b)

	max := len(a) + len(b)
	offset := max

	changes := make([]Change, 0, 16)

	for d := len(trace) - 1; d >= 0; d-- {
		V := trace[d]
		k := x - y

		var prevK int

		if k == -d || (k != d && V[offset+k-1] < V[offset+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}

		prevX := V[offset+prevK]
		prevY := prevX - prevK

		for x > prevX && y > prevY {
			x--
			y--
		}

		if d == 0 {
			break
		}

		if x == prevX {
			y--
			changes = append(changes, Change{
				P1:  P1 + x,
				P2:  P2 + y,
				Ins: 1,
			})
		} else {
			x--
			changes = append(changes, Change{
				P1:  P1 + x,
				P2:  P2 + y,
				Del: 1,
			})
		}
	}

	for i, j := 0, len(changes)-1; i < j; i, j = i+1, j-1 {
		changes[i], changes[j] = changes[j], changes[i]
	}

	return mergeChangesFast(changes), nil
}

func mergeChangesFast(ch []Change) []Change {
	if len(ch) == 0 {
		return ch
	}

	out := make([]Change, 0, len(ch))
	cur := ch[0]

	for i := 1; i < len(ch); i++ {
		n := ch[i]

		if cur.P1+cur.Del == n.P1 && cur.P2+cur.Ins == n.P2 {
			cur.Del += n.Del
			cur.Ins += n.Ins
		} else {
			out = append(out, cur)
			cur = n
		}
	}

	out = append(out, cur)
	return out
}

func generateTestLines(n int) []string {
	lines := make([]string, n)
	for i := range n {
		lines[i] = randStringBench(20)
	}
	return lines
}

func randStringBench(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func BenchmarkMyersOriginal(b *testing.B) {
	ctx := context.Background()
	a := generateTestLines(1000)
	c := make([]string, len(a))
	copy(c, a)
	// 10% modification
	for range 100 {
		idx := rand.Intn(len(c))
		c[idx] = randStringBench(20)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = myersCompute(ctx, a, 0, c, 0)
	}
}

func BenchmarkMyersFast(b *testing.B) {
	ctx := context.Background()
	a := generateTestLines(1000)
	c := make([]string, len(a))
	copy(c, a)
	// 10% modification
	for range 100 {
		idx := rand.Intn(len(c))
		c[idx] = randStringBench(20)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = myersFast(ctx, a, 0, c, 0)
	}
}

func BenchmarkMyersOriginalLarge(b *testing.B) {
	ctx := context.Background()
	a := generateTestLines(5000)
	c := make([]string, len(a))
	copy(c, a)
	for range 500 {
		idx := rand.Intn(len(c))
		c[idx] = randStringBench(20)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = myersCompute(ctx, a, 0, c, 0)
	}
}

func BenchmarkMyersFastLarge(b *testing.B) {
	ctx := context.Background()
	a := generateTestLines(5000)
	c := make([]string, len(a))
	copy(c, a)
	for range 500 {
		idx := rand.Intn(len(c))
		c[idx] = randStringBench(20)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = myersFast(ctx, a, 0, c, 0)
	}
}
