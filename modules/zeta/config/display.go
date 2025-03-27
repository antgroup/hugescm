// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/trace"
)

type DisplayOptions struct {
	io.Writer
	Z       bool
	Verbose bool
}

const (
	NUL = '\x00'
)

func (opts *DisplayOptions) Show(a any, keys ...string) error {
	prefixKey := strings.Join(keys, ".")
	v := reflect.ValueOf(a)
	switch v.Kind() {
	case reflect.Array:
		for i := range v.Len() {
			if err := opts.Show(v.Index(i).Interface(), keys...); err != nil {
				return err
			}
		}
		return nil
	case reflect.Slice:
		for i := range v.Len() {
			if err := opts.Show(v.Index(i).Interface(), keys...); err != nil {
				return err
			}
		}
		return nil
	case reflect.Struct:
		// don't support
	default:
		// nothing
	}
	if opts.Z {
		_, _ = fmt.Fprintf(opts.Writer, "%s\n%v%c", prefixKey, v, NUL)
		return nil
	}
	_, _ = fmt.Fprintf(opts.Writer, "%s=%v\n", prefixKey, v)
	return nil
}

func (opts *DisplayOptions) DbgPrint(format string, args ...any) {
	if !opts.Verbose {
		return
	}
	trace.DbgPrint(format, args...)
}

func displayTo(d Display, zfg string) error {
	md := make(Sections)
	if _, err := toml.DecodeFile(zfg, &md); err != nil {
		return err
	}
	for sectionKey, s := range md {
		if s == nil {
			continue
		}
		if err := s.displayTo(d, sectionKey); err != nil {
			return err
		}
	}
	return nil
}

func DisplaySystem(opts *DisplayOptions) error {
	zfg := configSystemPath()
	opts.DbgPrint("load system config: %s", zfg)
	if err := displayTo(opts, zfg); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func DisplayGlobal(opts *DisplayOptions) error {
	zfg := strengthen.ExpandPath("~/.zeta.toml")
	opts.DbgPrint("load global config: %s", zfg)
	if err := displayTo(opts, zfg); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func DisplayLocal(opts *DisplayOptions, zetaDir string) error {
	zfg := filepath.Join(zetaDir, "zeta.toml")
	opts.DbgPrint("load local config: %s", zfg)
	return displayTo(opts, zfg)
}

type GetOptions struct {
	io.Writer
	Keys    []string
	ALL     bool
	Z       bool
	Verbose bool
}

func (opts *GetOptions) show(vals []any) {
	if opts.Z {
		for _, v := range vals {
			_, _ = fmt.Fprintf(opts, "%v%c", v, NUL)
		}
		return
	}
	for _, v := range vals {
		_, _ = fmt.Fprintln(opts, v)
	}
}

func (opts *GetOptions) DbgPrint(format string, args ...any) {
	if !opts.Verbose {
		return
	}
	trace.DbgPrint(format, args...)
}

func getFromFile(opts *GetOptions, zfg string) error {
	md := make(Sections)
	if _, err := toml.DecodeFile(zfg, &md); err != nil {
		return err
	}
	if opts.ALL {
		for _, k := range opts.Keys {
			vals, err := md.filterAll(k)
			if err != nil {
				return err
			}
			opts.show(vals)
		}
		return nil
	}
	for _, k := range opts.Keys {
		val, err := md.filter(k)
		if err != nil {
			return err
		}
		opts.show([]any{val})
	}
	return nil
}

func GetSystem(opts *GetOptions) error {
	zfg := configSystemPath()
	opts.DbgPrint("load system config: %s", zfg)
	return getFromFile(opts, zfg)
}

func GetGlobal(opts *GetOptions) error {
	zfg := strengthen.ExpandPath("~/.zeta.toml")
	opts.DbgPrint("load global config: %s", zfg)
	return getFromFile(opts, zfg)
}

func GetLocal(opts *GetOptions, zetaDir string) error {
	zfg := filepath.Join(zetaDir, "zeta.toml")
	opts.DbgPrint("load local config: %s", zfg)
	return getFromFile(opts, zfg)
}

func Get(opts *GetOptions, zetaDir string, found bool) error {
	opts.DbgPrint("zeta-dir: %s filter keys: %v", zetaDir, opts.Keys)
	if len(zetaDir) != 0 {
		localPath := filepath.Join(zetaDir, "zeta.toml")
		opts.DbgPrint("load local config: %s", localPath)
		err := getFromFile(opts, localPath)
		switch {
		case err == nil:
			if !opts.ALL {
				return nil
			}
			found = true
		case !os.IsNotExist(err) && err != ErrKeyNotFound:
			return err
		}
	}
	userPath := strengthen.ExpandPath("~/.zeta.toml")
	opts.DbgPrint("load global config: %s", userPath)
	err := getFromFile(opts, userPath)
	switch {
	case err == nil:
		if !opts.ALL {
			return nil
		}
		found = true
	case !os.IsNotExist(err) && err != ErrKeyNotFound:
		return err
	}
	systemPath := configSystemPath()
	opts.DbgPrint("load system config: %s", systemPath)
	if err = getFromFile(opts, systemPath); err == nil {
		return nil
	}
	if found && (os.IsNotExist(err) || err == ErrKeyNotFound) {
		// get all key not found in system scope
		return nil
	}
	return err
}
