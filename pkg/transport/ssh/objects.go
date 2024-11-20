// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"bufio"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/pkg/transport"
)

var (
	objectsTransportMagic = [4]byte{'Z', 'B', '\x00', '\x02'}
)

// BatchObjects: zeta-serve objects "group/mono-zeta" --batch
func (c *client) BatchObjects(ctx context.Context, oids []plumbing.Hash) (transport.SessionReader, error) {
	var wg sync.WaitGroup
	wg.Add(1)
	pr, pw := io.Pipe()
	go func() {
		defer wg.Done()
		defer pw.Close()
		buf := bufio.NewWriter(pw)
		defer buf.Flush()
		for _, o := range oids {
			_, _ = buf.WriteString(o.String())
			_ = buf.WriteByte('\n')
		}
		_ = buf.WriteByte('\n')
	}()
	psArgs := []string{"zeta-serve", "objects", fmt.Sprintf("'%s'", c.Path), "--batch"}
	commandArgs := strings.Join(psArgs, " ")
	cmd, err := c.NewBaseCommand(ctx)
	if err != nil {
		return nil, err
	}
	cmd.Stdin = pr
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

// GetObject: zeta-serve objects "group/mono-zeta" --oid "${OID}" --offset=N
func (c *client) GetObject(ctx context.Context, oid plumbing.Hash, fromByte int64) (transport.SizeReader, error) {
	psArgs := []string{"zeta-serve", "objects", fmt.Sprintf("'%s'", c.Path), "--oid=" + oid.String(), fmt.Sprintf("--offset=%d", fromByte)}
	commandArgs := strings.Join(psArgs, " ")
	cmd, err := c.NewBaseCommand(ctx)
	if err != nil {
		return nil, err
	}
	if cmd.Reader, err = cmd.StdoutPipe(); err != nil {
		_ = cmd.Close()
		return nil, err
	}
	if err := cmd.Start(commandArgs); err != nil {
		_ = cmd.Close()
		return nil, err
	}
	var magic [4]byte
	if _, err := io.ReadFull(cmd, magic[:]); err != nil {
		_ = cmd.Close()
		return nil, cmd.lastError
	}
	if !bytes.Equal(magic[:], objectsTransportMagic[:]) {
		_ = cmd.Close()
		return nil, fmt.Errorf("unexpected magic '%c' '%c' '%c' '%c'", magic[0], magic[1], magic[2], magic[3])
	}
	var version uint32
	if err := binary.Read(cmd, binary.BigEndian, &version); err != nil {
		_ = cmd.Close()
		return nil, cmd.lastError
	}
	var compressedSize, readBytes int64
	if err := binary.Read(cmd, binary.BigEndian, &readBytes); err != nil {
		_ = cmd.Close()
		return nil, cmd.lastError
	}
	if err := binary.Read(cmd, binary.BigEndian, &compressedSize); err != nil {
		_ = cmd.Close()
		return nil, cmd.lastError
	}
	return &getObjectCommand{
		Command: cmd,
		offset:  compressedSize - readBytes,
		size:    compressedSize,
	}, nil
}

// Shared: get large objects shared links
func (c *client) Shared(ctx context.Context, wantObjects []*transport.WantObject) ([]*transport.Representation, error) {
	commandArgs := fmt.Sprintf("zeta-serve objects '%s' --share", c.Path)
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(&transport.BatchSharedsRequest{
		Objects: wantObjects,
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
	var r transport.BatchSharedsResponse
	if err := json.NewDecoder(stdout).Decode(&r); err != nil {
		_ = cmd.Close()
		return nil, cmd.lastError
	}
	if err := cmd.Close(); err != nil {
		_ = cmd.Close()
		return nil, cmd.lastError
	}
	return r.Objects, nil
}
