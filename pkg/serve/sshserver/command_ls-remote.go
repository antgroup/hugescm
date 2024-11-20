// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sshserver

import (
	"strings"

	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
)

// zeta-serve ls-remote "group/mono-zeta" --reference "${REFNAME}"
type LsRemote struct {
	Path      string
	Reference string
}

func (c *LsRemote) ParseArgs(args []string) error {
	var p ParseArgs
	p.Add("reference", REQUIRED, 'R')
	if err := p.Parse(args, func(index rune, nextArg, raw string) error {
		switch index {
		case 'R':
			c.Reference = nextArg
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

func (c *LsRemote) Exec(ctx *RunCtx) int {
	return ctx.S.LsRemote(ctx.Session, c.Path, c.Reference)
}

func (s *Server) LsRemote(e *Session, repoPath, refname string) int {
	if exitCode := s.doPermissionCheck(e, repoPath, protocol.DOWNLOAD); exitCode != 0 {
		return exitCode
	}
	if len(refname) == 0 || refname == protocol.HEAD {
		return s.LsBranchReference(e, e.DefaultBranch)
	}
	if branchName, ok := strings.CutPrefix(refname, protocol.BRANCH_PREFIX); ok {
		return s.LsBranchReference(e, branchName)
	}
	if tagName, ok := strings.CutPrefix(refname, protocol.TAG_PREFIX); ok {
		return s.LsTagReference(e, tagName)
	}
	if strings.HasPrefix(refname, protocol.REF_PREFIX) {
		// TODO: support pull or other refs ???
		e.WriteError("reference '%s' not exist", refname)
		return 404
	}
	return s.LsBranchReference(e, refname)
}

func (s *Server) LsBranchReference(e *Session, branchName string) int {
	b, err := s.db.FindBranch(e.Context(), e.RID, branchName)
	if err != nil {
		if database.IsErrRevisionNotFound(err) {
			e.WriteError(e.W("branch '%s' not exist"), err)
			return 404
		}
		e.WriteError("find branch error: %v", err)
		return 500
	}
	branch := &protocol.Reference{
		Remote:          e.makeRemoteURL(s.Endpoint),
		Name:            protocol.BRANCH_PREFIX + b.Name,
		Hash:            b.Hash,
		HEAD:            protocol.BRANCH_PREFIX + e.DefaultBranch,
		Version:         int(protocol.PROTOCOL_VERSION),
		Agent:           s.serverName,
		HashAlgo:        e.HashAlgo,
		CompressionAlgo: e.CompressionAlgo,
	}
	ZetaEncodeVND(e, branch)
	return 0
}

func (s *Server) LsTagReference(e *Session, tagName string) int {
	rr, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer rr.Close()
	oid, peeled, err := rr.LsTag(e.Context(), tagName)
	if err != nil {
		return e.ExitError(err)
	}
	branch := &protocol.Reference{
		Remote:          e.makeRemoteURL(s.Endpoint),
		Name:            protocol.TAG_PREFIX + tagName,
		Hash:            oid,
		Peeled:          peeled,
		HEAD:            protocol.BRANCH_PREFIX + e.DefaultBranch,
		Version:         int(protocol.PROTOCOL_VERSION),
		Agent:           s.serverName,
		HashAlgo:        e.HashAlgo,
		CompressionAlgo: e.CompressionAlgo,
	}
	ZetaEncodeVND(e, branch)
	return 0
}
