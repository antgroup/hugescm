// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"encoding/json"
	"math"
	"net/http"
	"strconv"

	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/pkg/serve/argon2id"
	"github.com/antgroup/hugescm/pkg/serve/database"
)

// paginationParams extracts pagination query parameters from the request.
// Defaults: page=1, perPage=20, max perPage=100.
func paginationParams(r *http.Request) (page, perPage, offset int) {
	page, perPage = 1, 20
	if p := r.URL.Query().Get("page"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			page = n
		}
	}
	if pp := r.URL.Query().Get("per_page"); pp != "" {
		if n, err := strconv.Atoi(pp); err == nil && n > 0 && n <= 100 {
			perPage = n
		}
	}
	offset = (page - 1) * perPage
	return
}

// writePaginationHeaders sets X-Total and X-Total-Pages response headers.
func writePaginationHeaders(w http.ResponseWriter, total int64, perPage int) {
	w.Header().Set("X-Total", strconv.FormatInt(total, 10))
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	w.Header().Set("X-Total-Pages", strconv.Itoa(totalPages))
}

// handleListUsers returns a paginated list of all users. Admin only.
func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	if !requireAdmin(w, req) {
		return
	}
	page, perPage, _ := paginationParams(r)
	users, total, err := s.db.ListUsers(r.Context(), page, perPage)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	writePaginationHeaders(w, total, perPage)
	JsonEncode(w, users)
}

type createUserRequest struct {
	UserName      string `json:"username"`
	Name          string `json:"name,omitempty"`
	Administrator bool   `json:"administrator"`
	Email         string `json:"email"`
	Password      string `json:"password"`
	Type          int    `json:"type,omitempty"`
}

// handleCreateUser creates a new user. Admin only.
func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	if !requireAdmin(w, req) {
		return
	}
	var body createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderFailureFormat(w, r, http.StatusBadRequest, "input body error: %v", err)
		return
	}
	if len(body.UserName) == 0 || len(body.Password) == 0 {
		renderFailure(w, r, http.StatusBadRequest, "username or password is empty")
		return
	}
	if len(body.Name) == 0 {
		body.Name = body.UserName
	}
	passwd, err := argon2id.CreateHash(body.Password, argon2id.DefaultParams)
	if err != nil {
		renderFailureFormat(w, r, http.StatusInternalServerError, "gen salt password error: %v", err)
		return
	}
	u, err := s.db.NewUser(r.Context(), &database.User{
		UserName:       body.UserName,
		Name:           body.Name,
		Administrator:  body.Administrator,
		Email:          body.Email,
		Type:           database.UserType(body.Type),
		Password:       passwd,
		SignatureToken: strengthen.NewRID(),
	})
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	u.Guard()
	JsonEncode(w, u)
}

// handleGetUser returns a single user by ID. Self or admin only.
func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	target, ok := s.requireTargetUser(w, req)
	if !ok {
		return
	}
	target.Guard()
	JsonEncode(w, target)
}

type updateUserRequest struct {
	Name  *string `json:"name,omitempty"`
	Email *string `json:"email,omitempty"`
}

// handleUpdateUser updates a user's profile (name, email). Self or admin only.
func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	target, ok := s.requireTargetUser(w, req)
	if !ok {
		return
	}
	var body updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderFailureFormat(w, r, http.StatusBadRequest, "input body error: %v", err)
		return
	}
	if body.Name != nil {
		target.Name = *body.Name
	}
	if body.Email != nil {
		target.Email = *body.Email
	}
	updated, err := s.db.UpdateUser(r.Context(), target)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	updated.Guard()
	JsonEncode(w, updated)
}

// handleDeleteUser soft-deletes a user. Admin only.
func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	if !requireAdmin(w, req) {
		return
	}
	target, ok := s.requireTargetUser(w, req)
	if !ok {
		return
	}
	if err := s.db.SoftDeleteUser(r.Context(), target.ID); err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleLockUser locks a user account. Admin only.
func (s *Server) handleLockUser(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	if !requireAdmin(w, req) {
		return
	}
	target, ok := s.requireTargetUser(w, req)
	if !ok {
		return
	}
	locked, err := s.db.LockUser(r.Context(), target.ID)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	locked.Guard()
	JsonEncode(w, locked)
}

// handleUnlockUser unlocks a user account. Admin only.
func (s *Server) handleUnlockUser(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	if !requireAdmin(w, req) {
		return
	}
	target, ok := s.requireTargetUser(w, req)
	if !ok {
		return
	}
	unlocked, err := s.db.UnlockUser(r.Context(), target.ID)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	unlocked.Guard()
	JsonEncode(w, unlocked)
}

// handleCurrentUserProfile returns the current authenticated user's profile.
func (s *Server) handleCurrentUserProfile(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	u, err := s.db.FindUser(r.Context(), req.U.ID)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	u.Guard()
	JsonEncode(w, u)
}

type changePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

// handleChangePassword changes the current user's password.
func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	var body changePasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderFailureFormat(w, r, http.StatusBadRequest, "input body error: %v", err)
		return
	}
	if len(body.OldPassword) == 0 || len(body.NewPassword) == 0 {
		renderFailure(w, r, http.StatusBadRequest, "old_password or new_password is empty")
		return
	}
	// Reload user to get the password hash
	u, err := s.db.FindUser(r.Context(), req.U.ID)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	ok, err := argon2id.ComparePasswordAndHash(body.OldPassword, u.Password)
	if err != nil {
		renderFailure(w, r, http.StatusInternalServerError, "broken salted password")
		return
	}
	if !ok {
		renderFailure(w, r, http.StatusForbidden, "old password unmatched")
		return
	}
	passwd, err := argon2id.CreateHash(body.NewPassword, argon2id.DefaultParams)
	if err != nil {
		renderFailureFormat(w, r, http.StatusInternalServerError, "gen salt password error: %v", err)
		return
	}
	u.Password = passwd
	_, err = s.db.UpdateUser(r.Context(), u)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	JsonEncode(w, map[string]string{"message": "password changed successfully"})
}
