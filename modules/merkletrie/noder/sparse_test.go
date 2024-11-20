package noder

import (
	"fmt"
	"os"
	"path"
	"testing"
)

func TestNewSparseTreeMatcher(t *testing.T) {
	tt := NewSparseTreeMatcher([]string{"dir3", "dir4/abc", "abcd/efgh/mnopq"})
	fmt.Fprintf(os.Stderr, "%d\n", tt.Len())
}

func TestPathDir(t *testing.T) {
	dirs := []string{
		"a.txt",
		"abc/abc.txt",
	}
	for _, d := range dirs {
		fmt.Fprintf(os.Stderr, "%s\n", path.Dir(d))
	}
}

func TestSparseMatcher(t *testing.T) {
	ss := []string{".aci.yml",
		".dailyCheck.aci.yml",
		".dailyTest.aci.yml",
		".gitignore",
		".ignore_pr.yml",
		"sigma/appops/OWNERS",
		"sigma/appops/intelligent_engine/abc.txt",
		"sigma/appops/intelligent_engine/business_intelligence-recommendation_engine/tapeargo/OWNERS",
		"sigma/appops/intelligent_engine/business_intelligence-recommendation_engine/tapeargo/README.md",
		"sigma/appops/intelligent_engine/business_intelligence-recommendation_engine/tapeargo/base/base.k",
		"sigma/appops/jackson/business_intelligence-recommendation_engine/tapeargo/OWNERS",
		"sigma/appops/jackson/business_intelligence-recommendation_engine/tapeargo/README.md",
		"sigma/appops/jackson/business_intelligence-recommendation_engine/tapeargo/base/base.k",
		"docs/dev.md",
	}
	m := NewSparseMatcher([]string{"sigma/appops/intelligent_engine"})
	for _, s := range ss {
		fmt.Fprintf(os.Stderr, "Matched: %v %s\n", m.Match(s), s)
	}
}
