// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/klauspost/compress/zstd"
)

// FetchReference: zeta-serve ls-remote "group/mono-zeta" --reference "${REFNAME}"
func (c *client) FetchReference(ctx context.Context, refname plumbing.ReferenceName) (*transport.Reference, error) {
	commandArgs := fmt.Sprintf("zeta-serve ls-remote '%s' --reference=%s", c.Path, refname)
	cmd, err := c.NewBaseCommand(ctx)
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = cmd.Close()
		return nil, err
	}
	if err := cmd.Start(commandArgs); err != nil {
		_ = cmd.Close()
		return nil, err
	}
	var r transport.Reference
	if err := json.NewDecoder(stdout).Decode(&r); err != nil {
		_ = cmd.Close()
		if cmd.lastError != nil && cmd.lastError.Code == 404 {
			return nil, transport.ErrReferenceNotExist
		}
		return nil, cmd.lastError
	}
	if err := cmd.Close(); err != nil {
		return nil, err
	}
	return &r, nil
}

func sparsesGenReader(sparses []string) io.Reader {
	var b strings.Builder
	var total int
	for _, s := range sparses {
		total += len(s) + 1
	}
	b.Grow(total)
	for _, s := range sparses {
		_, _ = b.WriteString(s)
		_ = b.WriteByte('\n')
	}
	return strings.NewReader(b.String())
}

type decompressReader struct {
	decoder *zstd.Decoder
	cmd     *Command
}

func (r decompressReader) Read(p []byte) (n int, err error) {
	return r.decoder.Read(p)
}

func (r decompressReader) Close() error {
	r.decoder.Close()
	return r.cmd.Close()
}

func (r *decompressReader) LastError() error {
	return r.cmd.lastError
}

// FetchMetadata: support base metadata and sparses metadata.
//
//	zeta-serve metadata "group/mono-zeta" --revision "${REVISION}" --depth=1 --deepen-from=${from}
//	zeta-serve metadata "group/mono-zeta" --revision "${REVISION}" --sparse --depth=1 --deepen-from=${from}
func (c *client) FetchMetadata(ctx context.Context, target plumbing.Hash, opts *transport.MetadataOptions) (transport.SessionReader, error) {
	psArgs := []string{"zeta-serve", "metadata", fmt.Sprintf("'%s'", c.Path), "--revision", target.String()}
	if !opts.Have.IsZero() {
		psArgs = append(psArgs, "--have="+opts.Have.String())
	}
	if !opts.DeepenFrom.IsZero() {
		psArgs = append(psArgs, "--deepen-from="+opts.DeepenFrom.String())
	}
	psArgs = append(psArgs, "--deepen="+strconv.Itoa(opts.Deepen))
	if opts.Depth >= 0 {
		psArgs = append(psArgs, "--depth="+strconv.Itoa(opts.Depth))
	}
	if len(opts.SparseDirs) != 0 {
		psArgs = append(psArgs, "--sparse")
	}
	psArgs = append(psArgs, "--zstd")
	commandArgs := strings.Join(psArgs, " ")
	cmd, err := c.NewBaseCommand(ctx)
	if err != nil {
		return nil, err
	}
	cmd.Stdin = sparsesGenReader(opts.SparseDirs)
	if cmd.Reader, err = cmd.StdoutPipe(); err != nil {
		_ = cmd.Close()
		return nil, err
	}
	if err := cmd.Start(commandArgs); err != nil {
		_ = cmd.Close()
		return nil, err
	}
	zr, err := zstd.NewReader(cmd)
	if err != nil {
		_ = cmd.Close()
		return nil, err
	}
	return &decompressReader{decoder: zr, cmd: cmd}, nil
}
