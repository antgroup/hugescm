package main

import (
	"fmt"
	"os"
)

type Result struct {
	buffer1index int
	buffer2index int
	chain        *Result
}

func LCS(buffer1, buffer2 []rune) *Result {
	equivalenceClasses := make(map[rune][]int)
	for j, item := range buffer2 {
		equivalenceClasses[item] = append(equivalenceClasses[item], j)
	}

	NULLRESULT := &Result{buffer1index: -1, buffer2index: -1, chain: nil}
	candidates := []*Result{NULLRESULT}

	for i, item := range buffer1 {
		buffer2indices := equivalenceClasses[item]
		r := 0
		c := candidates[0]

		for _, j := range buffer2indices {
			var s int
			for s = r; s < len(candidates); s++ {
				if (candidates[s].buffer2index < j) && (s == len(candidates)-1 || candidates[s+1].buffer2index > j) {
					break
				}
			}
			if s < len(candidates) {
				newCandidate := &Result{buffer1index: i, buffer2index: j, chain: candidates[s]}
				if r == len(candidates) {
					candidates = append(candidates, c)
				} else {
					candidates[r] = c
				}
				r = s + 1
				c = newCandidate
				if r == len(candidates) {
					break // no point in examining further (j)s
				}
			}
		}
		if r < len(candidates) {
			candidates[r] = c
		} else {
			candidates = append(candidates, c)
		}
	}
	// The LCS is the reverse of the linked-list through .chain of candidates[len(candidates) - 1].
	return candidates[len(candidates)-1]
}

func main() {
	// Example usage:
	buffer1 := []rune("ACCGGTCGAGTGCGCGGAAGCCGGCCGAA")
	buffer2 := []rune("GTCGTTCGGAATGCCGTTGCTCTGTAAA")
	lcsResult := LCS(buffer1, buffer2)

	// If you want to extract the actual LCS string, you would need to follow the .chain
	// to reconstruct it from `lcsResult`.
	fmt.Fprintf(os.Stderr, "%v\n", lcsResult)
}
