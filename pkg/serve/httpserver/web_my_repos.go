// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"net/http"

	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/sirupsen/logrus"
)

type webMyReposData struct {
	NamespacePath string
	Repos         []*database.Repository
	Total         int64
	Page          int
	PerPage       int
	TotalPages    int
}

// handleWebMyRepos lists repositories under the current user's personal
// namespace (the one created at signup by NewUser, whose name/path == username).
// Let a user see just their own repos instead of the global list. If the user
// somehow has no personal namespace yet, renders an empty listing.
func (s *Server) handleWebMyRepos(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	page, perPage, _ := paginationParams(r)
	ns, err := s.db.FindNamespaceByPath(r.Context(), u.UserName)
	var repos []*database.Repository
	var total int64
	if err == nil {
		repos, total, err = s.db.ListRepositoriesByNamespace(r.Context(), ns.ID, page, perPage)
		if err != nil {
			logrus.Errorf("web: my repos list error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
	} else {
		logrus.Debugf("web: personal namespace for %s not found: %v", u.UserName, err)
	}
	if repos == nil {
		repos = []*database.Repository{}
	}
	nsPath := u.UserName
	if ns != nil {
		nsPath = ns.Path
	}
	totalPages := 1
	if total > 0 {
		totalPages = int(total) / perPage
		if int(total)%perPage != 0 {
			totalPages++
		}
	}
	pageData := &webTemplateData{
		Title:    "My repositories",
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content: &webMyReposData{
			NamespacePath: nsPath,
			Repos:         repos,
			Total:         total,
			Page:          page,
			PerPage:       perPage,
			TotalPages:    totalPages,
		},
	}
	s.renderer.renderPage(w, s.serverName, "my_repos", pageData)
}
