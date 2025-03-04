// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/crc"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/zeta"
	"github.com/antgroup/hugescm/pkg/serve"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/antgroup/hugescm/pkg/serve/repo"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

const (
	//
	IfNoneMatch = "If-None-Match"
	ETag        = "ETag"
	// Zeta HTTP Header
	AUTHORIZATION        = "Authorization"
	ZETA_PROTOCOL        = "Zeta-Protocol"
	ZETA_COMMAND_OLDREV  = "X-Zeta-Command-OldRev"
	ZETA_COMMAND_NEWREV  = "X-Zeta-Command-NewRev"
	ZETA_TERMINAL        = "X-Zeta-Terminal"
	ZETA_OBJECTS_STATS   = "X-Zeta-Objects-Stats"
	ZETA_COMPRESSED_SIZE = "X-Zeta-Compressed-Size"
	// ZETA Protocol Content Type
	ZETA_MIME_BLOB          = "application/x-zeta-blob"
	ZETA_MIME_BLOBS         = "application/x-zeta-blobs"
	ZETA_MIME_MULTI_OBJECTS = "application/x-zeta-multi-objects"
	ZETA_MIME_MD            = "application/x-zeta-metadata"
	ZETA_MIME_COMPRESS_MD   = "application/x-zeta-compress-metadata"
	ZETA_MIME_REPORT_RESULT = "application/x-zeta-report-result"
	ZETA_MIME_VND_JSON      = "application/vnd.zeta+json"
	ZETA_MIME_FRAGMENTS_MD  = "application/vnd.zeta-fragments+json" // fragments json format
)

// ShareAuthorization: POST /{namespace}/{repo}/authorization
func (s *Server) ShareAuthorization(w http.ResponseWriter, r *http.Request) {
	var sa protocol.SASHandshake
	if err := json.NewDecoder(r.Body).Decode(&sa); err != nil {
		renderFailureFormat(w, r, http.StatusBadRequest, "decode handshake error: %v", err)
		return
	}
	req, err := s.basicAuth(w, r, sa.Operation, r.Header.Get(AUTHORIZATION))
	if err != nil {
		return
	}
	expiresAt := time.Now().Add(time.Hour * 2) // default 24h
	token, err := GenerateJWT(req.U, req.R.ID, sa.Operation, expiresAt)
	if err != nil {
		renderFailureFormat(w, r, http.StatusInternalServerError, "new token error: %v", err)
		return
	}
	JsonEncode(w, &protocol.SASPayload{
		Header: protocol.PayloadHeader{
			Authorization: BearerPrefix + token,
		},
		ExpiresAt: expiresAt,
	})
}

func (s *Server) LsBranchReference(w http.ResponseWriter, r *Request, branchName string) {
	b, err := s.db.FindBranch(r.Context(), r.R.ID, branchName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderFailureFormat(w, r.Request, http.StatusNotFound, r.W("branch '%s' not exist"), branchName)
			return
		}
		s.renderError(w, r, err)
		return
	}
	branch := &protocol.Reference{
		Remote:          r.makeRemoteURL(),
		Name:            protocol.BRANCH_PREFIX + b.Name,
		Hash:            b.Hash,
		HEAD:            protocol.BRANCH_PREFIX + r.R.DefaultBranch,
		Version:         int(protocol.PROTOCOL_VERSION),
		Agent:           s.serverName,
		HashAlgo:        r.R.HashAlgo,
		CompressionAlgo: r.R.CompressionAlgo,
	}
	ZetaEncodeVND(w, branch)
}

func (s *Server) LsTagReference(w http.ResponseWriter, r *Request, tagName string) {
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close()
	oid, peeled, err := rr.LsTag(r.Context(), tagName)
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	branch := &protocol.Reference{
		Remote:          r.makeRemoteURL(),
		Name:            protocol.TAG_PREFIX + tagName,
		Hash:            oid,
		Peeled:          peeled,
		HEAD:            protocol.BRANCH_PREFIX + r.R.DefaultBranch,
		Version:         int(protocol.PROTOCOL_VERSION),
		Agent:           s.serverName,
		HashAlgo:        r.R.HashAlgo,
		CompressionAlgo: r.R.CompressionAlgo,
	}
	ZetaEncodeVND(w, branch)
}

