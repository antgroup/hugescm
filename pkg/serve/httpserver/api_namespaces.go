// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/antgroup/hugescm/pkg/serve/database"
)

// handleListNamespaces returns a paginated list of namespaces.
// Query params: ?type=user|group, ?owned_by= (user id)
func (s *Server) handleListNamespaces(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	_ = req // authenticated, any user can list namespaces

	page, perPage, _ := paginationParams(r)

	var nsType *int
	if t := r.URL.Query().Get("type"); len(t) != 0 {
		v := parseNamespaceType(t)
		nsType = &v
	}

	var ownerID *int64
	if o := r.URL.Query().Get("owned_by"); len(o) != 0 {
		var v int64
		if _, err := fmt.Sscanf(o, "%d", &v); err == nil {
			ownerID = &v
		}
	}

	nss, total, err := s.db.ListNamespaces(r.Context(), nsType, ownerID, page, perPage)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	if nss == nil {
		nss = []*database.Namespace{}
	}
	writePaginationHeaders(w, total, perPage)
	JsonEncode(w, nss)
}

// parseNamespaceType converts a string to the corresponding namespace type value.
func parseNamespaceType(t string) int {
	switch t {
	case "user":
		return 0
	case "group":
		return 1
	default:
		return 0
	}
}

type createNamespaceRequest struct {
	Path        string `json:"path"`
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
}

// handleCreateNamespace creates a new group namespace.
func (s *Server) handleCreateNamespace(w http.ResponseWriter, r *http.Request) {
	req, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}

	var body createNamespaceRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		renderFailureFormat(w, r, http.StatusBadRequest, "input body error: %v", err)
		return
	}
	if len(body.Path) == 0 {
		renderFailure(w, r, http.StatusBadRequest, "path is empty")
		return
	}
	if len(body.Name) == 0 {
		body.Name = body.Path
	}

	ns, err := s.db.NewGroupNamespace(r.Context(), &database.Namespace{
		Path:        body.Path,
		Name:        body.Name,
		Owner:       req.U.ID,
		Description: body.Description,
	})
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	JsonEncode(w, ns)
}
