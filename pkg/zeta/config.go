// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/zeta/config"
)

var (
	ErrMissingKeys = errors.New("missing keys")
	ErrOnlyOneName = errors.New("only one config file at a time")
)

type ListConfigOptions struct {
	System  bool
	Global  bool
	Local   bool
	Z       bool
	CWD     string
	Values  []string
	Verbose bool
}

func (opts *ListConfigOptions) displayInput() {
	if !opts.Z {
		for _, v := range opts.Values {
			fmt.Fprintln(os.Stdout, v)
		}
		return
	}
	NUL := byte(0)
	for _, v := range opts.Values {
		i := strings.IndexByte(v, '=')
		if i == -1 {
			continue
		}
		fmt.Fprintf(os.Stdout, "%s\n%s%c", v[0:i], v[i+1:], NUL)
	}
}

func ListConfig(opts *ListConfigOptions) error {
	if (opts.System && opts.Global) || (opts.System && opts.Local) || (opts.Global && opts.Local) {
		die_error("only one config file at a time")
		return ErrOnlyOneName
	}
	d := &config.DisplayOptions{Writer: os.Stdout, Z: opts.Z, Verbose: opts.Verbose}
	if opts.System {
		if err := config.DisplaySystem(d); err != nil {
			fmt.Fprintf(os.Stderr, "zeta config --list --system error: %v\n", err)
			return err
		}
		return nil
	}
	if opts.Global {
		if err := config.DisplayGlobal(d); err != nil {
			fmt.Fprintf(os.Stderr, "zeta config --list --global error: %v\n", err)
			return err
		}
		return nil
	}
	if opts.Local {
		_, zetaDir, err := FindZetaDir(opts.CWD)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zeta config --list --local error: %v\n", err)
			return err
		}
		if err := config.DisplayLocal(d, zetaDir); err != nil {
			fmt.Fprintf(os.Stderr, "zeta config --list --local error: %v\n", err)
			return err
		}
		return nil
	}
	// List all config
	var err error
	if err = config.DisplaySystem(d); err != nil {
		fmt.Fprintf(os.Stderr, "zeta config --list error: %v\n", err)
		return err
	}
	if err = config.DisplayGlobal(d); err != nil {
		fmt.Fprintf(os.Stderr, "zeta config --list error: %v\n", err)
		return err
	}
	_, zetaDir, err := FindZetaDir(opts.CWD)
	switch {
	case err == nil:
		if err := config.DisplayLocal(d, zetaDir); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "zeta config --list error: %v\n", err)
			return err
		}
	case IsErrNotZetaDir(err):
		// success
	default:
		fmt.Fprintf(os.Stderr, "zeta config --list error: %v\n", err)
		return err
	}
	opts.displayInput()
	return nil
}

type GetConfigOptions struct {
	System  bool
	Global  bool
	Local   bool
	ALL     bool
	Z       bool
	Keys    []string
	CWD     string
	Values  []string
	Verbose bool
}

func (opts *GetConfigOptions) subCommand() string {
	if opts.ALL {
		return "--get-all"
	}
	return "--get"
}

func (opts *GetConfigOptions) getFromInputs() bool {
	newLine := '\n'
	if opts.Z {
		newLine = '\x00'
	}
	m := valuesMapArray(opts.Values)
	for _, k := range opts.Keys {
		if av, ok := m[strings.ToLower(k)]; ok {
			for _, a := range av {
				fmt.Fprintf(os.Stdout, "%v%c", a, newLine)
				if !opts.ALL {
					return true
				}
			}
			return true
		}
	}
	return false
}

func GetConfig(opts *GetConfigOptions) error {
	if (opts.System && opts.Global) || (opts.System && opts.Local) || (opts.Global && opts.Local) {
		fmt.Fprintf(os.Stderr, "error: only one config file at a time\n")
		return ErrOnlyOneName
	}
	if len(opts.Keys) == 0 {
		fmt.Fprintf(os.Stderr, "zeta config %s: missing keys\n", opts.subCommand())
		return ErrMissingKeys
	}
	o := &config.GetOptions{
		Writer:  os.Stdout,
		Keys:    opts.Keys,
		ALL:     opts.ALL,
		Z:       opts.Z,
		Verbose: opts.Verbose}
	if opts.System {
		if err := config.GetSystem(o); err != nil {
			fmt.Fprintf(os.Stderr, "zeta config %s --system error: %v\n", opts.subCommand(), err)
			return err
		}
		return nil
	}
	if opts.Global {
		if err := config.GetGlobal(o); err != nil {
			fmt.Fprintf(os.Stderr, "zeta config %s --global error: %v\n", opts.subCommand(), err)
			return err
		}
		return nil
	}
	if opts.Local {
		_, zetaDir, err := FindZetaDir(opts.CWD)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zeta config %s local error: %v\n", opts.subCommand(), err)
			return err
		}
		if err := config.GetLocal(o, zetaDir); err != nil {
			fmt.Fprintf(os.Stderr, "zeta config %s --local error: %v\n", opts.subCommand(), err)
			return err
		}
		return nil
	}
	found := opts.getFromInputs()
	if found && !opts.ALL {
		return nil
	}
	_, zetaDir, err := FindZetaDir(opts.CWD)
	if err != nil && !IsErrNotZetaDir(err) {
		fmt.Fprintf(os.Stderr, "zeta config %s error: %v\n", opts.subCommand(), err)
		return err
	}
	if err := config.Get(o, zetaDir, found); err != nil {
		fmt.Fprintf(os.Stderr, "zeta config %s error: %v\n", opts.subCommand(), err)
		return err
	}
	return nil
}

