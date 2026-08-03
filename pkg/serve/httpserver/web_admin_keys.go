// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type webAdminUserKeysData struct {
	UID          int64
	UserName     string
	Keys         []webKeyRow
	InputTitle   string
	InputContent string
	Error        string
}

// handleWebAdminUserKeys renders the target user's SSH keys with an add form.
// Admin-only (guarded by webAdminMiddleware). Mirrors handleWebAccountKeys
// but scoped to a target uid instead of the current user.
func (s *Server) handleWebAdminUserKeys(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	uid, ok := parseWebUID(w, r)
	if !ok {
		return
	}
	target, err := s.db.FindUser(r.Context(), uid)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	s.renderAdminUserKeys(w, r.Context(), u, target, "", "", "")
}

// handleWebAdminAddKeyForUser adds an SSH key for a target user. Mirrors
// handleCreateKey (api_keys.go) but resolves the target by {uid}.
func (s *Server) handleWebAdminAddKeyForUser(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	uid, ok := parseWebUID(w, r)
	if !ok {
		return
	}
	target, err := s.db.FindUser(r.Context(), uid)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	title := r.PostFormValue("title")
	content := r.PostFormValue("content")
	if title == "" || content == "" {
		s.renderAdminUserKeys(w, r.Context(), u, target, title, content, "title and public key are required")
		return
	}
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(content))
	if err != nil {
		s.renderAdminUserKeys(w, r.Context(), u, target, title, content, fmt.Sprintf("bad public key: %v", err))
		return
	}
	if _, err := s.db.AddKey(r.Context(), &database.Key{
		UID:         target.ID,
		Content:     content,
		Title:       title,
		Type:        database.BasicKey,
		Fingerprint: ssh.FingerprintSHA256(pk),
	}); err != nil {
		logrus.Errorf("web admin: add ssh key for user %d error: %v", target.ID, err)
		s.renderAdminUserKeys(w, r.Context(), u, target, title, content, err.Error())
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/users/%d/keys", uid), http.StatusSeeOther)
}

// handleWebAdminDeleteKeyForUser removes a key of a target user. Verifies the
// key belongs to that target before deleting, mirroring handleDeleteKey.
func (s *Server) handleWebAdminDeleteKeyForUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := parseWebUID(w, r)
	if !ok {
		return
	}
	kid, err := strconv.ParseInt(chi.URLParam(r, "kid"), 10, 64)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	k, err := s.db.FindKey(r.Context(), kid)
	if err != nil {
		http.Error(w, "key not found", http.StatusNotFound)
		return
	}
	if k.UID != uid {
		http.Error(w, "key does not belong to target user", http.StatusForbidden)
		return
	}
	if err := s.db.DeleteKey(r.Context(), kid); err != nil {
		logrus.Errorf("web admin: delete ssh key error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/users/%d/keys", uid), http.StatusSeeOther)
}

func (s *Server) renderAdminUserKeys(w http.ResponseWriter, ctx context.Context, u *database.User, target *database.User, inTitle, inContent, errMsg string) {
	keys, err := s.db.ListKeysByUser(ctx, target.ID)
	if err != nil {
		logrus.Errorf("web admin: list ssh keys for user %d error: %v", target.ID, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	rows := make([]webKeyRow, 0, len(keys))
	for _, k := range keys {
		rows = append(rows, webKeyRow{
			ID:          k.ID,
			Title:       k.Title,
			Fingerprint: k.Fingerprint,
			CreatedAt:   k.CreatedAt,
		})
	}
	pageData := &webTemplateData{
		Title:    fmt.Sprintf("%s — SSH keys", target.UserName),
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content: &webAdminUserKeysData{
			UID:          target.ID,
			UserName:     target.UserName,
			Keys:         rows,
			InputTitle:   inTitle,
			InputContent: inContent,
			Error:        errMsg,
		},
	}
	s.renderer.renderPage(w, s.serverName, "admin_user_keys", pageData)
}
