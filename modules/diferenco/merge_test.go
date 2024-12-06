package diferenco

import (
	"context"
	"fmt"
	"os"
	"testing"
)

func TestMerge(t *testing.T) {
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
	content, conflict, err := Merge(context.Background(), textO, textA, textB, "o.txt", "a.txt", "b.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)
}

func TestMerge2(t *testing.T) {
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

	content, conflict, err := Merge(context.Background(), textO, textA, textA, "o.txt", "a.txt", "b.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)
}

func TestMerge3(t *testing.T) {
	const textO = `celery
garlic
onions
salmon
tomatoes
wine
`

	const textA = `celery
garlic
onions
salmon
tomatoes
wine
0000
00000
`

	const textB = `celery
garlic
onions
salmon
tomatoes
wine
0000
00000
77777
`

	content, conflict, err := Merge(context.Background(), textO, textA, textB, "o.txt", "a.txt", "b.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)
}
