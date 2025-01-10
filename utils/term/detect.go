package main

import (
	"fmt"
	"os"

	"github.com/antgroup/hugescm/modules/term"
)

func main() {
	fmt.Fprintf(os.Stderr, "IsCygwinTerminal: %v\n", term.IsCygwinTerminal(os.Stderr.Fd()))
}
