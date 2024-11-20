package filemode

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

func TestFragments(t *testing.T) {
	mode := Executable | Fragments
	fmt.Fprintf(os.Stderr, "mode: %o\n", mode)
	if mode&Executable != 0 {
		fmt.Fprintf(os.Stderr, "Execute: %o\n", mode)
	}
	if mode&Regular != 0 {
		fmt.Fprintf(os.Stderr, "mode: %o\n", mode)
	}
	fmt.Fprintf(os.Stderr, "mode: %o: %o\n", mode^Fragments, Fragments^0170000)
}

func TestFragments2(t *testing.T) {
	ms := []FileMode{
		Regular,
		Regular | Fragments,
		Executable,
		Executable | Fragments,
		Dir,
		Dir | Fragments,
		Symlink,
		Symlink | Fragments,
		Submodule,
		Submodule | Fragments,
	}
	for _, m := range ms {
		om, err := m.ToOSFileMode()
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad filemode: %v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "%s --> %s\n", m, om)
	}
}

func TestFileModeJSON(t *testing.T) {
	type J struct {
		A FileMode `json:"a"`
	}
	j := &J{
		A: Executable,
	}
	var s strings.Builder
	_ = json.NewEncoder(io.MultiWriter(&s, os.Stderr)).Encode(j)
	var j2 J

	if err := json.NewDecoder(strings.NewReader(s.String())).Decode(&j2); err != nil {
		return
	}
	fmt.Fprintf(os.Stderr, "III: %s\n", j2.A)
}
