// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"net/http"
	"time"

	"github.com/antgroup/hugescm/pkg/serve/argon2id"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/sirupsen/logrus"
)

type webAccountData struct {
	UID       int64
	UserName  string
	Name      string
	Email     string
	TypeLabel string
	Admin     bool
	CreatedAt time.Time
	Error     string
}

// handleWebAccount renders the current user's profile page (profile form +
// change-password form). Available to every authenticated user, not just
// admins — the admin user_detail page is for managing other users.
func (s *Server) handleWebAccount(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	me, err := s.db.FindUser(r.Context(), u.ID)
	if err != nil {
		logrus.Errorf("web account: find user error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	s.renderAccount(w, me, "")
}

// handleWebAccountEdit updates the current user's own name and email.
func (s *Server) handleWebAccountEdit(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	me, err := s.db.FindUser(r.Context(), u.ID)
	if err != nil {
		logrus.Errorf("web account: find user error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	me.Name = r.PostFormValue("name")
	me.Email = r.PostFormValue("email")
	// UpdateUser preserves the password hash (loaded onto me) — it is untouched here.
	if _, err := s.db.UpdateUser(r.Context(), me); err != nil {
		s.renderAccount(w, me, err.Error())
		return
	}
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}

// handleWebAccountPassword changes the current user's own password. Unlike the
// admin reset (handleWebAdminResetPassword), this verifies the old password
// first — mirroring handleChangePassword (api_users.go).
func (s *Server) handleWebAccountPassword(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	me, err := s.db.FindUser(r.Context(), u.ID)
	if err != nil {
		logrus.Errorf("web account: find user error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	oldPassword := r.PostFormValue("old_password")
	newPassword := r.PostFormValue("new_password")
	if oldPassword == "" || newPassword == "" {
		s.renderAccount(w, me, "current and new passwords are required")
		return
	}
	ok, err := argon2id.ComparePasswordAndHash(oldPassword, me.Password)
	if err != nil {
		logrus.Errorf("web account: verify old password error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !ok {
		s.renderAccount(w, me, "current password is incorrect")
		return
	}
	passwd, err := argon2id.CreateHash(newPassword, argon2id.DefaultParams)
	if err != nil {
		logrus.Errorf("web account: hash password error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	me.Password = passwd
	if _, err := s.db.UpdateUser(r.Context(), me); err != nil {
		s.renderAccount(w, me, err.Error())
		return
	}
	http.Redirect(w, r, "/account", http.StatusSeeOther)
}

// renderAccount renders the account page. Only safe fields are copied onto the
// view struct, so the password / signature token never reach the template.
func (s *Server) renderAccount(w http.ResponseWriter, me *database.User, errMsg string) {
	pageData := &webTemplateData{
		Title:    "Account",
		Username: me.UserName,
		IsAdmin:  me.Administrator,
		Content: &webAccountData{
			UID:       me.ID,
			UserName:  me.UserName,
			Name:      me.Name,
			Email:     me.Email,
			TypeLabel: userTypeLabel(me.Type),
			Admin:     me.Administrator,
			CreatedAt: me.CreatedAt,
			Error:     errMsg,
		},
	}
	s.renderer.renderPage(w, s.serverName, "account", pageData)
}
