//go:build darwin

// Copyright 2013 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package keyring

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var pattern *regexp.Regexp

func init() {
	pattern = regexp.MustCompile(`[^\w@%+=:,./-]`)
}

// Quote returns a shell-escaped version of the string s. The returned value
// is a string that can safely be used as one token in a shell command line.
func Quote(s string) string {
	if len(s) == 0 {
		return "''"
	}

	if pattern.MatchString(s) {
		return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
	}

	return s
}

const (
	execPathKeychain = "/usr/bin/security"
)

type macOSXKeychain struct{}

// func (*MacOSXKeychain) IsAvailable() bool {
// 	return exec.Command(execPathKeychain).Run() != exec.ErrNotFound
// }

func init() {
	provider = macOSXKeychain{}
}

// security find-internet-password -s 'appstoreconnect.apple.com' -g 2>&1 | grep -E 'acct|password' |  sed 's/^ *//' | sort | rev  | cut -d'"' -f2 | rev

// keychain: "/Users/**/Library/Keychains/login.keychain-db"
// version: 512
// class: "inet"
// attributes:
//     0x00000007 <blob>="**"
//     0x00000008 <blob>=<NULL>
//     "acct"<blob>="ZetaPseudo"
//     "atyp"<blob>="dflt"
//     "cdat"<timedate>=0x32303234303731303130353331345A00  "20240710105314Z\000"
//     "crtr"<uint32>=<NULL>
//     "cusi"<sint32>=<NULL>
//     "desc"<blob>=<NULL>
//     "icmt"<blob>=<NULL>
//     "invi"<sint32>=<NULL>
//     "mdat"<timedate>=0x32303234303731303130353331345A00  "20240710105314Z\000"
//     "nega"<sint32>=<NULL>
//     "path"<blob>=<NULL>
//     "port"<uint32>=0x00000000
//     "prot"<blob>=<NULL>
//     "ptcl"<uint32>=0x00000000
//     "scrp"<sint32>=<NULL>
//     "sdmn"<blob>=<NULL>
//     "srvr"<blob>="https://zeta.example.io"
//     "type"<uint32>=<NULL>
// password: "**"

func parseKeychainOut(r io.Reader) (*Cred, error) {
	br := bufio.NewScanner(r)
	cred := &Cred{}
	var err error
	for br.Scan() {
		line := strings.TrimFunc(br.Text(), unicode.IsSpace)
		if suffix, ok := strings.CutPrefix(line, `"acct"`); ok {
			_, acct, _ := strings.Cut(suffix, "=")
			if cred.UserName, err = strconv.Unquote(strings.TrimFunc(acct, unicode.IsSpace)); err != nil {
				return nil, err
			}
			continue
		}
		// password: "**"
		if password, ok := strings.CutPrefix(line, "password:"); ok {
			// 'password: "**"' --> ' "**"' --> '"**"' --> '**'
			if cred.Password, err = strconv.Unquote(strings.TrimFunc(password, unicode.IsSpace)); err != nil {
				return nil, err
			}
			continue
		}
	}

	return cred, nil
}

func (k macOSXKeychain) Find(ctx context.Context, targetName string) (*Cred, error) {
	cmd := exec.CommandContext(ctx, execPathKeychain, "find-internet-password", "-s", targetName, "-g")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if cmd.ProcessState.ExitCode() == 44 {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return parseKeychainOut(bytes.NewReader(out))
}

func (k macOSXKeychain) Store(ctx context.Context, targetName string, c *Cred) error {
	// if the added secret has multiple lines or some non ascii,
	// osx will hex encode it on return. To avoid getting garbage, we
	// encode all passwords

	cmd := exec.CommandContext(ctx, execPathKeychain, "-i")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	if err = cmd.Start(); err != nil {
		return err
	}

	command := fmt.Sprintf("add-internet-password -U -s %s -a %s -w %s\n", Quote(targetName), Quote(c.UserName), Quote(c.Password))
	if len(command) > 4096 {
		return ErrSetDataTooBig
	}

	if _, err := io.WriteString(stdin, command); err != nil {
		return err
	}

	if err = stdin.Close(); err != nil {
		return err
	}

	err = cmd.Wait()
	return err
}

func (k macOSXKeychain) Discard(ctx context.Context, targetName string) error {
	out, err := exec.CommandContext(ctx,
		execPathKeychain,
		"delete-generic-password",
		"-s", targetName).CombinedOutput()
	if strings.Contains(string(out), "could not be found") {
		err = ErrNotFound
	}
	return err
}
