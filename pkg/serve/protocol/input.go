// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package protocol

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
)

func ReadInputPaths(r io.Reader) ([]string, error) {
	br := bufio.NewScanner(r)
	paths := make([]string, 0, 100)
	for br.Scan() {
		p := strings.TrimSpace(br.Text())
		if len(p) == 0 {
			break
		}
		paths = append(paths, p)
	}
	if br.Err() != nil {
		return nil, br.Err()
	}
	return paths, nil
}

func ReadInputOIDs(r io.Reader) ([]plumbing.Hash, error) {
	br := bufio.NewScanner(r)
	seen := make(map[string]bool)
	oids := make([]plumbing.Hash, 0, 100)
	for br.Scan() {
		sid := strings.TrimSpace(br.Text())
		if len(sid) == 0 {
			break
		}
		if !plumbing.ValidateHashHex(sid) {
			return nil, fmt.Errorf("invalid hash '%s'", sid)
		}
		if seen[sid] {
			continue
		}
		seen[sid] = true
		if sid == plumbing.BLANK_BLOB {
			continue
		}
		oids = append(oids, plumbing.NewHash(sid))
	}
	if br.Err() != nil {
		return nil, br.Err()
	}
	return oids, nil
}
