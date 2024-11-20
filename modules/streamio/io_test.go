package streamio

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestReadMax(t *testing.T) {
	text := `XZXdewdieded3oifdjfrf4frewfrfreferwfgrewfreferferfdedoidqjwqdjqedo3qjhd3hqdiwqehdro3eidhewdiehdbweqdgewdgewdedewgdbe`
	b, err := ReadMax(strings.NewReader(text), 10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v", err)
		return
	}
	fmt.Fprintf(os.Stderr, "length: %d\n", len(b))
}

func TestGrowReadMax(t *testing.T) {
	text := `XZXdewdieded3oifdjfrf4frewfrfreferwfgrewfreferferfdedoidqjwqdjqedo3qjhd3hqdiwqehdro3eidhewdiehdbweqdgewdgewdedewgdbe`
	b, err := GrowReadMax(strings.NewReader(text), 50, 10)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v", err)
		return
	}
	fmt.Fprintf(os.Stderr, "length: %d\n", len(b))
}
