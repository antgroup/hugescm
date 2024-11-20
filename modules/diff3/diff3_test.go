package diff3_test

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/diff3"
)

const textO = `celery
garlic
onions
salmon
tomatoes
wine
`

const textA = `celery
salmon
tomatoes
garlic
onions
wine
`

const textB = `celery
garlic
salmon
tomatoes
onions
wine
`

func TestMerge(t *testing.T) {
	result, err := diff3.Merge(strings.NewReader(textA), strings.NewReader(textO), strings.NewReader(textB), true, "a", "a")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	content, _ := io.ReadAll(result.Result)
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, result.Conflicts)
	fmt.Fprintf(os.Stderr, "-----------------------------------------\n")
	result, err = diff3.Merge(strings.NewReader(textA), strings.NewReader(textO), strings.NewReader(textB), false, "a", "a")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	content, _ = io.ReadAll(result.Result)
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, result.Conflicts)
}

func TestSimpleMerge(t *testing.T) {
	content, conflict, err := diff3.SimpleMerge(context.Background(), textO, textA, textB, "", "a", "a")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)
}
