// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/go-chi/chi/v5"
)

// handleListMembers returns all members of a repository (project members).
// Requires master access or higher.
func (s *Server) handleListMembers(w http.ResponseWriter, r *Request) {
	if !s.requireRepoMaster(w, r) {
		return
	}
	members, err := s.db.ListMembers(r.Context(), r.R.ID, database.ProjectMember)
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	if members == nil {
		members = []*database.Member{}
	}
	JsonEncode(w, members)
}

type addMemberRequest struct {
	UID         int64  `json:"uid"`
	AccessLevel int    `json:"access_level"`
	ExpiresAt   string `json:"expires_at,omitempty"` // ISO 8601 format
}

// handleAddMember adds a new member to a repository. Requires owner access.
func (s *Server) handleAddMember(w http.ResponseWriter, r *Request) {
	if !s.requireRepoAdmin(w, r) {
		return
	}
	var body addMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "input body error: %v", err)
		return
	}
	if body.UID == 0 {
		renderFailure(w, r.Request, http.StatusBadRequest, "uid is required")
		return
	}
	level := database.AccessLevel(body.AccessLevel)
	if level < database.NoneAccess || level > database.OwnerAccess {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "invalid access_level: %d", body.AccessLevel)
		return
	}

	var expiresAt time.Time
	if len(body.ExpiresAt) != 0 {
		t, err := time.Parse(time.RFC3339, body.ExpiresAt)
		if err != nil {
			renderFailureFormat(w, r.Request, http.StatusBadRequest, "invalid expires_at format: %v", err)
			return
		}
		expiresAt = t
	}

	if err := s.db.AddMember(r.Context(), &database.Member{
		UID:         body.UID,
		AccessLevel: level,
		SourceID:    r.R.ID,
		SourceType:  database.ProjectMember,
		ExpiresAt:   expiresAt,
	}); err != nil {
		s.renderError(w, r, err)
		return
	}
	JsonEncode(w, map[string]string{"message": "member added successfully"})
}

type updateMemberRequest struct {
	AccessLevel *int    `json:"access_level,omitempty"`
	ExpiresAt   *string `json:"expires_at,omitempty"` // ISO 8601 format
}

// handleUpdateMember updates a member's access level or expiration. Requires owner access.
func (s *Server) handleUpdateMember(w http.ResponseWriter, r *Request) {
	if !s.requireRepoAdmin(w, r) {
		return
	}

	memberUIDStr := chi.URLParam(r.Request, "member_uid")
	memberUID, err := strconv.ParseInt(memberUIDStr, 10, 64)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "invalid member uid: %s", memberUIDStr)
		return
	}

	members, err := s.db.ListMembers(r.Context(), r.R.ID, database.ProjectMember)
	if err != nil {
		s.renderError(w, r, err)
		return
	}

	var targetMember *database.Member
	for _, m := range members {
		if m.UID == memberUID {
			targetMember = m
			break
		}
	}
	if targetMember == nil {
		renderFailureFormat(w, r.Request, http.StatusNotFound, "member with uid %d not found", memberUID)
		return
	}

	var body updateMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "input body error: %v", err)
		return
	}

	if body.AccessLevel != nil {
		level := database.AccessLevel(*body.AccessLevel)
		if level < database.NoneAccess || level > database.OwnerAccess {
			renderFailureFormat(w, r.Request, http.StatusBadRequest, "invalid access_level: %d", *body.AccessLevel)
			return
		}
		targetMember.AccessLevel = level
	}
	if body.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *body.ExpiresAt)
		if err != nil {
			renderFailureFormat(w, r.Request, http.StatusBadRequest, "invalid expires_at format: %v", err)
			return
		}
		targetMember.ExpiresAt = t
	}

	if err := s.db.UpdateMember(r.Context(), targetMember); err != nil {
		s.renderError(w, r, err)
		return
	}
	JsonEncode(w, map[string]string{"message": "member updated successfully"})
}

// handleRemoveMember removes a member from a repository. Requires owner access.
func (s *Server) handleRemoveMember(w http.ResponseWriter, r *Request) {
	if !s.requireRepoAdmin(w, r) {
		return
	}

	memberUIDStr := chi.URLParam(r.Request, "member_uid")
	memberUID, err := strconv.ParseInt(memberUIDStr, 10, 64)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "invalid member uid: %s", memberUIDStr)
		return
	}

	members, err := s.db.ListMembers(r.Context(), r.R.ID, database.ProjectMember)
	if err != nil {
		s.renderError(w, r, err)
		return
	}

	var targetMember *database.Member
	for _, m := range members {
		if m.UID == memberUID {
			targetMember = m
			break
		}
	}
	if targetMember == nil {
		renderFailureFormat(w, r.Request, http.StatusNotFound, "member with uid %d not found", memberUID)
		return
	}

	if err := s.db.RemoveMember(r.Context(), targetMember.ID); err != nil {
		s.renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
