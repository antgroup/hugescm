package refs

import (
	"fmt"
	"os"
	"testing"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func TestRefRevParseRules(t *testing.T) {
	for _, r := range refRevParseRules {
		fmt.Fprintf(os.Stderr, "%s\n", r.ReferenceName("mainline"))
	}
}

func BenchmarkRepeat(b *testing.B) {
	for b.Loop() {
		for _, r := range refRevParseRules {
			_ = r.ReferenceName("mainline")
		}
	}
}

func BenchmarkRepeat2(b *testing.B) {
	for b.Loop() {
		for _, r := range plumbing.RefRevParseRules {
			_ = fmt.Sprintf(r, "mainline")
		}
	}
}
