package ignore

import (
	"fmt"
	"os"
	"testing"
)

func TestMatch(t *testing.T) {
	p := ParsePattern("**/*lue/vol?ano", nil)
	r := p.Match([]string{"head", "value", "volcano", "tail"}, false)
	fmt.Fprintf(os.Stderr, "%v\n", r)
}
