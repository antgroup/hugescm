// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sshserver

import (
	"errors"
	"fmt"
	"strings"
)

// option argument type
const (
	REQUIRED int = iota
	NOARG
	OPTIONAL
)

// error
var (
	ErrNilArgs       = errors.New("argv is nil")
	ErrUnExpectedArg = errors.New("unexpected argument '-'")
)

type ParseOptionHandler func(index rune, nextArg string, raw string) error

type option struct {
	longName  string
	rule      int
	shortName rune
}

// ParseArgs todo
type ParseArgs struct {
	opts           []*option
	unresolvedArgs []string
	index          int
}

func (p *ParseArgs) searchLongOption(longName string) *option {
	for _, o := range p.opts {
		if o.longName == longName {
			return o
		}
	}
	return nil
}

func (p *ParseArgs) searchShortOption(shortName rune) *option {
	for _, o := range p.opts {
		if o.shortName == shortName {
			return o
		}
	}
	return nil
}

// Add option
func (p *ParseArgs) Add(longName string, rule int, shortName rune) *ParseArgs {
	p.opts = append(p.opts, &option{longName: longName, rule: rule, shortName: shortName})
	return p
}

// Unresolved todo
func (p *ParseArgs) Unresolved(index int) (string, bool) {
	if len(p.unresolvedArgs) <= index {
		return "", false
	}
	return p.unresolvedArgs[index], true
}

func (p *ParseArgs) parseLong(rawArg string, args []string, fn ParseOptionHandler) error {
	longName, nextArg, ok := strings.Cut(rawArg, "=")
	o := p.searchLongOption(longName)
	if o == nil {
		return fmt.Errorf("unregistered option '--%s'", rawArg)
	}
	if o.rule == NOARG && ok {
		return fmt.Errorf("option '--%s' unexpected parameter: %s", rawArg, nextArg)
	}
	if o.rule == REQUIRED && !ok {
		if p.index+1 > len(args) {
			return fmt.Errorf("option '--%s' missing parameter", rawArg)
		}
		nextArg = args[p.index+1]
		p.index++
	}
	if err := fn(o.shortName, nextArg, rawArg); err != nil {
		return err
	}
	return nil
}

func (p *ParseArgs) parseShort(rawArg string, args []string, fn ParseOptionHandler) error {
	name, nextArg, ok := strings.Cut(rawArg, "=")
	if len(name) != 1 {
		return fmt.Errorf("unexpected argument '-%s'", rawArg)
	}
	shortName := rune(name[0])
	o := p.searchShortOption(shortName)
	if o == nil {
		return fmt.Errorf("unregistered option '-%c'", shortName)
	}
	if o.rule == NOARG && ok {
		return fmt.Errorf("option '-%c' unexpected parameter: %s", shortName, nextArg)
	}
	if o.rule == REQUIRED && !ok {
		if p.index+1 > len(args) {
			return fmt.Errorf("option '-%c' missing parameter", shortName)
		}
		nextArg = args[p.index+1]
		p.index++
	}
	if err := fn(shortName, nextArg, rawArg); err != nil {
		return err
	}
	return nil
}

func (p *ParseArgs) Parse(args []string, fn ParseOptionHandler) error {
	if len(args) == 0 {
		return ErrNilArgs
	}
	for ; p.index < len(args); p.index++ {
		arg := args[p.index]
		if arg == "--" {
			p.unresolvedArgs = append(p.unresolvedArgs, args[p.index:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") {
			p.unresolvedArgs = append(p.unresolvedArgs, arg)
			continue
		}
		if strings.HasPrefix(arg, "--") {
			if err := p.parseLong(arg[2:], args, fn); err != nil {
				return err
			}
			continue
		}
		if err := p.parseShort(arg[1:], args, fn); err != nil {
			return err
		}
	}
	return nil
}
