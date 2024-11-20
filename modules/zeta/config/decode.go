// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/modules/strengthen"
)

const (
	ENV_ZETA_CONFIG_SYSTEM = "ZETA_CONFIG_SYSTEM"
)

var (
	ErrKeyNotFound = errors.New("key not found")
)

func configSystemPath() string {
	if p, ok := os.LookupEnv(ENV_ZETA_CONFIG_SYSTEM); ok {
		return p
	}
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	// zeta prefix -->
	prefix := filepath.Dir(exe)
	if filepath.Base(prefix) == "bin" {
		prefix = filepath.Dir(prefix)
	}
	return filepath.Join(prefix, "/etc/zeta.toml")
}

func LoadSystem() (*Config, error) {
	systemPath := configSystemPath()
	if len(systemPath) == 0 {
		return nil, os.ErrNotExist
	}
	var cfg Config
	if _, err := os.Stat(systemPath); err != nil {
		return nil, err
	}
	if _, err := toml.DecodeFile(systemPath, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func LoadGlobal() (*Config, error) {
	var cfg Config
	userPath := strengthen.ExpandPath("~/.zeta.toml")
	if _, err := os.Stat(userPath); err != nil && os.IsNotExist(err) {
		return &cfg, nil
	}
	if _, err := toml.DecodeFile(userPath, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func LoadBaseline() (*Config, error) {
	gc, err := LoadGlobal()
	if err != nil {
		return nil, err
	}
	cfg, err := LoadSystem()
	if os.IsNotExist(err) {
		return gc, nil
	}
	if err != nil {
		return nil, err
	}
	cfg.Overwrite(gc)
	return cfg, nil
}

func Load(zetaDir string) (*Config, error) {
	cfg, err := LoadBaseline()
	if err != nil {
		return nil, err
	}
	if len(zetaDir) == 0 {
		return cfg, nil
	}
	var rc Config
	if _, err := toml.DecodeFile(filepath.Join(zetaDir, "zeta.toml"), &rc); err != nil {
		return nil, err
	}
	cfg.Overwrite(&rc)
	return cfg, nil
}