// GET /{namespace}/{repo}/reference/{refname:.*}
func (s *Server) LsReference(w http.ResponseWriter, r *Request) {
	refname, err := url.PathUnescape(mux.Vars(r.Request)["refname"])
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusNotFound, r.W("'%s' is not a valid reference name"), refname)
		return
	}
	if refname == protocol.HEAD {
		s.LsBranchReference(w, r, r.R.DefaultBranch)
		return
	}
	if branchName, ok := strings.CutPrefix(refname, protocol.BRANCH_PREFIX); ok {
		s.LsBranchReference(w, r, branchName)
		return
	}
	if tagName, ok := strings.CutPrefix(refname, protocol.TAG_PREFIX); ok {
		s.LsTagReference(w, r, tagName)
		return
	}
	if strings.HasPrefix(refname, protocol.REF_PREFIX) {
		// TODO: support pull or other refs ???
		renderFailureFormat(w, r.Request, http.StatusNotFound, r.W("reference '%s' not exist"), refname)
		return
	}
	s.LsBranchReference(w, r, refname)
}

// POST /{namespace}/{repo}/objects/batch
func (s *Server) BatchObjects(w http.ResponseWriter, r *Request) {
	oids, err := protocol.ReadInputOIDs(r.Body)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "batch-oids: %v", err)
		return
	}
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close()
	w.Header().Set("Content-Type", ZETA_MIME_BLOBS)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	buffedWriter := streamio.GetBufferWriter(w)
	defer func() {
		_ = buffedWriter.Flush()
		streamio.PutBufferWriter(buffedWriter)
	}()
	cw := crc.NewCrc64Writer(buffedWriter)
	if err := protocol.WriteBatchObjectsHeader(cw); err != nil {
		logrus.Errorf("write blob header error: %v", err)
		return
	}
	o := rr.ODB()
	writeFunc := func(oid plumbing.Hash) error {
		sr, err := o.Open(r.Context(), oid, 0)
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
			logrus.Errorf("batch-objects: write blob %s error: %v", oid, err)
			return
		}
	}
	_ = protocol.WriteObjectsItem(cw, nil, "", 0) // FLUSH
	if _, err := cw.Finish(); err != nil {
		logrus.Errorf("batch-objects: finish crc64 error: %v", err)
	}
}

// POST /{namespace}/{repo}/objects/share
func (s *Server) ShareObjects(w http.ResponseWriter, r *Request) {
	var request protocol.BatchShareObjectsRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "decode request body error: %v", err)
		return
	}
	rr, err := s.open(w, r)
	if err != nil {
		return
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
			renderFailureFormat(w, r.Request, http.StatusBadRequest, "require object is nil")
			return
		}
		want := plumbing.NewHash(o.OID)
		// oss shared download link
		ro, err := odb.Share(r.Context(), want, expiresAt)
		if err != nil {
			s.renderError(w, r, err)
			return
		}
		response.Objects = append(response.Objects, &protocol.Representation{
			OID:            want.String(),
			CompressedSize: ro.Size,
			Href:           ro.Href,
			ExpiresAt:      ExpiresAt,
		})
	}
	ZetaEncodeVND(w, response)
}

