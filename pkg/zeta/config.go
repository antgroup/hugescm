// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
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
	System bool
	Global bool
	Local  bool
	Z      bool
	JSON   bool
	CWD    string
	Values []string
}

func (opts *ListConfigOptions) displayInput() {
	if !opts.Z {
		for _, v := range opts.Values {
			_, _ = fmt.Fprintln(os.Stdout, v)
		}
		return
	}
	NUL := byte(0)
	for _, v := range opts.Values {
		before, after, ok := strings.Cut(v, "=")
		if !ok {
			continue
		}
		_, _ = fmt.Fprintf(os.Stdout, "%s\n%s%c", before, after, NUL)
	}
}

func ListConfig(opts *ListConfigOptions) error {
	if (opts.System && opts.Global) || (opts.System && opts.Local) || (opts.Global && opts.Local) {
		die_error("only one config file at a time")
		return ErrOnlyOneName
	}
	if opts.JSON {
		return listConfigJSON(opts)
	}
	d := &config.DisplayOptions{Writer: os.Stdout, Z: opts.Z}
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

func listConfigJSON(opts *ListConfigOptions) error {
	merged := make(map[string]map[string]any)
	mergeDoc := func(doc config.Document) {
		for section, values := range doc.Raw() {
			if _, ok := merged[section]; !ok {
				merged[section] = make(map[string]any)
			}
			maps.Copy(merged[section], values)
		}
	}
	if opts.System {
		doc, err := config.LoadSystemDocument()
		if err != nil {
			fmt.Fprintf(os.Stderr, "zeta config --list --system --json error: %v\n", err)
			return err
		}
		if doc != nil {
			mergeDoc(doc)
		}
		return json.NewEncoder(os.Stdout).Encode(merged)
	}
	if opts.Global {
		doc, err := config.LoadGlobalDocument()
		if err != nil {
			fmt.Fprintf(os.Stderr, "zeta config --list --global --json error: %v\n", err)
			return err
		}
		if doc != nil {
			mergeDoc(doc)
		}
		return json.NewEncoder(os.Stdout).Encode(merged)
	}
	if opts.Local {
		_, zetaDir, err := FindZetaDir(opts.CWD)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zeta config --list --local --json error: %v\n", err)
			return err
		}
		doc, err := config.LoadLocalDocument(zetaDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "zeta config --list --local --json error: %v\n", err)
			return err
		}
		if doc != nil {
			mergeDoc(doc)
		}
		return json.NewEncoder(os.Stdout).Encode(merged)
	}
	// List all configs (system -> global -> local, local overrides)
	doc, err := config.LoadSystemDocument()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zeta config --list --json error: %v\n", err)
		return err
	}
	if doc != nil {
		mergeDoc(doc)
	}
	doc, err = config.LoadGlobalDocument()
	if err != nil {
		fmt.Fprintf(os.Stderr, "zeta config --list --json error: %v\n", err)
		return err
	}
	if doc != nil {
		mergeDoc(doc)
	}
	_, zetaDir, err := FindZetaDir(opts.CWD)
	switch {
	case err == nil:
		doc, err = config.LoadLocalDocument(zetaDir)
		if err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "zeta config --list --json error: %v\n", err)
			return err
		}
		if doc != nil {
			mergeDoc(doc)
		}
	case IsErrNotZetaDir(err):
		// success
	default:
		fmt.Fprintf(os.Stderr, "zeta config --list --json error: %v\n", err)
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(merged)
}

type GetConfigOptions struct {
	System bool
	Global bool
	Local  bool
	ALL    bool
	Z      bool
	JSON   bool
	Keys   []string
	CWD    string
	Values []string
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
				_, _ = fmt.Fprintf(os.Stdout, "%v%c", a, newLine)
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
	if opts.JSON {
		return getConfigJSON(opts)
	}
	o := &config.GetOptions{
		Writer: os.Stdout,
		Keys:   opts.Keys,
		ALL:    opts.ALL,
		Z:      opts.Z,
	}
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

func getConfigJSON(opts *GetConfigOptions) error {
	// Load documents based on scope
	loadDocs := func() ([]config.Document, error) {
		if opts.System {
			doc, err := config.LoadSystemDocument()
			return []config.Document{doc}, err
		}
		if opts.Global {
			doc, err := config.LoadGlobalDocument()
			return []config.Document{doc}, err
		}
		if opts.Local {
			_, zetaDir, err := FindZetaDir(opts.CWD)
			if err != nil {
				return nil, err
			}
			doc, err := config.LoadLocalDocument(zetaDir)
			return []config.Document{doc}, err
		}
		// All scopes: local, global, system
		var docs []config.Document
		_, zetaDir, err := FindZetaDir(opts.CWD)
		if err == nil {
			doc, err := config.LoadLocalDocument(zetaDir)
			if err != nil && !os.IsNotExist(err) {
				return nil, err
			}
			if doc != nil {
				docs = append(docs, doc)
			}
		}
		doc, err := config.LoadGlobalDocument()
		if err != nil {
			return nil, err
		}
		if doc != nil {
			docs = append(docs, doc)
		}
		doc, err = config.LoadSystemDocument()
		if err != nil {
			return nil, err
		}
		if doc != nil {
			docs = append(docs, doc)
		}
		return docs, nil
	}
	docs, err := loadDocs()
	if err != nil {
		return err
	}
	result := make(map[string]any)
	for _, key := range opts.Keys {
		lowerKey := strings.ToLower(key)
		for _, doc := range docs {
			if doc == nil {
				continue
			}
			if opts.ALL {
				vals, err := doc.GetAll(lowerKey)
				if err != nil {
					continue
				}
				if existing, ok := result[key]; ok {
					if arr, ok := existing.([]any); ok {
						result[key] = append(arr, vals...)
					}
					continue
				}
				result[key] = vals
				continue
			}
			val, err := doc.GetFirst(lowerKey)
			if err != nil {
				continue
			}
			result[key] = val
			break
		}
	}
	return json.NewEncoder(os.Stdout).Encode(result)
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
		if before, after, ok := strings.Cut(kv, "="); ok {
			values[before] = valueCast(after)
			i++
			continue
		}
		if len(opts.NameAndValues) <= i+1 {
			fmt.Fprintf(os.Stderr, "error: config missing args\n")
			return errors.New("missing args")
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
	System bool
	Global bool
	Keys   []string
	CWD    string
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
