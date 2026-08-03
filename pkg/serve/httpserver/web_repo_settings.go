// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

// accessLevelLabel renders a human-readable label for a member access level.
func accessLevelLabel(l database.AccessLevel) string {
	switch l {
	case database.OwnerAccess:
		return "Owner"
	case database.MasterAccess:
		return "Maintainer"
	case database.DevAccess:
		return "Developer"
	case database.ReporterAccess:
		return "Reporter"
	default:
		return "None"
	}
}

type webMemberRow struct {
	ID          int64
	UID         int64
	UserName    string
	LevelLabel  string
	Level       int
	ExpiryLabel string
	ExpiryInput string // YYYY-MM-DD for the edit input, empty if permanent
}

type webRepoSettingsData struct {
	Namespace *database.Namespace
	Repo      *database.Repository
	Members   []webMemberRow
	Branches  []*database.Branch
	CanEdit   bool // owner+ may edit metadata and manage members
	Error     string
}

// webRepoForSettings resolves the {namespace}/{repo} path params to a repo and
// returns the current user's effective access level. It reuses webOpenRepo for
// the readable-access gating and lookup. Mirrors the access model used by the
// API member handlers (requireRepoMaster / requireRepoAdmin via RepoAccessLevel,
// whose second return value is the user's effective level).
func (s *Server) webRepoForSettings(w http.ResponseWriter, r *http.Request) (ns *database.Namespace, repo *database.Repository, level database.AccessLevel, ok bool) {
	u := webUserFromContext(r)
	ns, repo, ok = s.webOpenRepo(w, r, u)
	if !ok {
		return nil, nil, 0, false
	}
	if u.Administrator {
		return ns, repo, database.OwnerAccess, true
	}
	_, level, err := s.db.RepoAccessLevel(r.Context(), repo, u)
	if err != nil {
		logrus.Errorf("web settings: access check %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return nil, nil, 0, false
	}
	return ns, repo, level, true
}

// farFutureExpiry is the default member expiry used when the web form does not
// provide one, treating the membership as effectively permanent. The members
// column is NOT NULL, so a real future date is safer than a zero time.
var farFutureExpiry = time.Date(2199, 1, 1, 0, 0, 0, 0, time.UTC)

// handleWebRepoSettings renders the member list and repo metadata form for a
// repository. Requires master access (Sudo) or higher to view, matching
// handleListMembers.
func (s *Server) handleWebRepoSettings(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	ns, repo, level, ok := s.webRepoForSettings(w, r)
	if !ok {
		return
	}
	if !level.Sudo() {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	members, err := s.db.ListMembers(r.Context(), repo.ID, database.ProjectMember)
	if err != nil {
		logrus.Errorf("web settings: list members %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	rows := make([]webMemberRow, 0, len(members))
	for _, m := range members {
		expires := "permanent"
		expiresInput := ""
		if !m.ExpiresAt.IsZero() && m.ExpiresAt.Year() < 2199 {
			expires = m.ExpiresAt.Format("2006-01-02")
			expiresInput = m.ExpiresAt.Format("2006-01-02")
		}
		row := webMemberRow{
			ID:          m.ID,
			UID:         m.UID,
			LevelLabel:  accessLevelLabel(m.AccessLevel),
			Level:       int(m.AccessLevel),
			ExpiryLabel: expires,
			ExpiryInput: expiresInput,
		}
		if mu, ferr := s.db.FindUser(r.Context(), m.UID); ferr == nil {
			row.UserName = mu.UserName
		} else {
			row.UserName = fmt.Sprintf("uid:%d", m.UID)
		}
		rows = append(rows, row)
	}
	branches, err := s.db.ListBranches(r.Context(), repo.ID)
	if err != nil {
		logrus.Errorf("web settings: list branches %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if branches == nil {
		branches = []*database.Branch{}
	}
	pageData := &webTemplateData{
		Title:    fmt.Sprintf("%s/%s settings", ns.Path, repo.Path),
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content: &webRepoSettingsData{
			Namespace: ns,
			Repo:      repo,
			Members:   rows,
			Branches:  branches,
			CanEdit:   level >= database.OwnerAccess,
		},
	}
	s.renderer.renderPage(w, s.serverName, "repo_settings", pageData)
}

func (s *Server) settingsRedirect(w http.ResponseWriter, r *http.Request, ns *database.Namespace, repo *database.Repository) {
	http.Redirect(w, r, fmt.Sprintf("/%s/%s/settings", ns.Path, repo.Path), http.StatusSeeOther)
}

// handleWebRepoSetDefaultBranch changes the repo's default branch. Owner-only.
// Confirms the target branch actually exists (via ListBranches) before the
// dedicated UpdateRepositoryDefaultBranch update, so checkout keeps resolving
// — you can never point the default at a non-existent branch.
func (s *Server) handleWebRepoSetDefaultBranch(w http.ResponseWriter, r *http.Request) {
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
	newBranch := r.PostFormValue("default_branch")
	if newBranch == "" {
		http.Error(w, "default branch is required", http.StatusBadRequest)
		return
	}
	branches, err := s.db.ListBranches(r.Context(), repo.ID)
	if err != nil {
		logrus.Errorf("web settings: list branches %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	found := false
	for _, b := range branches {
		if b.Name == newBranch {
			found = true
			break
		}
	}
	if !found {
		http.Error(w, "branch not found: "+newBranch, http.StatusBadRequest)
		return
	}
	if _, err := s.db.UpdateRepositoryDefaultBranch(r.Context(), repo.ID, newBranch); err != nil {
		logrus.Errorf("web settings: update default branch %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	s.settingsRedirect(w, r, ns, repo)
}

// handleWebRepoSettingsUpdate edits a repo's description and visibility.
// Requires owner access, mirroring requireRepoAdmin.
func (s *Server) handleWebRepoSettingsUpdate(w http.ResponseWriter, r *http.Request) {
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
	repo.Description = r.PostFormValue("description")
	switch r.PostFormValue("visible_level") {
	case "private":
		repo.VisibleLevel = database.PrivateRepository
	case "internal":
		repo.VisibleLevel = database.InternalRepository
	case "public":
		repo.VisibleLevel = database.PublicRepository
	case "anonymous":
		repo.VisibleLevel = database.AnonymousRepository
	}
	if _, err := s.db.UpdateRepository(r.Context(), repo); err != nil {
		logrus.Errorf("web settings: update repo %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	s.settingsRedirect(w, r, ns, repo)
}

// handleWebRepoAddMember adds a member by username. Owner-only.
func (s *Server) handleWebRepoAddMember(w http.ResponseWriter, r *http.Request) {
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
	username := r.PostFormValue("username")
	if username == "" {
		http.Error(w, "username is required", http.StatusBadRequest)
		return
	}
	mu, err := s.db.SearchUser(r.Context(), username)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	al, err := strconv.Atoi(r.PostFormValue("access_level"))
	if err != nil {
		http.Error(w, "invalid access level", http.StatusBadRequest)
		return
	}
	accessLevel := database.AccessLevel(al)
	if accessLevel < database.ReporterAccess || accessLevel > database.OwnerAccess {
		http.Error(w, "invalid access level", http.StatusBadRequest)
		return
	}
	expiresAt := farFutureExpiry
	if ev := r.PostFormValue("expires_at"); ev != "" {
		t, err := time.Parse("2006-01-02", ev)
		if err != nil {
			http.Error(w, "invalid expires_at (use YYYY-MM-DD)", http.StatusBadRequest)
			return
		}
		// End of the chosen day so the member stays readable for the whole day.
		expiresAt = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	}
	if err := s.db.AddMember(r.Context(), &database.Member{
		UID:         mu.ID,
		AccessLevel: accessLevel,
		SourceID:    repo.ID,
		SourceType:  database.ProjectMember,
		ExpiresAt:   expiresAt,
	}); err != nil {
		logrus.Errorf("web settings: add member %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	s.settingsRedirect(w, r, ns, repo)
}

// handleWebRepoUpdateMember changes a member's access level. Owner-only.
func (s *Server) handleWebRepoUpdateMember(w http.ResponseWriter, r *http.Request) {
	ns, repo, level, ok := s.webRepoForSettings(w, r)
	if !ok {
		return
	}
	if level < database.OwnerAccess {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	memberUID, err := strconv.ParseInt(chi.URLParam(r, "member_uid"), 10, 64)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	al, err := strconv.Atoi(r.PostFormValue("access_level"))
	if err != nil {
		http.Error(w, "invalid access level", http.StatusBadRequest)
		return
	}
	accessLevel := database.AccessLevel(al)
	if accessLevel < database.ReporterAccess || accessLevel > database.OwnerAccess {
		http.Error(w, "invalid access level", http.StatusBadRequest)
		return
	}
	target, err := s.findRepoMember(r, repo.ID, memberUID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	target.AccessLevel = accessLevel
	// Optional expiry edit: an empty field means permanent (far future).
	if ev := r.PostFormValue("expires_at"); ev != "" {
		t, err := time.Parse("2006-01-02", ev)
		if err != nil {
			http.Error(w, "invalid expires_at (use YYYY-MM-DD)", http.StatusBadRequest)
			return
		}
		target.ExpiresAt = t.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
	} else {
		target.ExpiresAt = farFutureExpiry
	}
	if err := s.db.UpdateMember(r.Context(), target); err != nil {
		logrus.Errorf("web settings: update member %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	s.settingsRedirect(w, r, ns, repo)
}

// handleWebRepoRemoveMember removes a member. Owner-only.
func (s *Server) handleWebRepoRemoveMember(w http.ResponseWriter, r *http.Request) {
	ns, repo, level, ok := s.webRepoForSettings(w, r)
	if !ok {
		return
	}
	if level < database.OwnerAccess {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	memberUID, err := strconv.ParseInt(chi.URLParam(r, "member_uid"), 10, 64)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	target, err := s.findRepoMember(r, repo.ID, memberUID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if err := s.db.RemoveMember(r.Context(), target.ID); err != nil {
		logrus.Errorf("web settings: remove member %s/%s: %v", ns.Path, repo.Path, err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	s.settingsRedirect(w, r, ns, repo)
}

// findRepoMember lists a repo's members and returns the one matching memberUID,
// mirroring the lookup pattern in handleUpdateMember / handleRemoveMember.
func (s *Server) findRepoMember(r *http.Request, repoID int64, memberUID int64) (*database.Member, error) {
	members, err := s.db.ListMembers(r.Context(), repoID, database.ProjectMember)
	if err != nil {
		return nil, err
	}
	for _, m := range members {
		if m.UID == memberUID {
			return m, nil
		}
	}
	return nil, fmt.Errorf("member with uid %d not found", memberUID)
}