// GET /{namespace}/{repo}/objects/{oid}
func (s *Server) GetObject(w http.ResponseWriter, r *Request) {
	rg, err := protocol.ParseRangeEx(r.Request)
	if err != nil {
		renderFailure(w, r.Request, http.StatusBadRequest, err.Error())
		return
	}
	m := mux.Vars(r.Request)
	sid := m["oid"]
	if !plumbing.ValidateHashHex(sid) {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, r.W("'%s' is not a valid object name"), sid)
		return
	}
	repo, err := s.open(w, r)
	if err != nil {
		return
	}
	defer repo.Close()
	o := repo.ODB()
	sr, err := o.Open(r.Context(), plumbing.NewHash(sid), rg.Start)
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	defer sr.Close()
	w.Header().Set("Content-Type", ZETA_MIME_BLOB)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set(ZETA_COMPRESSED_SIZE, strconv.FormatInt(sr.Size(), 10))
	length := sr.Size()
	statusCode := http.StatusOK
	if rg.Start > 0 {
		// https://developer.mozilla.org/zh-CN/docs/Web/HTTP/Headers/Content-Range
		newRange := protocol.Range{Start: rg.Start, Length: sr.Size() - rg.Start}
		w.Header().Set("Content-Range", newRange.ContentRange(sr.Size()))
		length = newRange.Length
		statusCode = http.StatusPartialContent
	}
	w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(statusCode)
	if _, err := streamio.Copy(w, sr); err != nil {
		logrus.Errorf("copy error: %v", err)
	}
}

func (s *Server) updateBranchDryRun(w http.ResponseWriter, r *Request, branchName string) bool {
	if _, err := s.checkBranchCanUpdate(r.Context(), w, r, branchName); err != nil {
		if e, ok := err.(*zeta.ErrStatusCode); ok {
			renderFailure(w, r.Request, e.Code, e.Message)
			return false
		}
		renderFailureFormat(w, r.Request, http.StatusInternalServerError, "internal server error: %v", err)
		return false
	}
	return true
}

func (s *Server) updateReferenceDryRun(w http.ResponseWriter, r *Request) bool {
	m := mux.Vars(r.Request)
	escapedRefname := m["refname"]
	if len(escapedRefname) == 0 {
		renderFailureFormat(w, r.Request, http.StatusInternalServerError, "invalid url location %s", r.URL.Path)
		return false
	}
	unescapeRefname, err := url.PathUnescape(escapedRefname)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, r.W("'%s' is not a valid branch name"), escapedRefname)
		return false
	}
	refname := plumbing.ReferenceName(unescapeRefname)
	switch {
	case refname.IsBranch():
		return s.updateBranchDryRun(w, r, refname.BranchName())
	case refname.IsTag():
		//return s.updateTagDryRun(w, r, refname.TagName())
		return true
	case !strings.HasPrefix(unescapeRefname, plumbing.ReferencePrefix):
		return s.updateBranchDryRun(w, r, string(refname))
	}
	renderFailureFormat(w, r.Request, http.StatusNotImplemented, r.W("reference name '%s' is reserved"), refname)
	return false
}

// POST /{namespace}/{repo}/reference/{refname:.*}/objects/batch
func (s *Server) BatchCheck(w http.ResponseWriter, r *Request) {
	var request protocol.BatchCheckRequest
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "decode request body error: %v", err)
		return
	}
	if !s.updateReferenceDryRun(w, r) {
		return
	}
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close()
	response := &protocol.BatchCheckResponse{
		Objects: make([]*protocol.HaveObject, 0, len(request.Objects)),
	}
	odb := rr.ODB()
	for _, o := range request.Objects {
		if o == nil {
			renderFailureFormat(w, r.Request, http.StatusBadRequest, "require object is nil")
			return
		}
		oid := plumbing.NewHash(o.OID)
		si, err := odb.Stat(r.Context(), oid)
		if err == nil {
			response.Objects = append(response.Objects, &protocol.HaveObject{
				OID:            o.OID,
				CompressedSize: si.Size,
				Action:         string(protocol.DOWNLOAD),
			})
			continue
		}
		if !os.IsNotExist(err) {
			renderFailureFormat(w, r.Request, http.StatusInternalServerError, "upload object %s check error: %v", o.OID, err)
			return
		}
		response.Objects = append(response.Objects, &protocol.HaveObject{
			OID:            o.OID,
			CompressedSize: o.CompressedSize,
			Action:         string(protocol.UPLOAD),
		})
	}
	ZetaEncodeVND(w, response)
}

