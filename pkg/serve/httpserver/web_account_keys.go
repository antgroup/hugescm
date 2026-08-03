// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

type webKeyRow struct {
	ID          int64
	Title       string
	Fingerprint string
	CreatedAt   time.Time
}

type webKeysData struct {
	Keys         []webKeyRow
	InputTitle   string
	InputContent string
	Error        string
}

// handleWebAccountKeys renders the current user's SSH keys with an add form.
func (s *Server) handleWebAccountKeys(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	s.renderAccountKeys(w, r.Context(), u, "", "", "")
}

// handleWebAccountAddKey adds an SSH public key for the current user. Mirrors
// handleCreateKey (api_keys.go): parse the authorized key, then AddKey with the
// SHA-256 fingerprint.
func (s *Server) handleWebAccountAddKey(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	title := r.PostFormValue("title")
	content := r.PostFormValue("content")
	if title == "" || content == "" {
		s.renderAccountKeys(w, r.Context(), u, title, content, "title and public key are required")
		return
	}
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(content))
	if err != nil {
		s.renderAccountKeys(w, r.Context(), u, title, content, fmt.Sprintf("bad public key: %v", err))
		return
	}
	if _, err := s.db.AddKey(r.Context(), &database.Key{
		UID:         u.ID,
		Content:     content,
		Title:       title,
		Type:        database.BasicKey,
		Fingerprint: ssh.FingerprintSHA256(pk),
	}); err != nil {
		logrus.Errorf("web account: add ssh key error: %v", err)
		s.renderAccountKeys(w, r.Context(), u, title, content, err.Error())
		return
	}
	http.Redirect(w, r, "/account/keys", http.StatusSeeOther)
}

// handleWebAccountDeleteKey removes one of the current user's own keys. Like
// handleDeleteKey, it verifies the key belongs to the caller before deleting.
func (s *Server) handleWebAccountDeleteKey(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
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
	if k.UID != u.ID {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := s.db.DeleteKey(r.Context(), kid); err != nil {
		logrus.Errorf("web account: delete ssh key error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, "/account/keys", http.StatusSeeOther)
}

func (s *Server) renderAccountKeys(w http.ResponseWriter, ctx context.Context, u *database.User, inTitle, inContent, errMsg string) {
	keys, err := s.db.ListKeysByUser(ctx, u.ID)
	if err != nil {
		logrus.Errorf("web account: list ssh keys error: %v", err)
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
		Title:    "SSH keys",
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content: &webKeysData{
			Keys:         rows,
			InputTitle:   inTitle,
			InputContent: inContent,
			Error:        errMsg,
		},
	}
	s.renderer.renderPage(w, s.serverName, "account_keys", pageData)
}
