package strengthen

import (
	"fmt"
	"os"
	"os/user"
	"testing"
)

func TestExpandPath(t *testing.T) {
	u, err := user.Current()
	if err != nil {
		return
	}
	dirs := []string{
		"~/.zetaignore", "~" + u.Username + "/jacksone", "/tmp/jock", "~root/downloads",
	}
	for _, d := range dirs {
		fmt.Fprintf(os.Stderr, "%s --> %s\n", d, ExpandPath(d))
	}
}
