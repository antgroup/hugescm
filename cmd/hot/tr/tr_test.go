package tr

import (
	"fmt"
	"os"
	"runtime"
	"testing"
)

func TestFS(t *testing.T) {
	_ = DelayInitializeLocale()
	langTable["ok"] = "确定"
	fmt.Fprintf(os.Stderr, "load ok=%s\n", W("ok"))
	fmt.Fprintf(os.Stderr, "%s\n", W("Descending order by total size:"))
	Fprintf(os.Stderr, "current os '%s'\n", runtime.GOOS)
}

func TestLANG(t *testing.T) {
	os.Setenv("LC_ALL", "zh_CN.UTF8")
	_ = DelayInitializeLocale()
	fmt.Fprintf(os.Stderr, "load ok={%v}\n", W("ok"))
	Fprintf(os.Stderr, "current os '%s'\n", runtime.GOOS)
}