func checkName(r *Request) string {
	if r.U != nil {
		return r.U.Name
	}
	return "anonymous"
}

// PUT /{namespace}/{repo}/reference/{refname:.*}/objects/{oid}
func (s *Server) PutObject(w http.ResponseWriter, r *Request) {
	var err error
	var uploadSize int64
	if us := r.Header.Get(ZETA_COMPRESSED_SIZE); len(us) != 0 {
		if uploadSize, err = strconv.ParseInt(us, 10, 64); err != nil {
			renderFailureFormat(w, r.Request, http.StatusBadRequest, "'x-zeta-compressed-size' value not valid number: '%s'", us)
		}
	}
	sid := mux.Vars(r.Request)["oid"]
	if !plumbing.ValidateHashHex(sid) {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "invalid hash string: %s", sid)
		return
	}
	oid := plumbing.NewHash(sid)
	if !s.updateReferenceDryRun(w, r) {
		return
	}
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close()

	size, err := rr.ODB().WriteDirect(r.Context(), oid, r.Body, uploadSize)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusConflict, "upload object '%s' error: %v", err, sid)
		return
	}
	logrus.Infof("%s upload large object %s [size: %s] to %s [refname: %s] success", checkName(r), sid, strengthen.FormatSize(size), r.makeRemoteURL(), mux.Vars(r.Request)["refname"])
	ZetaEncodeVND(w, &protocol.ErrorCode{Code: 200, Message: "OK"})
}

const (
	GeneralBranch      = 0
	ProtectedBranch    = 10
	ArchivedBranch     = 20
	ConfidentialBranch = 30
)

func (s *Server) checkBranchCanUpdate(ctx context.Context, w http.ResponseWriter, r *Request, branchName string) (*database.Branch, error) {
	if !plumbing.ValidateBranchName([]byte(branchName)) {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, r.W("'%s' is not a valid branch name"), branchName)
		return nil, ErrStop
	}
	branch, err := s.db.FindBranch(ctx, r.R.ID, branchName)
	if database.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusInternalServerError, r.W("internal server error: %v"), err)
		return nil, ErrStop
	}
	switch branch.ProtectionLevel {
	case ConfidentialBranch:
		renderFailureFormat(w, r.Request, http.StatusNotFound, r.W("'%s' is archived, cannot be modified"), branchName)
		return nil, ErrStop
	case ArchivedBranch:
		renderFailureFormat(w, r.Request, http.StatusForbidden, r.W("'%s' is archived, cannot be modified"), branchName)
		return nil, ErrStop
	case ProtectedBranch:
		if !r.U.Administrator {
			renderFailureFormat(w, r.Request, http.StatusForbidden, r.W("'%s' is protected branch, cannot be modified"), branchName)
			return nil, ErrStop
		}
		return branch, nil
	default:
	}
	return branch, nil
}

// POST /{namespace}/{repo}/reference/{refname:.*}
func (s *Server) Push(w http.ResponseWriter, r *Request) {
	escapedRefname := mux.Vars(r.Request)["refname"]
	if len(escapedRefname) == 0 {
		renderFailureFormat(w, r.Request, http.StatusInternalServerError, "invalid url location %s", r.URL.Path)
		return
	}
	unescapeRefname, err := url.PathUnescape(escapedRefname)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, r.W("'%s' is not a valid branch name"), escapedRefname)
		return
	}
	if unescapeRefname == protocol.HEAD {
		s.BranchPush(w, r, r.R.DefaultBranch)
		return
	}
	if !plumbing.ValidateReferenceName([]byte(unescapeRefname)) {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, r.W("'%s' is not a valid branch name"), unescapeRefname)
		return
	}
	refname := plumbing.ReferenceName(unescapeRefname)
	switch {
	case refname.IsBranch():
		s.BranchPush(w, r, refname.BranchName())
		return
	case refname.IsTag():
		s.TagPush(w, r, refname.TagName())
		return
	case !strings.HasPrefix(unescapeRefname, plumbing.ReferencePrefix):
		s.BranchPush(w, r, string(refname))
		return
	}
	renderFailureFormat(w, r.Request, http.StatusNotImplemented, r.W("reference name '%s' is reserved"), refname)
}

