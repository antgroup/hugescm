package areas

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/modules/plumbing"
)

type Area struct {
	Name    string    `toml:"name,omitempty"`
	BasedOn string    `toml:"based_on"`
	Tree    string    `toml:"tree"`
	Updated time.Time `toml:"updated"`
	Message string    `toml:"message,omitempty"`
}

type Areas struct {
	Areas  []Area `toml:"areas"`
	Stashs []Area `toml:"stashs"`
}

func TestAreas(t *testing.T) {
	areas := []Area{
		{
			Name:    "improve-code-review-12",
			BasedOn: "364c313a43cd33cc51c4292cc9a390fc444300554efe4306e20c8c61a8017bce",
			Tree:    "b72cedec430dba1c40f4b8300da45c5fcd616f38f4ea0aff332ae5d165cdbda3",
			Updated: time.Now().Add(-time.Hour * 24),
		},
		{
			Name:    "fix-bug-review-7",
			BasedOn: "364c313a43cd33cc51c4292cc9a390fc444300554efe4306e20c8c61a8017bce",
			Tree:    "5e901770dcb3b903ac44d4f3ae20236458833b54919581c1fa462ea0f5acb966",
			Updated: time.Now().Add(-time.Hour * 7),
		},
	}
	stashs := []Area{
		{
			BasedOn: "364c313a43cd33cc51c4292cc9a390fc444300554efe4306e20c8c61a8017bce",
			Tree:    "b72cedec430dba1c40f4b8300da45c5fcd616f38f4ea0aff332ae5d165cdbda3",
			Updated: time.Now().Add(-time.Hour * 12),
			Message: "WIP on mainline: 364c313a43cd update code",
		},
		{
			BasedOn: "364c313a43cd33cc51c4292cc9a390fc444300554efe4306e20c8c61a8017bce",
			Tree:    "5e901770dcb3b903ac44d4f3ae20236458833b54919581c1fa462ea0f5acb966",
			Updated: time.Now().Add(-time.Hour * 2),
			Message: "WIP on mainline: 364c313a43cd update code",
		},
	}
	s := &Areas{
		Areas:  areas,
		Stashs: stashs,
	}

	var b strings.Builder
	if err := toml.NewEncoder(&b).Encode(s); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\n", b.String())
}

type RebaseMD struct {
	REBASE_HEAD plumbing.Hash   `toml:"REBASE_HEAD"`
	ONTO        plumbing.Hash   `toml:"ONTO"`
	MERGED      []plumbing.Hash `toml:"MERGED"`
}

func TestMD(t *testing.T) {
	d := &RebaseMD{
		REBASE_HEAD: plumbing.NewHash("222737e6c1be7b0794727929cb2d0ad0a07f3212106a94e333d7c38c3c4830f7"),
		ONTO:        plumbing.NewHash("392a1db47dab4d005f1d0ce6fb4c2f048d37e98d49441945b7e0d3dcbf1c40e0"),
		MERGED: []plumbing.Hash{
			plumbing.NewHash("688be8b31dc3dae7c597c133512e3943e1d72d1a355a54a924ac08327b423437"),
			plumbing.NewHash("ad1d4264cc3bc5437fcbce9c27f2865f1761140f2d6f864d75e0db317e941031"),
		},
	}
	if err := toml.NewEncoder(os.Stderr).Encode(d); err != nil {
		fmt.Fprintf(os.Stderr, "encode error: %v\n", err)
		return
	}
	var d2 RebaseMD
	m := `REBASE_HEAD="392a1db47dab4d005f1d0ce6fb4c2f048d37e98d49441945b7e0d3dcbf1c40e0"`
	if _, err := toml.NewDecoder(strings.NewReader(m)).Decode(&d2); err != nil {
		fmt.Fprintf(os.Stderr, "decode error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%s\n", d2.REBASE_HEAD)
}
