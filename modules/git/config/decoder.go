package config

import (
	"io"
	"os"
	"path/filepath"

	"github.com/antgroup/hugescm/modules/gcfg"
)

// A Decoder reads and decodes config files from an input stream.
type Decoder struct {
	io.Reader
}

// NewDecoder returns a new decoder that reads from r.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r}
}

// Decode reads the whole config from its input and stores it in the
// value pointed to by config.
func (d *Decoder) Decode(config *Config) error {
	cb := func(s string, ss string, k string, v string, bv bool) error {
		if ss == "" && k == "" {
			config.Section(s)
			return nil
		}

		if ss != "" && k == "" {
			config.Section(s).Subsection(ss)
			return nil
		}

		config.AddOption(s, ss, k, v)
		return nil
	}
	return gcfg.ReadWithCallback(d, cb)
}

func BareDecode(repoPath string) (*Config, error) {
	file := filepath.Join(repoPath, "config")
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	cfg := New()
	if err := NewDecoder(fd).Decode(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}
