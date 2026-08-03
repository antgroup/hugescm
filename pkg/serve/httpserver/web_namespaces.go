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
	"github.com/sirupsen/logrus"
)

type webNewNamespaceData struct {
	InputPath string
	InputDesc string
	Error     string
}

type webNamespaceRow struct {
	ID          int64
	Path        string
	Description string
	CanDelete   bool
	CreatedAt   time.Time
}

type webNamespacesData struct {
	Namespaces []webNamespaceRow
	Total      int64
	Page       int
	PerPage    int
	TotalPages int
}

// handleWebNamespaces lists group namespaces (type=1) with pagination. The
// namespace lookup is by-name-treating-as-path, and new group namespaces store
// name==path, so each card links to /{path}.
func (s *Server) handleWebNamespaces(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	page, perPage, _ := paginationParams(r)
	groupType := 1 // GroupNamespace, matching NewGroupNamespace / parseNamespaceType("group")
	nss, total, err := s.db.ListNamespaces(r.Context(), &groupType, nil, page, perPage)
	if err != nil {
		logrus.Errorf("web: list namespaces error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	rows := make([]webNamespaceRow, 0, len(nss))
	for _, ns := range nss {
		rows = append(rows, webNamespaceRow{
			ID:          ns.ID,
			Path:        ns.Path,
			Description: ns.Description,
			CanDelete:   u.Administrator || ns.Owner == u.ID,
			CreatedAt:   ns.CreatedAt,
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
		Title:    "Namespaces",
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content: &webNamespacesData{
			Namespaces: rows,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
		},
	}
	s.renderer.renderPage(w, s.serverName, "namespaces", pageData)
}

// handleWebNewNamespace renders the create-group-namespace form (GET) or creates
// one (POST). Mirrors handleCreateNamespace (api_namespaces.go) + NewGroupNamespace,
// which marks the row type=1 (group) and sets owner to the current user. The
// name is set to path to match the codebase's namespace lookup-by-name convention
// (FindNamespaceByPath), so the new namespace is reachable at /{path} after.
func (s *Server) handleWebNewNamespace(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	if r.Method == http.MethodPost {
		s.handleWebNewNamespacePost(w, r, u)
		return
	}
	pageData := &webTemplateData{
		Title:    "New Namespace",
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  &webNewNamespaceData{},
	}
	s.renderer.renderPage(w, s.serverName, "new_namespace", pageData)
}

func (s *Server) handleWebNewNamespacePost(w http.ResponseWriter, r *http.Request, u *database.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	path := r.PostFormValue("path")
	description := r.PostFormValue("description")
	input := &webNewNamespaceData{InputPath: path, InputDesc: description}
	if path == "" {
		input.Error = "namespace path is required"
		s.renderNewNamespace(w, u, input)
		return
	}
	ns, err := s.db.NewGroupNamespace(r.Context(), &database.Namespace{
		Path:        path,
		Name:        path,
		Owner:       u.ID,
		Description: description,
	})
	if err != nil {
		logrus.Errorf("web: create namespace error: %v", err)
		input.Error = err.Error()
		s.renderNewNamespace(w, u, input)
		return
	}
	http.Redirect(w, r, "/"+ns.Path, http.StatusSeeOther)
}

func (s *Server) renderNewNamespace(w http.ResponseWriter, u *database.User, data *webNewNamespaceData) {
	pageData := &webTemplateData{
		Title:    "New Namespace",
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  data,
	}
	s.renderer.renderPage(w, s.serverName, "new_namespace", pageData)
}

type webNamespaceDeleteData struct {
	Namespace    *database.Namespace
	RepoCount    int64
	Destinations []*database.Namespace
	Error        string
}

// handleWebNamespaceDelete renders the delete-confirm page (GET) or performs a
// delete with optional transfer (POST). Only group namespaces can be deleted,
// and only by their owner or an admin. The user must either empty the namespace
// first (no repos) or pick another GROUP namespace to transfer its repos to.
func (s *Server) handleWebNamespaceDelete(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	if r.Method == http.MethodPost {
		s.handleWebNamespaceDeletePost(w, r, u)
		return
	}
	id, err := strconv.ParseInt(r.URL.Query().Get("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ns, err := s.db.FindNamespaceByID(r.Context(), id)
	if err != nil {
		http.Error(w, "namespace not found", http.StatusNotFound)
		return
	}
	if !canDeleteNamespace(u, ns) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if ns.Type != 1 { // group only (DB convention: 0=user, 1=group)
		s.renderNamespaceDelete(w, r.Context(), u, ns, 0, "only group namespaces can be deleted")
		return
	}
	_, total, err := s.db.ListRepositoriesByNamespace(r.Context(), ns.ID, 1, 1)
	if err != nil {
		logrus.Errorf("web ns delete: count repos %s: %v", ns.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	s.renderNamespaceDelete(w, r.Context(), u, ns, total, "")
}

func (s *Server) handleWebNamespaceDeletePost(w http.ResponseWriter, r *http.Request, u *database.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	id, err := strconv.ParseInt(r.PostFormValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ns, err := s.db.FindNamespaceByID(r.Context(), id)
	if err != nil {
		http.Error(w, "namespace not found", http.StatusNotFound)
		return
	}
	if !canDeleteNamespace(u, ns) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if ns.Type != 1 {
		http.Error(w, "only group namespaces can be deleted", http.StatusBadRequest)
		return
	}
	_, total, err := s.db.ListRepositoriesByNamespace(r.Context(), ns.ID, 1, 1)
	if err != nil {
		logrus.Errorf("web ns delete: count repos %s: %v", ns.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	var dstID int64
	if total > 0 {
		transferTo := r.PostFormValue("transfer_to")
		if transferTo == "" {
			s.renderNamespaceDelete(w, r.Context(), u, ns, total, "this namespace still has repositories — choose a destination to transfer them, or empty it first")
			return
		}
		dst, err := s.db.FindNamespaceByPath(r.Context(), transferTo)
		if err != nil {
			s.renderNamespaceDelete(w, r.Context(), u, ns, total, "destination namespace not found")
			return
		}
		if dst.ID == ns.ID {
			s.renderNamespaceDelete(w, r.Context(), u, ns, total, "destination must be a different namespace")
			return
		}
		if dst.Type != 1 {
			s.renderNamespaceDelete(w, r.Context(), u, ns, total, "destination must be a group namespace")
			return
		}
		dstID = dst.ID
	}
	if _, err := s.db.DeleteNamespaceWithTransfer(r.Context(), ns.ID, dstID); err != nil {
		logrus.Errorf("web ns delete: transfer/delete %s: %v", ns.Path, err)
		s.renderNamespaceDelete(w, r.Context(), u, ns, total, "transfer failed (the destination likely already has a repository with a clashing path): "+err.Error())
		return
	}
	http.Redirect(w, r, "/namespaces", http.StatusSeeOther)
}

// canDeleteNamespace: owner or admin only.
func canDeleteNamespace(u *database.User, ns *database.Namespace) bool {
	return u != nil && (u.Administrator || ns.Owner == u.ID)
}

// destinationsFor lists group namespaces (type=1) other than excludeID, for the
// transfer-to dropdown on the delete page.
func (s *Server) destinationsFor(ctx context.Context, excludeID int64) []*database.Namespace {
	groupType := 1
	nss, _, err := s.db.ListNamespaces(ctx, &groupType, nil, 1, 100)
	if err != nil {
		return []*database.Namespace{}
	}
	out := make([]*database.Namespace, 0, len(nss))
	for _, d := range nss {
		if d.ID != excludeID {
			out = append(out, d)
		}
	}
	return out
}

func (s *Server) renderNamespaceDelete(w http.ResponseWriter, ctx context.Context, u *database.User, ns *database.Namespace, repoCount int64, errMsg string) {
	var dests []*database.Namespace
	if repoCount > 0 {
		dests = s.destinationsFor(ctx, ns.ID)
	}
	pageData := &webTemplateData{
		Title:    fmt.Sprintf("Delete %s", ns.Path),
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content: &webNamespaceDeleteData{
			Namespace:    ns,
			RepoCount:    repoCount,
			Destinations: dests,
			Error:        errMsg,
		},
	}
	s.renderer.renderPage(w, s.serverName, "namespace_delete", pageData)
}
