package diferenco

import (
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
salmon
garlic
onions
tomatoes
wine
`

	content, conflict, err := DefaultMerge(t.Context(), textO, textA, textB, "o.txt", "a.txt", "b.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)

	content, conflict, err = Merge(t.Context(), &MergeOptions{TextO: textO, TextA: textA, TextB: textB, LabelO: "o.txt", LabelA: "a.txt", LabelB: "b.txt", Style: STYLE_ZEALOUS_DIFF3})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "ZEALOUS_DIFF3\n%s\nconflicts: %v\n", content, conflict)

	content, conflict, err = Merge(t.Context(), &MergeOptions{TextO: textO, TextA: textA, TextB: textB, LabelO: "o.txt", LabelA: "a.txt", LabelB: "b.txt", Style: STYLE_DIFF3})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "DIFF3\n%s\nconflicts: %v\n", content, conflict)
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

	content, conflict, err := DefaultMerge(t.Context(), textO, textA, textA, "o.txt", "a.txt", "b.txt")
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

	content, conflict, err := DefaultMerge(t.Context(), textO, textA, textB, "o.txt", "a.txt", "b.txt")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)

	content, conflict, err = Merge(t.Context(), &MergeOptions{TextO: textO, TextA: textA, TextB: textB, LabelO: "o.txt", LabelA: "a.txt", LabelB: "b.txt", Style: STYLE_ZEALOUS_DIFF3})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)

	content, conflict, err = Merge(t.Context(), &MergeOptions{TextO: textO, TextA: textA, TextB: textB, LabelO: "o.txt", LabelA: "a.txt", LabelB: "b.txt", Style: STYLE_DIFF3})
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\nconflicts: %v\n", content, conflict)

}
