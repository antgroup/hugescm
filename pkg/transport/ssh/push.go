// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/pkg/transport"
)

// Push: zeta-serve push "group/mono-zeta" --reference "$REFNAME"
func (c *client) Push(ctx context.Context, r io.Reader, command *transport.Command) (rc transport.SessionReader, err error) {
	commandArgs := fmt.Sprintf("zeta-serve push '%s' --reference=%s --old-rev=%s --new-rev=%s", c.Path, command.Refname, command.OldRev, command.NewRev)
	cmd, err := c.NewBaseCommand(ctx)
	if err != nil {
		return nil, err
	}
	_ = cmd.Setenv("ZETA_OBJECTS_STATS", fmt.Sprintf("m-%d;b-%d", command.Metadata, command.Objects))
	if len(command.PushOptions) != 0 {
		_ = cmd.Setenv("ZETA_PUSH_OPTION_COUNT", strconv.Itoa(len(command.PushOptions)))
		for i, o := range command.PushOptions {
			_ = cmd.Setenv(fmt.Sprintf("ZETA_PUSH_OPTION_%d", i), o)
		}
	}
	cmd.Stdin = r
	if cmd.Reader, err = cmd.StdoutPipe(); err != nil {
		_ = cmd.Close()
		return nil, err
	}
	if err := cmd.Start(commandArgs); err != nil {
		_ = cmd.Close()
		return nil, err
	}
	return cmd, nil
}

// BatchCheck: zeta-serve push "group/mono-zeta" --reference "$REFNAME" --batch-check
func (c *client) BatchCheck(ctx context.Context, refname plumbing.ReferenceName, haveObjects []*transport.HaveObject) ([]*transport.HaveObject, error) {
	commandArgs := fmt.Sprintf("zeta-serve push '%s' --reference=%s --batch-check", c.Path, refname)
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(&transport.BatchRequest{
		Objects: haveObjects,
	}); err != nil {
		return nil, err
	}
	cmd, err := c.NewBaseCommand(ctx)
	if err != nil {
		return nil, err
	}
	cmd.Stdin = &b
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = cmd.Close()
		return nil, err
	}
	if err := cmd.Start(commandArgs); err != nil {
		_ = cmd.Close()
		return nil, err
	}
	var response transport.BatchResponse
	if err := json.NewDecoder(stdout).Decode(&response); err != nil {
		_ = cmd.Close()
		return nil, cmd.lastError
	}
	if err := cmd.Close(); err != nil {
		_ = cmd.Close()
		return nil, cmd.lastError
	}
	return response.Objects, nil
}

// PutObject: zeta-serve push "group/mono-zeta" --reference "$REFNAME" --oid "$OID" --size "${SIZE}"
func (c *client) PutObject(ctx context.Context, refname plumbing.ReferenceName, oid plumbing.Hash, r io.Reader, size int64) error {
	commandArgs := fmt.Sprintf("zeta-serve push '%s' --reference=%s --oid=%s --size=%d", c.Path, refname, oid, size)
	cmd, err := c.NewBaseCommand(ctx)
	if err != nil {
		return err
	}
	cmd.Stdin = r
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = cmd.Close()
		return err
	}
	if err := cmd.Start(commandArgs); err != nil {
		_ = cmd.Close()
		return err
	}
	_, _ = io.Copy(io.Discard, stdout)
	if err := cmd.Close(); err != nil {
		return err
	}
	return cmd.lastError
}