// ParseBool returns the boolean value represented by the string.
// It accepts 1, t, T, TRUE, true, True, 0, f, F, FALSE, false, False.
// Any other value returns an error.
func ParseBool(str string) (bool, error) {
	switch strings.ToLower(str) {
	case "1", "t", "true", "on", "yes":
		return true, nil
	case "0", "f", "false", "off", "no":
		return false, nil
	}
	return false, strconv.ErrSyntax
}

type UpdateConfigOptions struct {
	System        bool
	Global        bool
	Add           bool
	NameAndValues []string
	Type          string
	CWD           string
	Verbose       bool
}

func UpdateConfig(opts *UpdateConfigOptions) error {
	if opts.System && opts.Global {
		fmt.Fprintf(os.Stderr, "error: only one config file at a time\n")
		return ErrOnlyOneName
	}
	valueType := strings.ToLower(opts.Type)
	valueCast := func(s string) any {
		switch valueType {
		case "int":
			if i, err := strconv.ParseInt(s, 10, 64); err == nil {
				return i
			}
		case "float":
			if f, err := strconv.ParseFloat(s, 64); err == nil {
				return f
			}
		case "bool":
			if b, err := ParseBool(s); err == nil {
				return b
			}
		case "path":
		default:
		}
		return s
	}

	values := make(map[string]any)
	nlen := len(opts.NameAndValues)
	for i := 0; i < nlen; {
		kv := opts.NameAndValues[i]
		if index := strings.IndexByte(kv, '='); index != -1 {
			values[kv[0:index]] = valueCast(kv[index+1:])
			i++
			continue
		}
		values[kv] = valueCast(opts.NameAndValues[i+1])
		i += 2
	}
	if opts.System {
		return config.UpdateSystem(&config.UpdateOptions{
			Values: values,
			Append: opts.Add,
		})
	}

	if opts.Global {
		return config.UpdateGlobal(&config.UpdateOptions{
			Values: values,
			Append: opts.Add,
		})
	}
	_, zetaDir, err := FindZetaDir(opts.CWD)
	if err != nil {
		fmt.Fprintf(os.Stderr, "set config error: %s\n", err)
		return err
	}
	return config.UpdateLocal(zetaDir, &config.UpdateOptions{
		Values: values,
		Append: opts.Add,
	})
}

type UnsetConfigOptions struct {
	System  bool
	Global  bool
	Keys    []string
	CWD     string
	Verbose bool
}

func UnsetConfig(opts *UnsetConfigOptions) error {
	if opts.System && opts.Global {
		fmt.Fprintf(os.Stderr, "error: only one config file at a time\n")
		return ErrOnlyOneName
	}
	if len(opts.Keys) == 0 {
		fmt.Fprintf(os.Stderr, "zeta config --unset: missing keys\n")
		return ErrMissingKeys
	}
	if opts.System {
		if err := config.UnsetSystem(opts.Keys...); err != nil {
			fmt.Fprintf(os.Stderr, "zeta config --unset --system error: %v\n", err)
			return err
		}
		return nil
	}
	if opts.Global {
		if err := config.UnsetGlobal(opts.Keys...); err != nil {
			fmt.Fprintf(os.Stderr, "zeta config --unset --global error: %v\n", err)
			return err
		}
		return nil
	}
	_, zetaDir, err := FindZetaDir(opts.CWD)
	if err != nil {
		fmt.Fprintf(os.Stderr, "unset keys error: %s\n", err)
		return err
	}
	if err := config.UnsetLocal(zetaDir, opts.Keys...); err != nil {
		fmt.Fprintf(os.Stderr, "zeta config --unset error: %v\n", err)
		return err
	}
	return nil
}
