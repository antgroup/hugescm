// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package replay

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/antgroup/hugescm/modules/git/gitobj"
	"github.com/antgroup/hugescm/modules/wildmatch"
)

type Matcher interface {
	Match(entry *gitobj.TreeEntry, absPath string) bool
}

type equaler struct {
	paths map[string]any
}

func NewEqualer(paths []string) Matcher {
	e := &equaler{
		paths: make(map[string]any),
	}
	for _, p := range paths {
		e.paths[p] = nil
	}
	return e
}

func (e *equaler) Match(entry *gitobj.TreeEntry, absPath string) bool {
	if _, ok := e.paths[absPath]; ok {
		return true
	}
	return false
}

var (
	caseInsensitive = func() bool {
		return runtime.GOOS == "windows" || runtime.GOOS == "darwin"
	}()
	escapeChars = func() string {
		switch runtime.GOOS {
		case "windows":
			return "*?[]"
		default:
		}

		return "*?[]\\"
	}()
)

func systemCaseEqual(a, b string) bool {
	if caseInsensitive {
		return strings.EqualFold(a, b)
	}
	return a == b
}

type matcher struct {
	prefix []string
	ws     []*wildmatch.Wildmatch
}

func NewMatcher(patterns []string) Matcher {
	m := &matcher{}
	for _, pattern := range patterns {
		if len(pattern) == 0 {
			continue
		}
		if !strings.ContainsAny(pattern, escapeChars) {
			m.prefix = append(m.prefix, strings.TrimSuffix(pattern, "/"))
			continue
		}
		w, err := wildmatch.NewWildmatch(pattern, wildmatch.SystemCase, wildmatch.Contents)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Ignore bad wildcard '%s' error: %v\n", pattern, err)
			continue
		}
		m.ws = append(m.ws, w)
	}
	return m
}

func (m *matcher) Match(entry *gitobj.TreeEntry, absPath string) bool {
	if len(m.ws) == 0 && len(m.prefix) == 0 {
		return true
	}
	for _, p := range m.prefix {
		prefixLen := len(p)
		if len(absPath) >= prefixLen && systemCaseEqual(absPath[0:prefixLen], p) && (len(absPath) == prefixLen || absPath[prefixLen] == '/') {
			return true
		}
	}
	for _, w := range m.ws {
		if w.Match(absPath) {
			return true
		}
	}
	return false
}
