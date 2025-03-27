// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/modules/strengthen"
)

func atomicEncode(zf string, a any) error {
	name, err := func() (string, error) {
		now := time.Now()
		zfd := filepath.Dir(zf)
		_ = os.MkdirAll(zfd, 0755)
		cachePath := fmt.Sprintf("%s/.zeta-%d.toml", zfd, now.UnixNano())
		fd, err := os.Create(cachePath)
		if err != nil {
			return "", err
		}
		defer fd.Close() // nolint
		enc := toml.NewEncoder(fd)
		enc.Indent = ""
		if err := enc.Encode(a); err != nil {
			return cachePath, err
		}
		return cachePath, nil
	}()
	if err != nil {
		if len(name) != 0 {
			_ = os.Remove(name)
		}
		return err
	}
	if err := os.Rename(name, zf); err != nil {
		_ = os.Remove(name)
		return err
	}
	return nil
}

func Encode(zetaDir string, config *Config) error {
	if config == nil || len(zetaDir) == 0 {
		return ErrInvalidArgument
	}
	zf := filepath.Join(zetaDir, "zeta.toml")
	return atomicEncode(zf, config)
}

func EncodeGlobal(config *Config) error {
	if config == nil {
		return ErrInvalidArgument
	}
	zfg := strengthen.ExpandPath("~/.zeta.toml")
	return atomicEncode(zfg, config)
}

type UpdateOptions struct {
	Values map[string]any
	Append bool
}

func updateInternal(zf string, opts *UpdateOptions) error {
	if opts == nil || opts.Values == nil {
		return errors.New("invalid argument for update config")
	}
	md := make(Sections)
	if _, err := toml.DecodeFile(zf, &md); err != nil && !os.IsNotExist(err) {
		return err
	}
	for k, v := range opts.Values {
		if _, err := md.updateKey(k, v, opts.Append); err != nil {
			return err
		}
	}
	return atomicEncode(zf, md)
}

func UpdateSystem(opts *UpdateOptions) error {
	zfg := configSystemPath()
	return updateInternal(zfg, opts)
}

func UpdateGlobal(opts *UpdateOptions) error {
	zfg := strengthen.ExpandPath("~/.zeta.toml")
	return updateInternal(zfg, opts)
}

func UpdateLocal(zetaDir string, opts *UpdateOptions) error {
	zf := filepath.Join(zetaDir, "zeta.toml")
	return updateInternal(zf, opts)
}

func unsetInternal(zf string, keys ...string) error {
	if len(keys) == 0 {
		return nil
	}
	md := make(Sections)
	if _, err := toml.DecodeFile(zf, &md); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, k := range keys {
		if _, err := md.deleteKey(k); err != nil && err != ErrKeyNotFound {
			return err
		}
	}
	return atomicEncode(zf, md)
}

func UnsetSystem(keys ...string) error {
	zfg := configSystemPath()
	return unsetInternal(zfg, keys...)
}

func UnsetGlobal(keys ...string) error {
	zfg := strengthen.ExpandPath("~/.zeta.toml")
	return unsetInternal(zfg, keys...)
}

func UnsetLocal(zetaDir string, keys ...string) error {
	zf := filepath.Join(zetaDir, "zeta.toml")
	return unsetInternal(zf, keys...)
}
