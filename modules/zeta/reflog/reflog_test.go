package reflog

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

func TestReflogRead(t *testing.T) {
	m := `0000000000000000000000000000000000000000000000000000000000000000 7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a LBW <dev@zeta.io> 1706772738 +0800	WIP on master: 8438002 form-string.md: correct the example
7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a 46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 LBW <dev@zeta.io> 1706772760 +0800	WIP on master: 8438002 form-string.md: correct the example
46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 c0869060ede3e208c464cac81fd78e6f31cecb572a3450b9a7dce4784c6dab5f LBW <dev@zeta.io> 1706773202 +0800	WIP on master: d343999 ZZZZ
`
	d := &DB{}
	entries, err := d.parse(strings.NewReader(m))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		return
	}
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "%s\n", e.Message)
	}
	_ = d.serialize(os.Stderr, entries)
}

func TestReflogWrite(t *testing.T) {
	m := `0000000000000000000000000000000000000000000000000000000000000000 7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a LBW <dev@zeta.io> 1706772738 +0800	WIP on master: 8438002 form-string.md: correct the example
7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a 46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 LBW <dev@zeta.io> 1706772760 +0800	WIP on master: 8438002 form-string.md: correct the example
46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 c0869060ede3e208c464cac81fd78e6f31cecb572a3450b9a7dce4784c6dab5f LBW <dev@zeta.io> 1706773202 +0800	WIP on master: d343999 ZZZZ
`
	d := &DB{root: "/tmp"}
	entries, err := d.parse(strings.NewReader(m))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		return
	}
	for _, e := range entries {
		fmt.Fprintf(os.Stderr, "%s\n", e.Message)
	}
	o := &Reflog{name: "stash", Entries: entries}
	if err := d.Write(o); err != nil {
		fmt.Fprintf(os.Stderr, "write reflog: %v\n", err)
	}
}

func TestReflogDrop(t *testing.T) {
	m := `0000000000000000000000000000000000000000000000000000000000000000 7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a LBW <dev@zeta.io> 1706772738 +0800	WIP on master: 8438002 form-string.md: correct the example
7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a 46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 LBW <dev@zeta.io> 1706772760 +0800	WIP on master: 8438002 form-string.md: correct the example
46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 c0869060ede3e208c464cac81fd78e6f31cecb572a3450b9a7dce4784c6dab5f LBW <dev@zeta.io> 1706773202 +0800	WIP on master: d343999 ZZZZ
`
	d := &DB{}
	entries, err := d.parse(strings.NewReader(m))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		return
	}
	log := &Reflog{
		name:    "refs/stash",
		Entries: entries,
	}
	_ = log.Drop(0, true)
	_ = d.serialize(os.Stderr, log.Entries)
}
func TestReflogDrop1(t *testing.T) {
	m := `0000000000000000000000000000000000000000000000000000000000000000 7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a LBW <dev@zeta.io> 1706772738 +0800	WIP on master: 8438002 form-string.md: correct the example
7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a 46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 LBW <dev@zeta.io> 1706772760 +0800	WIP on master: 8438002 form-string.md: correct the example
46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 c0869060ede3e208c464cac81fd78e6f31cecb572a3450b9a7dce4784c6dab5f LBW <dev@zeta.io> 1706773202 +0800	WIP on master: d343999 ZZZZ
`
	d := &DB{}
	entries, err := d.parse(strings.NewReader(m))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		return
	}
	log := &Reflog{
		name:    "refs/stash",
		Entries: entries,
	}
	_ = log.Drop(1, true)
	_ = d.serialize(os.Stderr, log.Entries)
}

func TestReflogDrop2(t *testing.T) {
	m := `0000000000000000000000000000000000000000000000000000000000000000 7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a LBW <dev@zeta.io> 1706772738 +0800	WIP on master: 8438002 form-string.md: correct the example
7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a 46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 LBW <dev@zeta.io> 1706772760 +0800	WIP on master: 8438002 form-string.md: correct the example
46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 c0869060ede3e208c464cac81fd78e6f31cecb572a3450b9a7dce4784c6dab5f LBW <dev@zeta.io> 1706773202 +0800	WIP on master: d343999 ZZZZ
`
	d := &DB{}
	entries, err := d.parse(strings.NewReader(m))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		return
	}
	log := &Reflog{
		name:    "refs/stash",
		Entries: entries,
	}
	_ = log.Drop(2, true)
	_ = d.serialize(os.Stderr, log.Entries)
}

func TestReflogDrop3(t *testing.T) {
	m := `0000000000000000000000000000000000000000000000000000000000000000 7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a LBW <dev@zeta.io> 1706772738 +0800	WIP on master: 8438002 form-string.md: correct the example
7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a 46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 LBW <dev@zeta.io> 1706772760 +0800	WIP on master: 8438002 form-string.md: correct the example
46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 c0869060ede3e208c464cac81fd78e6f31cecb572a3450b9a7dce4784c6dab5f LBW <dev@zeta.io> 1706773202 +0800	WIP on master: d343999 ZZZZ
`
	d := &DB{}
	entries, err := d.parse(strings.NewReader(m))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		return
	}
	log := &Reflog{
		name:    "refs/stash",
		Entries: entries,
	}
	_ = log.Drop(3, true)
	_ = d.serialize(os.Stderr, log.Entries)
}

func TestReflogPush(t *testing.T) {
	m := `0000000000000000000000000000000000000000000000000000000000000000 7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a LBW <dev@zeta.io> 1706772738 +0800	WIP on master: 8438002 form-string.md: correct the example
7d93f7dad4160ce2a30e7083e1fbe189b68142bcefd029fdc376f892eedb250a 46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 LBW <dev@zeta.io> 1706772760 +0800	WIP on master: 8438002 form-string.md: correct the example
46ec16b743c9020366a11f9cb3ea61f1ec04ca6d588132eff4c5028a2a49a815 c0869060ede3e208c464cac81fd78e6f31cecb572a3450b9a7dce4784c6dab5f LBW <dev@zeta.io> 1706773202 +0800	WIP on master: d343999 ZZZZ
`
	d := &DB{}
	entries, err := d.parse(strings.NewReader(m))
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse error: %s\n", err)
		return
	}
	log := &Reflog{
		name:    "refs/stash",
		Entries: entries,
	}
	log.Push(plumbing.NewHash("bd9ddb6547b224fd6bb39b7f7fddf833b37f4ddb9ea94be8628c3f7aae465e64"), &object.Signature{
		Name:  "LBW",
		Email: "dev@zeta.io",
		When:  time.Now(),
	}, "PushE")
	_ = d.serialize(os.Stderr, log.Entries)
}
