// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/pkg/serve/argon2id"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

// userTypeLabel renders a human-readable label for a user type.
func userTypeLabel(t database.UserType) string {
	switch t {
	case database.UserTypeBot:
		return "Bot"
	case database.UserTypeRemoteUser:
		return "Remote"
	default:
		return "Individual"
	}
}

// webUserRow is a render-safe view of a user row for the listing page.
type webUserRow struct {
	ID        int64
	UserName  string
	Name      string
	Email     string
	TypeLabel string
	Locked    bool
	Admin     bool
	CreatedAt time.Time
}

type webUserListData struct {
	Users      []webUserRow
	Total      int64
	Page       int
	PerPage    int
	TotalPages int
	FlashMsg   string
}

// handleWebAdminUsers lists all users (admin only).
func (s *Server) handleWebAdminUsers(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	page, perPage, _ := paginationParams(r)
	users, total, err := s.db.ListUsers(r.Context(), page, perPage)
	if err != nil {
		logrus.Errorf("web admin: list users error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	rows := make([]webUserRow, 0, len(users))
	for _, usr := range users {
		rows = append(rows, webUserRow{
			ID:        usr.ID,
			UserName:  usr.UserName,
			Name:      usr.Name,
			Email:     usr.Email,
			TypeLabel: userTypeLabel(usr.Type),
			Locked:    !usr.LockedAt.IsZero(),
			Admin:     usr.Administrator,
			CreatedAt: usr.CreatedAt,
		})
	}
	totalPages := 1
	if total > 0 {
		totalPages = int(total) / perPage
		if int(total)%perPage != 0 {
			totalPages++
		}
	}
	pageData := &webTemplateData{
		Title:    "Users",
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content: &webUserListData{
			Users:      rows,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			FlashMsg:   flashMessage(r.URL.Query().Get("msg")),
		},
	}
	s.renderer.renderPage(w, s.serverName, "users", pageData)
}

type webNewUserData struct {
	Error string
}

// handleWebAdminNewUser renders the create-user form (GET) or creates a user (POST).
func (s *Server) handleWebAdminNewUser(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	if r.Method == http.MethodPost {
		s.handleWebAdminNewUserPost(w, r, u)
		return
	}
	pageData := &webTemplateData{
		Title:    "New User",
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  &webNewUserData{},
	}
	s.renderer.renderPage(w, s.serverName, "new_user", pageData)
}

func (s *Server) handleWebAdminNewUserPost(w http.ResponseWriter, r *http.Request, u *database.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	username := r.PostFormValue("username")
	password := r.PostFormValue("password")
	name := r.PostFormValue("name")
	email := r.PostFormValue("email")
	typeStr := r.PostFormValue("type")
	admin := r.PostFormValue("administrator") == "on"
	if username == "" || password == "" {
		s.renderNewUserForm(w, u, "username and password are required")
		return
	}
	if name == "" {
		name = username
	}
	ut := database.UserTypeIndividual
	switch typeStr {
	case "bot":
		ut = database.UserTypeBot
	case "remote":
		ut = database.UserTypeRemoteUser
	}
	passwd, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		logrus.Errorf("web admin: hash password error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if _, err := s.db.NewUser(r.Context(), &database.User{
		UserName:       username,
		Name:           name,
		Administrator:  admin,
		Email:          email,
		Type:           ut,
		Password:       passwd,
		SignatureToken: strengthen.NewRID(),
	}); err != nil {
		s.renderNewUserForm(w, u, err.Error())
		return
	}
	http.Redirect(w, r, "/admin/users", http.StatusSeeOther)
}

func (s *Server) renderNewUserForm(w http.ResponseWriter, u *database.User, errMsg string) {
	pageData := &webTemplateData{
		Title:    "New User",
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  &webNewUserData{Error: errMsg},
	}
	s.renderer.renderPage(w, s.serverName, "new_user", pageData)
}

type webUserDetailData struct {
	UID       int64
	UserName  string
	Name      string
	Email     string
	TypeLabel string
	Locked    bool
	Admin     bool
	CreatedAt time.Time
	Error     string
}

// handleWebAdminUserDetail renders a user's detail / edit page (admin only).
func (s *Server) handleWebAdminUserDetail(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	uid, ok := parseWebUID(w, r)
	if !ok {
		return
	}
	target, err := s.db.FindUser(r.Context(), uid)
	if err != nil {
		logrus.Errorf("web admin: find user error: %v", err)
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	s.renderUserDetail(w, u, target, "")
}

// handleWebAdminEditUser updates a user's name and email (admin only).
func (s *Server) handleWebAdminEditUser(w http.ResponseWriter, r *http.Request) {
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
	target.Name = r.PostFormValue("name")
	target.Email = r.PostFormValue("email")
	// UpdateUser preserves the password hash (loaded onto target) — it is not touched here.
	if _, err := s.db.UpdateUser(r.Context(), target); err != nil {
		s.renderUserDetail(w, u, target, err.Error())
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/users/%d", uid), http.StatusSeeOther)
}

// handleWebAdminLockUser locks a user account (admin only).
func (s *Server) handleWebAdminLockUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := parseWebUID(w, r)
	if !ok {
		return
	}
	if _, err := s.db.LockUser(r.Context(), uid); err != nil {
		logrus.Errorf("web admin: lock user error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/users/%d", uid), http.StatusSeeOther)
}

// handleWebAdminUnlockUser unlocks a user account (admin only).
func (s *Server) handleWebAdminUnlockUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := parseWebUID(w, r)
	if !ok {
		return
	}
	if _, err := s.db.UnlockUser(r.Context(), uid); err != nil {
		logrus.Errorf("web admin: unlock user error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/users/%d", uid), http.StatusSeeOther)
}

// handleWebAdminPromoteUser grants administrator status (admin only). Uses the
// dedicated SetUserAdministrator statement, independent of profile edits.
func (s *Server) handleWebAdminPromoteUser(w http.ResponseWriter, r *http.Request) {
	uid, ok := parseWebUID(w, r)
	if !ok {
		return
	}
	if err := s.db.SetUserAdministrator(r.Context(), uid, true); err != nil {
		logrus.Errorf("web admin: promote user error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/users/%d", uid), http.StatusSeeOther)
}

// handleWebAdminDemoteUser revokes administrator status (admin only). An admin
// cannot demote their own account, to avoid a one-click self-lockout.
func (s *Server) handleWebAdminDemoteUser(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	uid, ok := parseWebUID(w, r)
	if !ok {
		return
	}
	if uid == u.ID {
		http.Error(w, "cannot demote your own account", http.StatusBadRequest)
		return
	}
	if err := s.db.SetUserAdministrator(r.Context(), uid, false); err != nil {
		logrus.Errorf("web admin: demote user error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/users/%d", uid), http.StatusSeeOther)
}

// handleWebAdminDeleteUser deletes a user, or locks the account if the user
// still owns repositories (admin only). An admin cannot delete their own
// account, to avoid a one-click self-lockout.
func (s *Server) handleWebAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	uid, ok := parseWebUID(w, r)
	if !ok {
		return
	}
	if uid == u.ID {
		http.Error(w, "cannot delete your own account", http.StatusBadRequest)
		return
	}
	locked, err := s.db.DeleteUser(r.Context(), uid)
	if err != nil {
		logrus.Errorf("web admin: delete user error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if locked {
		http.Redirect(w, r, "/admin/users?msg=locked", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/admin/users?msg=deleted", http.StatusSeeOther)
}

// handleWebAdminResetPassword lets an admin reset a user's password without
// the old password (unlike handleChangePassword, which verifies the old one).
func (s *Server) handleWebAdminResetPassword(w http.ResponseWriter, r *http.Request) {
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
	newPassword := r.PostFormValue("new_password")
	if newPassword == "" {
		s.renderUserDetail(w, u, target, "new password is required")
		return
	}
	passwd, err := argon2id.CreateHash(newPassword, argon2id.DefaultParams)
	if err != nil {
		logrus.Errorf("web admin: hash password error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	target.Password = passwd
	if _, err := s.db.UpdateUser(r.Context(), target); err != nil {
		s.renderUserDetail(w, u, target, err.Error())
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/admin/users/%d", uid), http.StatusSeeOther)
}

// parseWebUID extracts and validates the {uid} URL param into an int64.
// Mirrors the parsing in requireTargetUser (auth_open_api.go).
func parseWebUID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	uid, err := strconv.ParseInt(chi.URLParam(r, "uid"), 10, 64)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return 0, false
	}
	return uid, true
}

// flashMessage maps a short msg query-param code to a human-readable string.
func flashMessage(msg string) string {
	switch msg {
	case "locked":
		return "User owns repositories — account has been locked and credentials cleared."
	case "deleted":
		return "User has been permanently deleted."
	default:
		return ""
	}
}

// renderUserDetail renders the user detail page, optionally with an error
// message (e.g. a failed edit). The user fields are copied onto the view
// struct, so the password / signature token never reach the template.
func (s *Server) renderUserDetail(w http.ResponseWriter, u *database.User, target *database.User, errMsg string) {
	pageData := &webTemplateData{
		Title:    target.UserName,
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content: &webUserDetailData{
			UID:       target.ID,
			UserName:  target.UserName,
			Name:      target.Name,
			Email:     target.Email,
			TypeLabel: userTypeLabel(target.Type),
			Locked:    !target.LockedAt.IsZero(),
			Admin:     target.Administrator,
			CreatedAt: target.CreatedAt,
			Error:     errMsg,
		},
	}
	s.renderer.renderPage(w, s.serverName, "user_detail", pageData)
}
