package tr

import (
	"embed"
	"fmt"
	"io"
	"path"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/modules/locale"
)

//go:embed languages
var langFS embed.FS

var (
	langTable = make(map[string]any)
)

func parseLocale() string {
	t, err := locale.Detect()
	if err != nil {
		return "en-US"
	}
	lang := t.String()
	switch {
	case strings.HasPrefix(lang, "zh-Hans"):
		return "zh-CN"
		// TODO FIXME
	}
	return lang
}

func DelayInitializeLocale() error {
	fd, err := langFS.Open(path.Join("languages", parseLocale()+".toml"))
	if err != nil {
		return err
	}
	defer fd.Close() // nolint
	if _, err := toml.NewDecoder(fd).Decode(&langTable); err != nil {
		return err
	}
	return nil
}

func DefaultLocaleName() string {
	return parseLocale()
}

func W(k string) string {
	if v, ok := langTable[k]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return k
}

func Fprintf(w io.Writer, format string, a ...any) (n int, err error) {
	return fmt.Fprintf(w, W(format), a...)
}