func (s *Server) TagPush(w http.ResponseWriter, r *Request, tagName string) {
	tag, err := s.db.FindTag(r.Context(), r.R.ID, tagName)
	if err != nil && !database.IsErrRevisionNotFound(err) {
		renderFailureFormat(w, r.Request, http.StatusInternalServerError, r.W("internal server error: %v"), err)
		return
	}
	command := &repo.Command{
		RID:           r.R.ID,
		UID:           r.U.ID,
		ReferenceName: plumbing.NewTagReferenceName(tagName),
		OldRev:        r.Header.Get("X-Zeta-Command-OldRev"),
		NewRev:        r.Header.Get("X-Zeta-Command-NewRev"),
		Terminal:      r.Header.Get("X-Zeta-Terminal"),
		Language:      serve.Language(r.Request),
	}
	if !plumbing.ValidateHashHex(command.NewRev) {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "NewRev '%s' is bad commit", command.NewRev)
		return
	}
	if !plumbing.ValidateHashHex(command.OldRev) {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "OldRev '%s' is bad commit", command.OldRev)
		return
	}
	if tag != nil && tag.Hash != command.OldRev {
		renderFailure(w, r.Request, http.StatusConflict, r.W("tag is updated, please update and try again"))
		return
	}
	command.UpdateStats(r.Header.Get("X-Zeta-Objects-Stats"))
	rr, err := s.open(w, r)
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	defer rr.Close()

	w.Header().Set("Content-Type", ZETA_MIME_REPORT_RESULT)
	w.Header().Set("Cache-Control", "no-cache")
	if err = rr.DoPush(r.Context(), command, r.Body, w); err != nil {
		if es, ok := err.(*zeta.ErrStatusCode); ok {
			renderFailure(w, r.Request, es.Code, es.Message)
		}
		return
	}
}

func (s *Server) BranchPush(w http.ResponseWriter, r *Request, branchName string) {
	oldBranch, err := s.checkBranchCanUpdate(r.Context(), w, r, branchName)
	if err != nil {
		return
	}
	command := &repo.Command{
		RID:           r.R.ID,
		UID:           r.U.ID,
		ReferenceName: plumbing.NewBranchReferenceName(branchName),
		OldRev:        r.Header.Get("X-Zeta-Command-OldRev"),
		NewRev:        r.Header.Get("X-Zeta-Command-NewRev"),
		Terminal:      r.Header.Get("X-Zeta-Terminal"),
		Language:      serve.Language(r.Request),
	}
	if !plumbing.ValidateHashHex(command.NewRev) {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "NewRev '%s' is bad commit", command.NewRev)
		return
	}
	if !plumbing.ValidateHashHex(command.OldRev) {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "OldRev '%s' is bad commit", command.OldRev)
		return
	}
	if oldBranch != nil && oldBranch.Hash != command.OldRev {
		renderFailure(w, r.Request, http.StatusConflict, r.W("branch is updated, please update and try again"))
		return
	}
	command.UpdateStats(r.Header.Get("X-Zeta-Objects-Stats"))
	rr, err := s.open(w, r)
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	defer rr.Close()

	w.Header().Set("Content-Type", ZETA_MIME_REPORT_RESULT)
	w.Header().Set("Cache-Control", "no-cache")
	if err = rr.DoPush(r.Context(), command, r.Body, w); err != nil {
		if es, ok := err.(*zeta.ErrStatusCode); ok {
			renderFailure(w, r.Request, es.Code, es.Message)
		}
		return
	}
}
