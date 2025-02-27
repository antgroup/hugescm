// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sshserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/antgroup/hugescm/modules/crc"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/pkg/serve/odb"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/sirupsen/logrus"
)

// zeta-serve objects "group/mono-zeta" --oid "${OID}" --offset=N

// zeta-serve objects "group/mono-zeta" --batch

// zeta-serve objects "group/mono-zeta" --share

type Objects struct {
	Path   string
	OID    plumbing.Hash
	Offset int64
	Batch  bool
	Share  bool
}

func (c *Objects) ParseArgs(args []string) error {
	var p ParseArgs
	p.Add("oid", REQUIRED, 'O').
		Add("offset", REQUIRED, 'o').
		Add("share", NOARG, 'S').
		Add("batch", NOARG, 'B')
	if err := p.Parse(args, func(index rune, nextArg, raw string) error {
		switch index {
		case 'O':
			if !plumbing.ValidateHashHex(nextArg) {
				return fmt.Errorf("oid is invalid hash: %s", nextArg)
			}
			c.OID = plumbing.NewHash(nextArg)
		case 'o':
			offset, err := strconv.ParseInt(nextArg, 10, 64)
			if err != nil {
				return fmt.Errorf("parse '--offset': %s error: %s", nextArg, err)
			}
			if offset < 0 {
				return errors.New("--offset cannot be less than 0")
			}
			c.Offset = offset
		case 'B':
			c.Batch = true
		case 'S':
			c.Share = true
		case 'L':

		}
		return nil
	}); err != nil {
		return err
	}
	var ok bool
	if c.Path, ok = p.Unresolved(0); !ok {
		return ErrPathNecessary
	}
	return nil
}

func (c *Objects) Exec(ctx *RunCtx) int {
	if exitCode := ctx.S.doPermissionCheck(ctx.Session, c.Path, protocol.DOWNLOAD); exitCode != 0 {
		return exitCode
	}
	if c.Batch {
		return ctx.S.BatchObjects(ctx.Session)
	}
	if c.Share {
		return ctx.S.ShareObjects(ctx.Session)
	}
	if c.OID.IsZero() {
		ctx.Session.WriteError("bad oid")
		return 400
	}
	return ctx.S.GetObject(ctx.Session, c.OID, c.Offset)
}

func (s *Server) BatchObjects(e *Session) int {
	oids, err := protocol.ReadInputOIDs(e)
	if err != nil {
		return e.ExitFormat(400, "batch-objects: %v", err)
	}
	rr, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer rr.Close()
	buffedWriter := streamio.GetBufferWriter(e)
	defer func() {
		_ = buffedWriter.Flush()
		streamio.PutBufferWriter(buffedWriter)
	}()
	cw := crc.NewCrc64Writer(buffedWriter)
	if err := protocol.WriteBatchObjectsHeader(cw); err != nil {
		logrus.Errorf("write blob header error: %v", err)
		return e.ExitError(err)
	}
	o := rr.ODB()
	writeFunc := func(oid plumbing.Hash) error {
		sr, err := o.Open(e.Context(), oid, 0)
		if plumbing.IsNoSuchObject(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if sr.Size() > protocol.MAX_BATCH_BLOB_SIZE {
			sr.Close()
			return nil
		}
		defer sr.Close()
		return protocol.WriteObjectsItem(cw, sr, oid.String(), sr.Size())
	}
	for _, oid := range oids {
		if err := writeFunc(oid); err != nil {
			logrus.Errorf("batch-objects write blob %s error: %v", oid, err)
			return e.ExitError(err)
		}
	}
	_ = protocol.WriteObjectsItem(cw, nil, "", 0) // FLUSH
	if _, err := cw.Finish(); err != nil {
		logrus.Errorf("batch-objects finish crc64 error: %v", err)
	}
	return 0
}

func (s *Server) GetObject(e *Session, oid plumbing.Hash, offset int64) int {
	repo, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer repo.Close()
	o := repo.ODB()
	sr, err := o.Open(e.Context(), oid, 0)
	if err != nil {
		return e.ExitError(err)
	}
	defer sr.Close()
	logrus.Infof("write %s content-length: %d size: %d", oid, sr.Size()-offset, sr.Size())
	if err := protocol.WriteSingleObjectsHeader(e, sr.Size()-offset, sr.Size()); err != nil {
		return e.ExitError(err)
	}
	if _, err := odb.Copy(e, sr); err != nil {
		return e.ExitError(err)
	}
	return 0
}

func (s *Server) ShareObjects(e *Session) int {
	var request protocol.BatchShareObjectsRequest
	if err := json.NewDecoder(e).Decode(&request); err != nil {
		return e.ExitFormat(400, "decode request body error: %v", err)
	}
	rr, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer rr.Close()

	response := &protocol.BatchShareObjectsResponse{
		Objects: make([]*protocol.Representation, 0, len(request.Objects)),
	}
	odb := rr.ODB()
	ExpiresAt := time.Now().Add(time.Hour * 2)
	expiresAt := ExpiresAt.Unix()
	for _, o := range request.Objects {
		if o == nil {
			return e.ExitFormat(400, "require object is nil")
		}
		want := plumbing.NewHash(o.OID)
		// oss shared download link
		ro, err := odb.Share(e.Context(), want, expiresAt)
		if err != nil {
			return e.ExitError(err)
		}
		response.Objects = append(response.Objects, &protocol.Representation{
			OID:            want.String(),
			CompressedSize: ro.Size,
			Href:           ro.Href,
			ExpiresAt:      ExpiresAt,
		})
	}
	ZetaEncodeVND(e, response)
	return 0
}
