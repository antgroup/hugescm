// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/pkg/transport"
)

const (
	ENV_ZETA_EXTENSION_ARIA2C = "ZETA_EXTENSION_ARIA2C"
)

func LookupAria2c() (string, error) {
	if aria2c, ok := os.LookupEnv(ENV_ZETA_EXTENSION_ARIA2C); ok {
		if d, err := exec.LookPath(aria2c); err == nil {
			return d, nil
		}
	}
	aria2c, err := exec.LookPath("aria2c")
	if err != nil {
		return "", err
	}
	return aria2c, nil
}

// TODO: gen input file
func (r *Repository) aria2Input(objects []*transport.Representation) (io.Reader, map[plumbing.Hash]string) {
	var b strings.Builder
	m := make(map[plumbing.Hash]string)
	for _, o := range objects {
		oid := plumbing.NewHash(o.OID)
		p := r.odb.JoinPart(oid)
		fmt.Fprintf(&b, "%s\n dir=%s\n out=%s\n", o.Href, filepath.Dir(p), filepath.Base(p))
		for h, v := range o.Header {
			fmt.Fprintf(&b, " header=%s: %s\n", h, v)
		}
		m[oid] = p
	}

	return strings.NewReader(b.String()), m
}

func (r *Repository) aria2cGet(ctx context.Context, aria2c string, stdin io.Reader, stdout, stderr io.Writer, concurrent int) error {
	if concurrent <= 0 {
		concurrent = 1
	}
	if concurrent > 50 {
		concurrent = 50
	}
	cmd := exec.CommandContext(ctx, aria2c, "-i", "-", "-j", strconv.Itoa(concurrent))
	cmd.Stdin = stdin
	cmd.Stderr = stdout
	cmd.Stdout = stderr
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func (r *Repository) aria2Get(ctx context.Context, objects []*transport.Representation) error {
	if len(objects) == 0 {
		return nil
	}
	concurrent := r.ConcurrentTransfers()
	trace.DbgPrint("concurrent transfers %d", concurrent)
	aria2c, err := LookupAria2c()
	if err != nil {
		fmt.Fprintf(os.Stderr, "lookup aria2c %s\n", err)
		return err
	}
	input, m := r.aria2Input(objects)
	if err := r.aria2cGet(ctx, aria2c, input, os.Stdout, os.Stderr, concurrent); err != nil {
		return err
	}
	for oid, saveTo := range m {
		if err := r.odb.ValidatePart(saveTo, oid); err != nil {
			fmt.Fprintf(os.Stderr, "validate %s error: %v\n", saveTo, err)
			return err
		}
	}
	return nil
}
