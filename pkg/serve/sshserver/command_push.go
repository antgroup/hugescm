// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sshserver

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/zeta"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/antgroup/hugescm/pkg/serve/repo"
	"github.com/sirupsen/logrus"
)

// zeta-serve push "group/mono-zeta" --reference "$REFNAME" --batch-check

// zeta-serve push "group/mono-zeta" --reference "$REFNAME" --oid "$OID" --size "${SIZE}"

// zeta-serve push "group/mono-zeta" --reference "$REFNAME" --old-rev "$OLD_REV" --new-rev "$NEW_REV"

type Push struct {
	Path       string
	Reference  string
	OID        plumbing.Hash
	Size       int64
	OldRev     plumbing.Hash
	NewRev     plumbing.Hash
	BatchCheck bool
}

func (c *Push) ParseArgs(args []string) error {
	var p ParseArgs
	p.Add("reference", REQUIRED, 'R').
		Add("oid", REQUIRED, 'O').
		Add("batch-check", NOARG, 'B').
		Add("size", REQUIRED, 'S').
		Add("old-rev", REQUIRED, 'o').
		Add("new-rev", REQUIRED, 'n')
	if err := p.Parse(args, func(index rune, nextArg, raw string) error {
		switch index {
		case 'R':
			c.Reference = nextArg
		case 'O':
			if !plumbing.ValidateHashHex(nextArg) {
				return fmt.Errorf("oid is invalid hash: %s", nextArg)
			}
			c.OID = plumbing.NewHash(nextArg)
		case 'B':
			c.BatchCheck = true
		case 'S':
			size, err := strconv.ParseInt(nextArg, 10, 64)
			if err != nil {
				return fmt.Errorf("parse '--size': %s error: %s", nextArg, err)
			}
			if size < 0 {
				return errors.New("--size cannot be less than 0")
			}
			c.Size = size
		case 'n':
			if !plumbing.ValidateHashHex(nextArg) {
				return fmt.Errorf("new-rev is invalid hash: %s", nextArg)
			}
			c.NewRev = plumbing.NewHash(nextArg)
		case 'o':
			if !plumbing.ValidateHashHex(nextArg) {
				return fmt.Errorf("old-rev is invalid hash: %s", nextArg)
			}
			c.OldRev = plumbing.NewHash(nextArg)
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

func (c *Push) Exec(ctx *RunCtx) int {
	if exitCode := ctx.S.doPermissionCheck(ctx.Session, c.Path, protocol.UPLOAD); exitCode != 0 {
		return exitCode
	}
	if c.BatchCheck {
		return ctx.S.BatchCheck(ctx.Session, c.Reference)
	}
	if c.OID.IsZero() {
		return ctx.S.Push(ctx.Session, c.Reference, c.OldRev, c.NewRev)
	}
	return ctx.S.PutObject(ctx.Session, c.Reference, c.OID, c.Size)
}

func (s *Server) BatchCheck(e *Session, refname string) int {
	var request protocol.BatchCheckRequest
	if err := json.NewDecoder(e).Decode(&request); err != nil {
		return e.ExitFormat(400, "decode request body error: %v", err)
	}
	if exitCode := s.updateReferenceDryRun(e, refname); exitCode != 0 {
		return exitCode
	}
	rr, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer rr.Close() // nolint
	response := &protocol.BatchCheckResponse{
		Objects: make([]*protocol.HaveObject, 0, len(request.Objects)),
	}
	odb := rr.ODB()
	for _, o := range request.Objects {
		if o == nil {
			return e.ExitFormat(400, "require object is nil")
		}
		oid := plumbing.NewHash(o.OID)
		si, err := odb.Stat(e.Context(), oid)
		if err == nil {
			response.Objects = append(response.Objects, &protocol.HaveObject{
				OID:            o.OID,
				CompressedSize: si.Size,
				Action:         string(protocol.DOWNLOAD),
			})
			continue
		}
		if !os.IsNotExist(err) {
			return e.ExitFormat(500, "upload object %s check error: %v", o.OID, err)
		}
		response.Objects = append(response.Objects, &protocol.HaveObject{
			OID:            o.OID,
			CompressedSize: o.CompressedSize,
			Action:         string(protocol.UPLOAD),
		})
	}
	ZetaEncodeVND(e, response)
	return 0
}

func (s *Server) PutObject(e *Session, refname string, oid plumbing.Hash, compressedSize int64) int {
	if exitCode := s.updateReferenceDryRun(e, refname); exitCode != 0 {
		return exitCode
	}
	rr, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer rr.Close() // nolint

	size, err := rr.ODB().WriteDirect(e.Context(), oid, e, compressedSize)
	if err != nil {
		return e.ExitFormat(409, "upload object '%s' error: %v", oid, err)
	}
	logrus.Infof("%s upload large object %s [size: %s] to %s [refname: %s] success", e.UserName, oid, strengthen.FormatSize(size), e.makeRemoteURL(s.Endpoint), refname)
	ZetaEncodeVND(e, &protocol.ErrorCode{Code: 200, Message: "OK"})
	return 0
}

func (s *Server) Push(e *Session, referenceName string, oldRev, newRev plumbing.Hash) int {
	if referenceName == protocol.HEAD {
		return s.BranchPush(e, e.DefaultBranch, oldRev, newRev)
	}
	if !plumbing.ValidateReferenceName([]byte(referenceName)) {
		return e.ExitFormat(400, e.W("'%s' is not a valid branch name"), referenceName)
	}
	refname := plumbing.ReferenceName(referenceName)
	switch {
	case refname.IsBranch():
		return s.BranchPush(e, refname.BranchName(), oldRev, newRev)
	case refname.IsTag():
		return s.TagPush(e, refname.TagName(), oldRev, newRev)
	case !strings.HasPrefix(referenceName, plumbing.ReferencePrefix):
		return s.BranchPush(e, referenceName, oldRev, newRev)
	}
	return e.ExitFormat(501, e.W("reference name '%s' is reserved"), referenceName)
}

func (s *Server) TagPush(e *Session, tagName string, oldRev, newRev plumbing.Hash) int {
	tag, err := s.db.FindTag(e.Context(), e.RID, tagName)
	if err != nil && !database.IsErrRevisionNotFound(err) {
		return e.ExitFormat(500, e.W("internal server error: %v"), err)
	}
	command := &repo.Command{
		RID:           e.RID,
		UID:           e.UID,
		ReferenceName: plumbing.NewTagReferenceName(tagName),
		OldRev:        oldRev.String(),
		NewRev:        newRev.String(),
		Terminal:      e.Getenv("TERM"),
		Language:      e.language,
	}
	if tag != nil && tag.Hash != command.OldRev {
		return e.ExitFormat(409, "%s", e.W("tag is updated, please update and try again")) //nolint:govet
	}
	command.UpdateStats(e.Getenv("ZETA_OBJECTS_STATS"))
	rr, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer rr.Close() // nolint
	if err = rr.DoPush(e.Context(), command, e, e); err != nil {
		if es, ok := err.(*zeta.ErrStatusCode); ok {
			return e.ExitFormat(es.Code, "reason: %v", err)
		}
		return e.ExitError(err)
	}
	return 0
}

func (s *Server) BranchPush(e *Session, branchName string, oldRev, newRev plumbing.Hash) int {
	oldBranch, exitCode := s.checkBranchCanUpdate(e, branchName)
	if exitCode != 0 {
		return exitCode
	}
	command := &repo.Command{
		RID:           e.RID,
		UID:           e.UID,
		ReferenceName: plumbing.NewBranchReferenceName(branchName),
		OldRev:        oldRev.String(),
		NewRev:        newRev.String(),
		Terminal:      e.Getenv("TERM"),
		Language:      e.language,
	}
	if oldBranch != nil && oldBranch.Hash != command.OldRev {
		return e.ExitFormat(409, "%s", e.W("branch is updated, please update and try again")) //nolint:govet
	}
	command.UpdateStats(e.Getenv("ZETA_OBJECTS_STATS"))
	rr, err := s.open(e)
	if err != nil {
		return e.ExitError(err)
	}
	defer rr.Close() // nolint
	if err = rr.DoPush(e.Context(), command, e, e); err != nil {
		if es, ok := err.(*zeta.ErrStatusCode); ok {
			return e.ExitFormat(es.Code, "reason: %v", err)
		}
		return e.ExitError(err)
	}
	return 0
}

const (
	GeneralBranch      = 0
	ProtectedBranch    = 10
	ArchivedBranch     = 20
	ConfidentialBranch = 30
)

func (s *Server) checkBranchCanUpdate(e *Session, branchName string) (*database.Branch, int) {
	if !plumbing.ValidateBranchName([]byte(branchName)) {
		return nil, e.ExitFormat(400, e.W("'%s' is not a valid branch name"), branchName)
	}
	branch, err := s.db.FindBranch(e.Context(), e.RID, branchName)
	if database.IsNotFound(err) {
		return nil, 0
	}
	if err != nil {
		return nil, e.ExitFormat(500, e.W("internal server error: %v"), err)
	}
	switch branch.ProtectionLevel {
	case ConfidentialBranch:
		return nil, e.ExitFormat(404, e.W("'%s' is archived, cannot be modified"), branchName)
	case ArchivedBranch:
		return nil, e.ExitFormat(403, e.W("'%s' is archived, cannot be modified"), branchName)
	case ProtectedBranch:
		if !e.IsAdministrator {
			return nil, e.ExitFormat(403, e.W("'%s' is protected branch, cannot be modified"), branchName)
		}
		return branch, 0
	default:
	}
	return branch, 0
}

func (s *Server) updateReferenceDryRun(e *Session, reference string) int {
	refname := plumbing.ReferenceName(reference)
	switch {
	case refname.IsBranch():
		_, exitCode := s.checkBranchCanUpdate(e, refname.BranchName())
		return exitCode
	case refname.IsTag():
		//return s.updateTagDryRun(w, r, refname.TagName())
		return 0
	case !strings.HasPrefix(reference, plumbing.ReferencePrefix):
		_, exitCode := s.checkBranchCanUpdate(e, string(refname))
		return exitCode
	}
	return e.ExitFormat(501, e.W("reference name '%s' is reserved"), refname)
}
