package tr

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"testing"

	"github.com/antgroup/hugescm/pkg/tr/locale"
	"golang.org/x/text/language"
)

func TestFS(t *testing.T) {
	_ = Initialize()
	fmt.Fprintf(os.Stderr, "load ok={%v}\n", W("ok"))
	fmt.Fprintf(os.Stderr, "%s\n", W("Descending order by total size:"))
	_, _ = Fprintf(os.Stderr, "current os '%s'\n", runtime.GOOS)
}

func TestLANG(t *testing.T) {
	_ = os.Setenv("LC_ALL", "zh_CN.UTF8")
	_ = Initialize()
	fmt.Fprintf(os.Stderr, "load ok={%v}\n", W("ok"))
	_, _ = Fprintf(os.Stderr, "current os '%s'\n", runtime.GOOS)
}

func TestLocale(t *testing.T) {
	tag, err := locale.Detect()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Fprintf(os.Stderr, "%s\n", tag.String())
}

func TestLocale2(t *testing.T) {
	tag := language.Make("zh-Hans-US")
	tag2 := language.Make("zh-CN")
	base, c := tag.Base()
	fmt.Fprintf(os.Stderr, "%s %s %s\n", base.String(), c.String(), tag2)
}
