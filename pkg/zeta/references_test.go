package zeta

import (
	"fmt"
	"os"
	"sort"
	"testing"
	"time"
)

func TestSort(t *testing.T) {
	ss := []string{
		"2006-01-02",
		"2006-01-03",
		"2006-01-04",
		"2006-01-05",
		"2005-11-02",
		"2023-12-12",
	}
	times := make([]time.Time, 0, len(ss))
	for _, s := range ss {
		t, err := time.Parse(time.DateOnly, s)
		if err == nil {
			times = append(times, t)
		}
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i].Before(times[j])
	})
	for _, t := range times {
		fmt.Fprintf(os.Stderr, "S1: %s\n", t)
	}
	sort.Slice(times, func(i, j int) bool {
		return times[i].After(times[j])
	})
	for _, t := range times {
		fmt.Fprintf(os.Stderr, "S2: %s\n", t)
	}
}
