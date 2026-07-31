// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/ssh"
)

// handleListKeys returns all SSH keys for a user. Self or admin only.
func (s *Server) handleListKeys(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	target, ok := s.requireTargetUser(w, req)
	if !ok {
		return
	}
	keys, err := s.db.ListKeysByUser(r.Context(), target.ID)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	if keys == nil {
		keys = []*database.Key{}
	}
	JsonEncode(w, keys)
}

type createKeyRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

// handleCreateKey adds an SSH key for a user. Self or admin only.
func (s *Server) handleCreateKey(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	target, ok := s.requireTargetUser(w, req)
	if !ok {
		return
	}
	var body createKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderFailureFormat(w, r, http.StatusBadRequest, "input body error: %v", err)
		return
	}
	if len(body.Title) == 0 {
		renderFailure(w, r, http.StatusBadRequest, "title is empty")
		return
	}
	if len(body.Content) == 0 {
		renderFailure(w, r, http.StatusBadRequest, "content is empty")
		return
	}
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(body.Content))
	if err != nil {
		renderFailureFormat(w, r, http.StatusBadRequest, "bad public key: %v", err)
		return
	}
	k, err := s.db.AddKey(r.Context(), &database.Key{
		UID:         target.ID,
		Content:     body.Content,
		Title:       body.Title,
		Type:        database.BasicKey,
		Fingerprint: ssh.FingerprintSHA256(pk),
	})
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	JsonEncode(w, k)
}

// handleGetKey returns a single SSH key. Self or admin, and the key must belong to the target user.
func (s *Server) handleGetKey(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	target, ok := s.requireTargetUser(w, req)
	if !ok {
		return
	}
	kidStr := chi.URLParam(r, "kid")
	kid, err := strconv.ParseInt(kidStr, 10, 64)
	if err != nil {
		renderFailureFormat(w, r, http.StatusBadRequest, "invalid key id: %s", kidStr)
		return
	}
	k, err := s.db.FindKey(r.Context(), kid)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	if k.UID != target.ID {
		renderFailure(w, r, http.StatusForbidden, "key does not belong to target user")
		return
	}
	JsonEncode(w, k)
}

// handleDeleteKey deletes an SSH key. Self or admin, and the key must belong to the target user.
func (s *Server) handleDeleteKey(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	target, ok := s.requireTargetUser(w, req)
	if !ok {
		return
	}
	kidStr := chi.URLParam(r, "kid")
	kid, err := strconv.ParseInt(kidStr, 10, 64)
	if err != nil {
		renderFailureFormat(w, r, http.StatusBadRequest, "invalid key id: %s", kidStr)
		return
	}
	k, err := s.db.FindKey(r.Context(), kid)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	if k.UID != target.ID {
		renderFailure(w, r, http.StatusForbidden, "key does not belong to target user")
		return
	}
	if err := s.db.DeleteKey(r.Context(), kid); err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
