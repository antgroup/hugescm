// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh"
)

// handleWebRepoAddDeployKey adds a deploy key (repo-scoped SSH public key)
// to the repository.  Owner-only, mirroring handleWebRepoAddMember.
// Deploy keys are type=DeployKey, have no associated user (uid=0), and
// are bound to the repo via deploy_keys_repositories.
func (s *Server) handleWebRepoAddDeployKey(w http.ResponseWriter, r *http.Request) {
	ns, repo, level, ok := s.webRepoForSettings(w, r)
	if !ok {
		return
	}
	if level < database.OwnerAccess {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	title := r.PostFormValue("title")
	content := r.PostFormValue("content")
	if title == "" || content == "" {
		http.Error(w, "title and public key are required", http.StatusBadRequest)
		return
	}
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(content))
	if err != nil {
		http.Error(w, fmt.Sprintf("bad public key: %v", err), http.StatusBadRequest)
		return
	}
	if _, err := s.db.AddDeployKey(r.Context(), &database.Key{
		Title:       title,
		Content:     content,
		Type:        database.DeployKey,
		Fingerprint: ssh.FingerprintSHA256(pk),
	}, repo.ID); err != nil {
		logrus.Errorf("web settings: add deploy key %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	s.settingsRedirect(w, r, ns, repo)
}

// handleWebRepoRemoveDeployKey removes a deploy key from a repository.
// Owner-only.  Unbinds the key from the repo (deploy_keys_repositories)
// and deletes the key record (ssh_keys), mirroring the database layer's
// two-step RemoveDeployKey.
func (s *Server) handleWebRepoRemoveDeployKey(w http.ResponseWriter, r *http.Request) {
	ns, repo, level, ok := s.webRepoForSettings(w, r)
	if !ok {
		return
	}
	if level < database.OwnerAccess {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	kid, err := strconv.ParseInt(chi.URLParam(r, "kid"), 10, 64)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := s.db.RemoveDeployKey(r.Context(), kid, repo.ID); err != nil {
		logrus.Errorf("web settings: remove deploy key %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	s.settingsRedirect(w, r, ns, repo)
}
