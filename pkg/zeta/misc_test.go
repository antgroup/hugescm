package zeta

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"testing"
)

func TestDiscoverZetaDir(t *testing.T) {
	dirs := []string{
		"",
		"/",
		"/tmp",
		"/tmp/zeta-demo",
		"/tmp/zeta-demo/.zeta",
		"/tmp/abc",
		"/tmp/abc/.zeta",
		"/tmp/abc/source/dev",
		"/tmp/abc/zeta-demo",
		"/tmp/abc/zeta-demo/dev",
		"/usr/local",
	}
	for _, d := range dirs {
		w, z, err := FindZetaDir(d)
		if err != nil {
			fmt.Fprintf(os.Stderr, "BAD: not zeta dir: %s %v\n", d, err)
			continue
		}
		fmt.Fprintf(os.Stderr, "OK: %s worktree: %s zetaDir: %s\n", d, w, z)
	}
}

func TestPathClean(t *testing.T) {
	dirs := []string{
		"",
		"/",
		"tmp",
		"tmp/zeta-demo",
		"tmp/zeta-demo/.zeta",
		"tmp/abc",
		"tmp/abc/.zeta",
		"tmp/abc/source/dev",
		"tmp/abc/zeta-demo",
		"tmp/abc/zeta-demo/dev",
		"usr/local",
		"sssss////bbbbb",
	}
	for _, d := range dirs {
		fmt.Fprintf(os.Stderr, "%s --> [%s]\n", d, path.Clean(d))
	}
}

func TestMakeXEx(t *testing.T) {
	dd := []string{"tmp/*.cc", "../../", ".", "..", "abc/../*.c"}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "open cwd: %s\n", err)
		return
	}
	for _, d := range dd {
		rel, err := filepath.Rel(cwd, filepath.Join(cwd, d))
		if err != nil {
			fmt.Fprintf(os.Stderr, "bad: %s\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "%s --> %s\n", d, rel)
	}
}

func abc(a, b any) {
	fmt.Fprintf(os.Stderr, "%v %v\n", reflect.TypeOf(a), reflect.TypeOf(b))
}

func TestMatch(t *testing.T) {
	m := NewMatcher([]string{"tmp/*.cc"})
	ss := []string{
		"",
		"/",
		"tmp",
		"tmp/zeta-demo",
		"tmp/zeta-demo/.cc",
		"tmp/helloworld.cc",
		"tmp/abc/.zeta",
		"tmp/abc/source/dev",
		"tmp/abc/zeta-demo",
		"tmp/abc/zeta-demo/dev",
		"usr/local",
		"sssss////bbbbb",
	}
	for _, s := range ss {
		fmt.Fprintf(os.Stderr, "%s --> %v\n", s, m.Match(s))
	}
	a := 9999999999999999.0
	b := 9999999999999998.0
	fmt.Fprintf(os.Stderr, "%v %v\n", 9999999999999999.0-9999999999999998.0, a-b)
	abc(9999999999999999.0-9999999999999998.0, a-b)
}

func TestStringNoCRUD(t *testing.T) {
	sss := []string{"<<<<<<<wqwJack>", "<<<<<<<wqw\nJack>", "<<<<<<<wqwJack>;;;;;", "Jack Jose", "Jaco;JOn"}
	for _, s := range sss {
		fmt.Fprintf(os.Stderr, "%s -> %s\n", s, stringNoCRUD(s))
	}
}

func TestWarn(t *testing.T) {
	warn("WARNING SOME THINGS")
}
