// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/pkg/serve"
)

type pseudoConfig struct {
	DecryptedKey string `toml:"decrypted_key,omitempty"`
}

func (pc *pseudoConfig) Decode(cfg string, expandEnv bool) error {
	r, err := serve.NewExpandReader(cfg, expandEnv)
	if err != nil {
		return err
	}
	defer r.Close()
	if _, err := toml.NewDecoder(r).Decode(pc); err != nil {
		return err
	}
	return nil
}

type Encrypt struct {
	Source      string `arg:"" name:"source" help:"source text to be encrypted"`
	FromEnv     bool   `short:"s" name:"from-env" help:"read source text from environment variable"`
	FromFile    bool   `short:"p" name:"from-file" help:"read source text from a file"`
	Config      string `short:"c" name:"config" optional:"" help:"Location of server config file" type:"path"`
	Destination string `short:"d" name:"destination" optional:"" help:"save variable to specified file"`
}

func (c *Encrypt) Run(globals *Globals) error {
	var pc pseudoConfig
	if err := pc.Decode(c.Config, globals.ExpandEnv); err != nil {
		fmt.Fprintf(os.Stderr, "load config error: %v\n", err)
		return err
	}
	source, err := func() (string, error) {
		if c.FromFile {
			fd, err := os.Open(c.Source)
			if err != nil {
				return "", err
			}
			defer fd.Close()
			si, err := fd.Stat()
			if err != nil {
				return "", err
			}
			if sz := si.Size(); sz > serve.MiByte {
				return "", fmt.Errorf("file size too large: %s", strengthen.FormatSize(sz))
			}
			b, err := io.ReadAll(fd)
			if err != nil {
				return "", err
			}
			return string(b), nil
		}
		if c.FromEnv {
			return os.Getenv(c.Source), nil
		}
		return c.Source, nil
	}()
	if err != nil {
		fmt.Fprintf(os.Stderr, "read from file error: %v\n", err)
		return err
	}
	secret, err := serve.Encrypt(source, pc.DecryptedKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "encrypt error: %v\n", err)
		return err
	}
	if len(c.Destination) == 0 {
		fmt.Fprintln(os.Stdout, secret)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(c.Destination), 0755); err != nil {
		fmt.Fprintf(os.Stderr, "write secret error: %v\n", err)
		return err
	}
	fd, err := os.Create(c.Destination)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create secret file error: %v\n", err)
		return err
	}
	defer fd.Close()
	if _, err := fd.WriteString(secret); err != nil {
		fmt.Fprintf(os.Stderr, "write secret to file error: %v\n", err)
		return err
	}
	return nil
}
