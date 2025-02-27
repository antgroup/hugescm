// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sshserver

import (
	"fmt"
	"strconv"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/sirupsen/logrus"
)

// zeta-serve metadata "group/mono-zeta" --revision "${REVISION}" --depth=1 --deepen-from=${from}

// zeta-serve metadata "group/mono-zeta" --revision "${REVISION}" --sparse --depth=1 --deepen-from=${from}

// zeta-serve metadata "group/mono-zeta" --batch --depth=1

const (
	UseZSTD rune = 1000
)

type Metadata struct {
	Path       string
	Revision   string
	Have       plumbing.Hash
	DeepenFrom plumbing.Hash
	Deepen     int
	Depth      int
	Batch      bool
	Sparse     bool
	UseZSTD    bool
}

func (c *Metadata) ParseArgs(args []string) error {
	c.Deepen = 1
	c.Depth = -1
	var p ParseArgs
	p.Add("revision", REQUIRED, 'R').
		Add("depth", REQUIRED, 'N').
		Add("have", REQUIRED, 'H').
		Add("deepen-from", REQUIRED, 'F').
		Add("deepen", REQUIRED, 'D').
		Add("sparse", NOARG, 'S').
		Add("batch", NOARG, 'B').
		Add("zstd", NOARG, UseZSTD)
	if err := p.Parse(args, func(index rune, nextArg, raw string) error {
		switch index {
		case 'R':
			c.Revision = nextArg
		case 'N':
			i, err := strconv.Atoi(nextArg)
			if err != nil {
				return fmt.Errorf("parse depth '%s' error: %v", nextArg, err)
			}
			c.Depth = i
		case 'H':
			if !plumbing.ValidateHashHex(nextArg) {
				return fmt.Errorf("have is invalid hash: %s", nextArg)
			}
			c.Have = plumbing.NewHash(nextArg)
		case 'F':
			if !plumbing.ValidateHashHex(nextArg) {
				return fmt.Errorf("deepen-from is invalid hash: %s", nextArg)
			}
			c.DeepenFrom = plumbing.NewHash(nextArg)
			c.Deepen = -1
		case 'D':
			i, err := strconv.Atoi(nextArg)
			if err != nil {
				return fmt.Errorf("parse depth '%s' error: %v", nextArg, err)
			}
			c.Deepen = i
		case 'S':
			c.Sparse = true
		case 'B':
			c.Batch = true
		case UseZSTD:
			c.UseZSTD = true
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

func (c *Metadata) Exec(ctx *RunCtx) int {
	if exitCode := ctx.S.doPermissionCheck(ctx.Session, c.Path, protocol.DOWNLOAD); exitCode != 0 {
		return exitCode
	}
	if c.Batch {
		return ctx.S.BatchMetadata(ctx.Session, c.Depth, c.UseZSTD)
	}
	if c.Sparse {
		return ctx.S.GetSparseMetadata(ctx.Session, c)
	}
	return ctx.S.FetchMetadata(ctx.Session, c)
}

func (s *Server) FetchMetadata(e *Session, c *Metadata) int {
	rr, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer rr.Close()
	ro, err := rr.ParseRev(e.Context(), c.Revision)
	if err != nil {
		return e.ExitError(err)
	}
	if ro.Target == nil {
		return e.ExitFormat(400, "revision %s target not commit", c.Revision)
	}
	p, err := protocol.NewPipePacker(rr.ODB(), e, c.Depth, c.UseZSTD)
	if err != nil {
		logrus.Errorf("new packer error %v", err)
		return e.ExitError(err)
	}
	defer p.Close()
	for oid, o := range ro.Objects {
		if err := p.WriteAny(e.Context(), o, oid); err != nil {
			logrus.Errorf("write objects error %v", err)
			return e.ExitError(err)
		}
	}
	if err := p.WriteDeepenMetadata(e.Context(), ro.Target, c.DeepenFrom, c.Have, c.Deepen); err != nil {
		logrus.Errorf("write commits error %v", err)
		return e.ExitError(err)
	}
	if err := p.Done(); err != nil {
		logrus.Errorf("finish metadata error %v", err)
		return e.ExitError(err)
	}
	return 0
}

func (s *Server) GetSparseMetadata(e *Session, c *Metadata) int {
	paths, err := protocol.ReadInputPaths(e)
	if err != nil {
		return e.ExitFormat(400, "bad input paths: %v", err)
	}

	rr, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer rr.Close()

	ro, err := rr.ParseRev(e.Context(), c.Revision)
	if err != nil {
		return e.ExitError(err)
	}
	if ro.Target == nil {
		return e.ExitFormat(400, "revision %s target not commit", c.Revision)
	}
	cc := ro.Target
	p, err := protocol.NewPipePacker(rr.ODB(), e, c.Depth, c.UseZSTD)
	if err != nil {
		logrus.Errorf("new packer error %v", err)
		return e.ExitError(err)
	}
	defer p.Close()
	for oid, o := range ro.Objects {
		if err := p.WriteAny(e.Context(), o, oid); err != nil {
			logrus.Errorf("write objects error %v", err)
			return e.ExitError(err)
		}
	}
	if err := p.WriteDeepenSparseMetadata(e.Context(), cc, c.DeepenFrom, c.Have, c.Deepen, paths); err != nil {
		logrus.Errorf("write commits error %v", err)
		return e.ExitError(err)
	}
	if err := p.Done(); err != nil {
		logrus.Errorf("finish metadata error %v", err)
		return e.ExitError(err)
	}
	return 0
}

func (s *Server) BatchMetadata(e *Session, depth int, useZSTD bool) int {
	oids, err := protocol.ReadInputOIDs(e)
	if err != nil {
		e.WriteError("batch-metadata: %v", err)
		return 400
	}
	rr, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer rr.Close()
	odb := rr.ODB()
	objects := make([]any, 0, len(oids))
	for _, oid := range oids {
		a, err := odb.Objects(e.Context(), oid)
		if err != nil {
			return e.ExitError(err)
		}
		objects = append(objects, a)
	}
	p, err := protocol.NewPipePacker(rr.ODB(), e, depth, useZSTD)
	if err != nil {
		logrus.Errorf("new packer error %v", err)
		return e.ExitError(err)
	}
	defer p.Close()
	for _, a := range objects {
		switch v := a.(type) {
		case *object.Commit:
			if err := p.WriteDeduplication(e.Context(), v, v.Hash); err != nil {
				logrus.Errorf("write commit error %v", err)
				return e.ExitError(err)
			}
			if err := p.WriteTree(e.Context(), v.Tree, 0); err != nil {
				logrus.Errorf("write tree error %v", err)
				return e.ExitError(err)
			}
		case *object.Tree:
			if err := p.WriteTree(e.Context(), v.Hash, 0); err != nil {
				logrus.Errorf("write tree error %v", err)
				return e.ExitError(err)
			}
		case *object.Tag:
			ro, err := rr.ParseRev(e.Context(), v.Object.String())
			if err != nil {
				return e.ExitError(err)
			}
			if err := p.WriteDeduplication(e.Context(), v, v.Hash); err != nil {
				logrus.Errorf("write fragments error %v", err)
				return e.ExitError(err)
			}
			for h, o := range ro.Objects {
				if err := p.WriteDeduplication(e.Context(), o, plumbing.NewHash(h)); err != nil {
					logrus.Errorf("write fragments error %v", err)
					return e.ExitError(err)
				}
			}
			target := ro.Target
			if err := p.WriteDeduplication(e.Context(), target, target.Hash); err != nil {
				logrus.Errorf("write fragments error %v", err)
				return e.ExitError(err)
			}
			if err := p.WriteTree(e.Context(), target.Tree, 0); err != nil {
				logrus.Errorf("write tree error %v", err)
				return e.ExitError(err)
			}
		case *object.Fragments:
			if err := p.WriteDeduplication(e.Context(), v, v.Hash); err != nil {
				logrus.Errorf("write fragments error %v", err)
				return e.ExitError(err)
			}
		}
	}
	if err := p.Done(); err != nil {
		logrus.Errorf("finish metadata error %v", err)
		return e.ExitError(err)
	}
	return 0
}
