// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package tr

import (
	"embed"
	"fmt"
	"io"
	"path"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/pkg/tr/locale"
)

//go:embed languages
var langFS embed.FS

var (
	langTable = make(map[string]any)
)

var (
	Language = sync.OnceValue(func() string {
		t, err := locale.Detect()
		if err != nil {
			return "en-US"
		}
		lang := t.String()
		switch {
		case strings.HasPrefix(lang, "zh-Hans"):
			return "zh-CN"
		}
		return lang
	})
)

var (
	Initialize = sync.OnceValue(func() error {
		fd, err := langFS.Open(path.Join("languages", Language()+".toml"))
		if err != nil {
			return err
		}
		defer fd.Close() // nolint
		if _, err := toml.NewDecoder(fd).Decode(&langTable); err != nil {
			return err
		}
		return nil
	})
)

func translate(k string) string {
	if v, ok := langTable[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return k
}

func W(k string) string {
	return translate(k)
}

func Fprintf(w io.Writer, format string, a ...any) (n int, err error) {
	return fmt.Fprintf(w, translate(format), a...)
}

func Sprintf(format string, a ...any) string {
	return fmt.Sprintf(translate(format), a...)
}
