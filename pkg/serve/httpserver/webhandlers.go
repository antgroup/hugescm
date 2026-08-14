// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/mime"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/repo"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

// webDiffOptions passes a non-nil PatchOptions to changes.Patch. getPatchContext
// dereferences opts without a nil check, so a zero literal would panic.
var webDiffOptions = &object.PatchOptions{
	Match: func(string) bool { return true },
}

// -----------------------------------------------------------------------------
// Helper: open a repo for web browsing
// -----------------------------------------------------------------------------

func (s *Server) webOpenRepo(w http.ResponseWriter, r *http.Request, u *database.User) (*database.Namespace, *database.Repository, bool) {
	nsPath := chi.URLParam(r, "namespace")
	repoPath := chi.URLParam(r, "repo")
	if nsPath == "" || repoPath == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return nil, nil, false
	}
	ns, repo, err := s.db.FindRepositoryByPath(r.Context(), nsPath, repoPath)
	if err != nil {
		http.Error(w, "repo not found", http.StatusNotFound)
		return nil, nil, false
	}
	if !u.Administrator {
		_, accessLevel, err := s.db.RepoAccessLevel(r.Context(), repo, u)
		if err != nil {
			logrus.Errorf("web: check access for %s/%s error: %v", nsPath, repoPath, err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return nil, nil, false
		}
		if !checkRepoReadable(u, repo, accessLevel) {
			http.Error(w, "access denied", http.StatusForbidden)
			return nil, nil, false
		}
	}
	return ns, repo, true
}

// webHubOpen opens the repo at the object layer (for commits, trees, files).
func (s *Server) webHubOpen(w http.ResponseWriter, r *http.Request, repo *database.Repository) (repo.Repository, bool) {
	rr, err := s.hub.Open(r.Context(), repo.ID, repo.CompressionAlgo, repo.DefaultBranch)
	if err != nil {
		http.Error(w, "repo not available", http.StatusNotFound)
		return nil, false
	}
	return rr, true
}

// webRevParam returns the "rev" query param or defaults to "HEAD".
func webRevParam(r *http.Request) string {
	if rev := r.URL.Query().Get("rev"); rev != "" {
		return rev
	}
	return "HEAD"
}

// -----------------------------------------------------------------------------
// Index — redirect to /repos or /login
// -----------------------------------------------------------------------------

func (s *Server) handleWebIndex(w http.ResponseWriter, r *http.Request) {
	if webUserFromContext(r) != nil {
		http.Redirect(w, r, "/repos", http.StatusFound)
		return
	}
	redirectToLogin(w, r)
}

// -----------------------------------------------------------------------------
// Repository list
// -----------------------------------------------------------------------------

type webRepoListData struct {
	Repos      []*database.Repository
	Namespaces map[int64]*database.Namespace
	Total      int64
	Page       int
	PerPage    int
	TotalPages int
	Query      string
	QueryEnc   string // url-encoded Query for href use
}

func (s *Server) handleWebRepos(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	page, perPage, _ := paginationParams(r)
	q := r.URL.Query().Get("q")

	var repos []*database.Repository
	var total int64
	var err error
	if q != "" {
		repos, total, err = s.db.SearchRepositories(r.Context(), q, page, perPage)
	} else {
		repos, total, err = s.db.ListRepositories(r.Context(), page, perPage)
	}
	if err != nil {
		logrus.Errorf("web: list repos error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Build namespace lookup map
	nsIDs := make(map[int64]bool)
	for _, repo := range repos {
		nsIDs[repo.NamespaceID] = true
	}
	nss := make(map[int64]*database.Namespace)
	for id := range nsIDs {
		if ns, err := s.db.FindNamespaceByID(r.Context(), id); err == nil {
			nss[id] = ns
		}
	}

	totalPages := 1
	if total > 0 {
		totalPages = int(total) / perPage
		if int(total)%perPage != 0 {
			totalPages++
		}
	}

	data := &webTemplateData{
		Title:    "Repositories",
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content: &webRepoListData{
			Repos:      repos,
			Namespaces: nss,
			Total:      total,
			Page:       page,
			PerPage:    perPage,
			TotalPages: totalPages,
			Query:      q,
			QueryEnc:   url.QueryEscape(q),
		},
	}
	s.renderer.renderPage(w, s.serverName, "repos", data)
}

// -----------------------------------------------------------------------------
// Namespace page — shows all repos under a namespace
// -----------------------------------------------------------------------------

type webNamespaceData struct {
	Namespace *database.Namespace
	Repos     []*database.Repository
}

func (s *Server) handleWebNamespace(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	nsPath := chi.URLParam(r, "namespace")
	if nsPath == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	ns, err := s.db.FindNamespaceByPath(r.Context(), nsPath)
	if err != nil {
		http.Error(w, "namespace not found", http.StatusNotFound)
		return
	}
	repos, _, err := s.db.ListRepositoriesByNamespace(r.Context(), ns.ID, 1, 100)
	if err != nil {
		logrus.Errorf("web: list repos by namespace error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if repos == nil {
		repos = []*database.Repository{}
	}
	data := &webNamespaceData{Namespace: ns, Repos: repos}
	pageData := &webTemplateData{
		Title:    ns.Path,
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  data,
	}
	s.renderer.renderPage(w, s.serverName, "namespace", pageData)
}

// -----------------------------------------------------------------------------
// New repository page — create a new repo
// -----------------------------------------------------------------------------

type webNewRepoData struct {
	Namespaces []*database.Namespace
	Error      string
}

func (s *Server) handleWebNewRepo(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)

	if r.Method == http.MethodPost {
		s.handleWebNewRepoPost(w, r, u)
		return
	}

	// GET: render the form, listing available namespaces
	nss, _, err := s.db.ListNamespaces(r.Context(), nil, &u.ID, 1, 100)
	if err != nil || nss == nil {
		// fallback: list all namespaces
		nss, _, _ = s.db.ListNamespaces(r.Context(), nil, nil, 1, 100)
	}
	if nss == nil {
		nss = []*database.Namespace{}
	}

	data := &webNewRepoData{Namespaces: nss}
	pageData := &webTemplateData{
		Title:    "New Repository",
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  data,
	}
	s.renderer.renderPage(w, s.serverName, "new_repo", pageData)
}

func (s *Server) handleWebNewRepoPost(w http.ResponseWriter, r *http.Request, u *database.User) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	nsPath := r.PostFormValue("namespace_path")
	repoPath := r.PostFormValue("repo_path")
	description := r.PostFormValue("description")
	defaultBranch := r.PostFormValue("default_branch")
	visibleLevel := r.PostFormValue("visible_level")

	if nsPath == "" || repoPath == "" {
		nss, _, _ := s.db.ListNamespaces(r.Context(), nil, &u.ID, 1, 100)
		if nss == nil {
			nss = []*database.Namespace{}
		}
		data := &webNewRepoData{
			Namespaces: nss,
			Error:      "namespace and repository path are required",
		}
		pageData := &webTemplateData{
			Title: "New Repository", Username: u.UserName, IsAdmin: u.Administrator, Content: data,
		}
		s.renderer.renderPage(w, s.serverName, "new_repo", pageData)
		return
	}

	vl := database.PrivateRepository
	switch visibleLevel {
	case "internal":
		vl = database.InternalRepository
	case "public":
		vl = database.PublicRepository
	}

	ns, err := s.db.FindNamespaceByPath(r.Context(), nsPath)
	if err != nil {
		nss, _, _ := s.db.ListNamespaces(r.Context(), nil, &u.ID, 1, 100)
		if nss == nil {
			nss = []*database.Namespace{}
		}
		data := &webNewRepoData{Namespaces: nss, Error: "namespace '" + nsPath + "' not found"}
		pageData := &webTemplateData{
			Title: "New Repository", Username: u.UserName, IsAdmin: u.Administrator, Content: data,
		}
		s.renderer.renderPage(w, s.serverName, "new_repo", pageData)
		return
	}

	if defaultBranch == "" {
		defaultBranch = database.DefaultBranch
	}

	repo, err := s.hub.New(r.Context(), &database.Repository{
		NamespaceID:   ns.ID,
		Name:          repoPath,
		Path:          repoPath,
		Description:   description,
		VisibleLevel:  vl,
		DefaultBranch: defaultBranch,
	}, u, false)
	if err != nil {
		nss, _, _ := s.db.ListNamespaces(r.Context(), nil, &u.ID, 1, 100)
		if nss == nil {
			nss = []*database.Namespace{}
		}
		data := &webNewRepoData{
			Namespaces: nss,
			Error:      err.Error(),
		}
		pageData := &webTemplateData{
			Title: "New Repository", Username: u.UserName, IsAdmin: u.Administrator, Content: data,
		}
		s.renderer.renderPage(w, s.serverName, "new_repo", pageData)
		return
	}

	http.Redirect(w, r, fmt.Sprintf("/%s/%s", nsPath, repo.Path), http.StatusSeeOther)
}

// -----------------------------------------------------------------------------
// Repository detail — info card + root tree
// -----------------------------------------------------------------------------

type webRepoDetailData struct {
	Namespace   *database.Namespace
	Repo        *database.Repository
	Rev         string
	Entries     []webTreeEntry
	PathParts   []webPathPart
	Breadcrumbs string
	CloneURL    string
	SSHCloneURL string // set when ssh_listen is configured AND endpoint has a value
	CanManage   bool   // master+ — may open the repository settings page
}

type webTreeEntry struct {
	Name string
	Type string
	Size int64
	Mode string
	Hash string
}

type webPathPart struct {
	Name string
	Path string
}

func (s *Server) handleWebRepo(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	ns, repo, ok := s.webOpenRepo(w, r, u)
	if !ok {
		return
	}
	rev := webRevParam(r)
	path := r.URL.Query().Get("path")

	// Fetch initial tree entries
	entries := []webTreeEntry{}
	if rr, ok := s.webHubOpen(w, r, repo); ok {
		ro, err := rr.ParseRev(r.Context(), rev)
		if err == nil {
			rootTree, err := ro.Target.Root(r.Context())
			if err == nil {
				var t *object.Tree
				if path == "" {
					t = rootTree
				} else {
					t, err = rootTree.Tree(r.Context(), path)
				}
				if err == nil && t != nil {
					for _, e := range t.Entries {
						entries = append(entries, webTreeEntry{
							Name: e.Name,
							Type: e.Type().String(),
							Size: e.Size,
							Mode: e.Mode.String(),
							Hash: e.Hash.String(),
						})
					}
				}
			}
		}
		_ = rr.Close()
	}

	cloneURL := fmt.Sprintf("%s://%s/%s/%s", resolveScheme(r), r.Host, ns.Path, repo.Path)

	// Build a SSH remote URL (`zeta@{endpoint}:{namespace}/{repo}`) when
	// the operator has configured ssh_listen AND `endpoint` is set in the
	// shared config.  Without `endpoint` we can't form a client-facing
	// remote URL, so we leave it empty; the template only renders the
	// SSH row when this is non-empty.
	var sshCloneURL string
	if s.SSHListen != "" && s.Endpoint != "" {
		sshCloneURL = fmt.Sprintf("zeta@%s:%s/%s", s.Endpoint, ns.Path, repo.Path)
	}

	canManage := u.Administrator
	if !canManage {
		if _, lvl, err := s.db.RepoAccessLevel(r.Context(), repo, u); err == nil && lvl.Sudo() {
			canManage = true
		}
	}

	data := &webRepoDetailData{
		Namespace:   ns,
		Repo:        repo,
		Rev:         rev,
		Entries:     entries,
		PathParts:   buildPathParts(path),
		Breadcrumbs: path,
		CloneURL:    cloneURL,
		SSHCloneURL: sshCloneURL,
		CanManage:   canManage,
	}
	pageData := &webTemplateData{
		Title:    fmt.Sprintf("%s/%s", ns.Path, repo.Path),
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  data,
	}
	s.renderer.renderPage(w, s.serverName, "repo_detail", pageData)
}

// -----------------------------------------------------------------------------
// Tree partial (HTMX) — returns tree entries as an HTML fragment
// -----------------------------------------------------------------------------

func (s *Server) handleWebTree(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	ns, repo, ok := s.webOpenRepo(w, r, u)
	if !ok {
		return
	}
	rev := webRevParam(r)
	path := r.URL.Query().Get("path")

	entries := []webTreeEntry{}
	if rr, ok := s.webHubOpen(w, r, repo); ok {
		defer rr.Close() //nolint:errcheck
		ro, err := rr.ParseRev(r.Context(), rev)
		if err == nil {
			rootTree, err := ro.Target.Root(r.Context())
			if err == nil {
				var t *object.Tree
				if path == "" {
					t = rootTree
				} else {
					t, err = rootTree.Tree(r.Context(), path)
				}
				if err == nil && t != nil {
					for _, e := range t.Entries {
						entries = append(entries, webTreeEntry{
							Name: e.Name,
							Type: e.Type().String(),
							Size: e.Size,
							Mode: e.Mode.String(),
							Hash: e.Hash.String(),
						})
					}
				}
			}
		}
	}

	data := &webRepoDetailData{
		Namespace: ns,
		Repo:      repo,
		Rev:       rev,
		Entries:   entries,
		PathParts: buildPathParts(path),
	}
	pageData := &webTemplateData{
		Username: u.UserName,
		Content:  data,
	}
	s.renderer.renderPartial(w, "tree_entries", pageData)
}

// buildPathParts splits a path string into breadcrumb segments.
func buildPathParts(path string) []webPathPart {
	if path == "" {
		return nil
	}
	parts := strings.Split(path, "/")
	result := make([]webPathPart, 0, len(parts))
	var acc strings.Builder
	for i, p := range parts {
		if p == "" {
			continue
		}
		if i > 0 {
			acc.WriteString("/")
		}
		acc.WriteString(p)
		result = append(result, webPathPart{Name: p, Path: acc.String()})
	}
	return result
}

// -----------------------------------------------------------------------------
// Blob/file viewer — shows file content with syntax highlighting
// -----------------------------------------------------------------------------

type webBlobData struct {
	Namespace *database.Namespace
	Repo      *database.Repository
	Rev       string
	FilePath  string
	FileName  string
	PathParts []webPathPart
	IsBinary  bool
	IsImage   bool
	Content   string
	Size      int64
	Hash      string
}

const webMaxBlobPreview = 512 * 1024 // 512KB preview limit

func (s *Server) handleWebBlob(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	ns, repo, ok := s.webOpenRepo(w, r, u)
	if !ok {
		return
	}

	rev := chi.URLParam(r, "rev")
	filePath := chi.URLParam(r, "*")
	if rev == "" || filePath == "" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	filePath = strings.TrimPrefix(filePath, "/")

	rr, ok := s.webHubOpen(w, r, repo)
	if !ok {
		return
	}
	defer rr.Close() //nolint:errcheck

	ro, err := rr.ParseRev(r.Context(), rev)
	if err != nil {
		http.Error(w, "revision not found", http.StatusNotFound)
		return
	}

	f, err := ro.Target.File(r.Context(), filePath)
	if err != nil {
		http.Error(w, "file not found", http.StatusNotFound)
		return
	}

	rd, _, err := f.OriginReader(r.Context())
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}
	defer rd.Close() //nolint:errcheck

	// Sniff the first bytes to detect MIME type
	sniffBytes, err := io.ReadAll(io.LimitReader(rd, 3072))
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	// Detect MIME type by sniffing file content
	detected := mime.DetectAny(sniffBytes)
	isImage := isImageMIME(detected)
	isText := isTextMIME(detected)
	isBinary := !isText && !isImage

	fileName := getFileName(filePath)

	// Directory path (without the file name) for breadcrumb navigation
	dirPath := ""
	if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
		dirPath = filePath[:idx]
	}

	// For binary non-image files, show download view
	if isBinary {
		data := &webBlobData{
			Namespace: ns,
			Repo:      repo,
			Rev:       rev,
			FilePath:  filePath,
			FileName:  fileName,
			PathParts: buildPathParts(dirPath),
			IsBinary:  true,
			IsImage:   false,
			Size:      f.Size,
			Hash:      f.Hash.String(),
		}
		pageData := &webTemplateData{
			Title:    fmt.Sprintf("%s/%s - %s", ns.Path, repo.Path, filePath),
			Username: u.UserName,
			IsAdmin:  u.Administrator,
			Content:  data,
		}
		s.renderer.renderPage(w, s.serverName, "blob", pageData)
		return
	}

	// For images, use the raw API URL — no need to load content into memory
	if isImage {
		// Images use the raw API URL, no need to load content
		data := &webBlobData{
			Namespace: ns,
			Repo:      repo,
			Rev:       rev,
			FilePath:  filePath,
			FileName:  fileName,
			PathParts: buildPathParts(dirPath),
			IsBinary:  false,
			IsImage:   true,
			Size:      f.Size,
			Hash:      f.Hash.String(),
		}
		pageData := &webTemplateData{
			Title:    fmt.Sprintf("%s/%s - %s", ns.Path, repo.Path, filePath),
			Username: u.UserName,
			IsAdmin:  u.Administrator,
			Content:  data,
		}
		s.renderer.renderPage(w, s.serverName, "blob", pageData)
		return
	}

	// Text file: read content for rendering
	content, err := io.ReadAll(io.MultiReader(bytes.NewReader(sniffBytes), io.LimitReader(rd, int64(webMaxBlobPreview+1)-int64(len(sniffBytes)))))
	if err != nil {
		http.Error(w, "read error", http.StatusInternalServerError)
		return
	}

	preview := string(content)
	if len(content) > webMaxBlobPreview {
		preview = string(content[:webMaxBlobPreview]) + "\n\n... file truncated (showing first 512KB)"
	}

	data := &webBlobData{
		Namespace: ns,
		Repo:      repo,
		Rev:       rev,
		FilePath:  filePath,
		FileName:  fileName,
		PathParts: buildPathParts(dirPath),
		IsBinary:  false,
		IsImage:   false,
		Content:   preview,
		Size:      f.Size,
		Hash:      f.Hash.String(),
	}

	pageData := &webTemplateData{
		Title:    fmt.Sprintf("%s/%s - %s", ns.Path, repo.Path, filePath),
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  data,
	}
	s.renderer.renderPage(w, s.serverName, "blob", pageData)
}

func getFileName(filePath string) string {
	if idx := strings.LastIndex(filePath, "/"); idx >= 0 {
		return filePath[idx+1:]
	}
	return filePath
}

// isTextMIME walks the MIME parent chain to check if the content is text/plain.
// This mirrors the approach in modules/zeta/backend/file_storer.go:isBinaryPayload.
func isTextMIME(m *mime.MIME) bool {
	for p := m; p != nil; p = p.Parent() {
		if p.Is("text/plain") {
			return true
		}
	}
	return m != nil && (m.Is("application/json") || m.Is("application/xml") ||
		m.Is("application/javascript") || m.Is("application/x-yaml"))
}

// isImageMIME walks the MIME parent chain to check if the content is an image.
func isImageMIME(m *mime.MIME) bool {
	for p := m; p != nil; p = p.Parent() {
		if p.Is("image/png") || p.Is("image/jpeg") || p.Is("image/gif") ||
			p.Is("image/svg+xml") || p.Is("image/webp") || p.Is("image/bmp") ||
			p.Is("image/x-icon") || p.Is("image/vnd.microsoft.icon") {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Commit history
// -----------------------------------------------------------------------------

type webCommitListData struct {
	Namespace *database.Namespace
	Repo      *database.Repository
	Rev       string
	Commits   []webCommitInfo
	Page      int
	PerPage   int
	HasNext   bool
}

type webCommitInfo struct {
	Hash    string
	Message string
	Author  string
	Email   string
	When    string
	Parents []string
}

const webCommitPerPage = 30

func (s *Server) handleWebCommits(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	ns, repo, ok := s.webOpenRepo(w, r, u)
	if !ok {
		return
	}

	rev := webRevParam(r)
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page <= 0 {
		page = 1
	}

	commits := make([]webCommitInfo, 0, webCommitPerPage)
	skip := (page - 1) * webCommitPerPage
	skipped := 0
	hasNext := false

	if rr, ok := s.webHubOpen(w, r, repo); ok {
		defer rr.Close() //nolint:errcheck
		ro, err := rr.ParseRev(r.Context(), rev)
		if err != nil {
			http.Error(w, "revision not found", http.StatusNotFound)
			return
		}
		iter := object.NewCommitPreorderIter(ro.Target, nil, nil)
		defer iter.Close()
		for {
			c, err := iter.Next(r.Context())
			if errors.Is(err, io.EOF) {
				break
			}
			if err != nil {
				logrus.Errorf("web: iterate commits error: %v", err)
				break
			}
			if skipped < skip {
				skipped++
				continue
			}
			if len(commits) >= webCommitPerPage {
				hasNext = true
				break
			}
			commits = append(commits, webCommitInfo{
				Hash:    c.Hash.String(),
				Message: firstLine(c.Message),
				Author:  c.Author.Name,
				Email:   c.Author.Email,
				When:    c.Author.When.Format("2006-01-02 15:04:05"),
				Parents: parentHashes(c.Parents),
			})
		}
	}

	data := &webCommitListData{
		Namespace: ns,
		Repo:      repo,
		Rev:       rev,
		Commits:   commits,
		Page:      page,
		PerPage:   webCommitPerPage,
		HasNext:   hasNext,
	}
	pageData := &webTemplateData{
		Title:    fmt.Sprintf("%s/%s - Commits", ns.Path, repo.Path),
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  data,
	}
	s.renderer.renderPage(w, s.serverName, "commits", pageData)
}

func parentHashes(hashes []plumbing.Hash) []string {
	if len(hashes) == 0 {
		return nil
	}
	result := make([]string, 0, len(hashes))
	for _, h := range hashes {
		result = append(result, h.String())
	}
	return result
}

// -----------------------------------------------------------------------------
// Commit detail — shows a single commit with diff
// -----------------------------------------------------------------------------

type webCommitDetailData struct {
	Namespace  *database.Namespace
	Repo       *database.Repository
	Commit     webCommitInfo
	CommitFull string
	Diff       []webDiffFile
}

type webDiffFile struct {
	OldPath   string
	NewPath   string
	IsBinary  bool
	Additions int
	Deletions int
	PatchText string
}

func (s *Server) handleWebCommit(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	ns, repo, ok := s.webOpenRepo(w, r, u)
	if !ok {
		return
	}

	hash := chi.URLParam(r, "hash")
	if hash == "" {
		http.Error(w, "commit hash required", http.StatusBadRequest)
		return
	}

	rr, ok := s.webHubOpen(w, r, repo)
	if !ok {
		return
	}
	defer rr.Close() //nolint:errcheck

	ro, err := rr.ParseRev(r.Context(), hash)
	if err != nil {
		http.Error(w, "commit not found", http.StatusNotFound)
		return
	}
	c := ro.Target

	// Compute diff against first parent
	diffFiles := []webDiffFile{}
	if len(c.Parents) > 0 {
		parentRO, perr := rr.ParseRev(r.Context(), c.Parents[0].String())
		if perr == nil && parentRO.Target != nil {
			computeDiffFiles(r.Context(), c, parentRO.Target, &diffFiles)
		}
	}

	data := &webCommitDetailData{
		Namespace: ns,
		Repo:      repo,
		Commit: webCommitInfo{
			Hash:    c.Hash.String(),
			Message: firstLine(c.Message),
			Author:  c.Author.Name,
			Email:   c.Author.Email,
			When:    c.Author.When.Format("2006-01-02 15:04:05"),
			Parents: parentHashes(c.Parents),
		},
		CommitFull: c.Message,
		Diff:       diffFiles,
	}

	pageData := &webTemplateData{
		Title:    fmt.Sprintf("%s/%s - %s", ns.Path, repo.Path, shortHash(hash)),
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  data,
	}
	s.renderer.renderPage(w, s.serverName, "commit_detail", pageData)
}

// computeDiffFiles populates out with per-file diff info between current and parent.
func computeDiffFiles(ctx context.Context, current, parent *object.Commit, out *[]webDiffFile) {
	thisTree, err := current.Root(ctx)
	if err != nil {
		return
	}
	parentTree, err := parent.Root(ctx)
	if err != nil {
		return
	}
	// Diff: old (parent) -> new (current)
	changes, err := parentTree.DiffContext(ctx, thisTree, nil)
	if err != nil {
		return
	}
	patches, err := changes.Patch(ctx, webDiffOptions)
	if err != nil {
		return
	}
	for _, p := range patches {
		df := webDiffFile{
			IsBinary: p.IsBinary,
			NewPath:  p.Name(),
		}
		if p.From != nil && p.From.Name != "" {
			df.OldPath = p.From.Name
		}
		if p.To != nil && p.To.Name != "" {
			df.NewPath = p.To.Name
		}
		for _, hunk := range p.Hunks {
			for _, line := range hunk.Lines {
				switch line.Kind {
				case diferenco.Insert:
					df.Additions++
				case diferenco.Delete:
					df.Deletions++
				}
			}
		}
		patchBytes, _ := p.Format()
		df.PatchText = string(patchBytes)
		*out = append(*out, df)
	}
}

// -----------------------------------------------------------------------------
// Branch list
// -----------------------------------------------------------------------------

type webBranchListData struct {
	Namespace *database.Namespace
	Repo      *database.Repository
	Branches  []*database.Branch
}

func (s *Server) handleWebBranches(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	ns, repo, ok := s.webOpenRepo(w, r, u)
	if !ok {
		return
	}
	branches, err := s.db.ListBranches(r.Context(), repo.ID)
	if err != nil {
		logrus.Errorf("web: list branches error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if branches == nil {
		branches = []*database.Branch{}
	}
	data := &webBranchListData{Namespace: ns, Repo: repo, Branches: branches}
	pageData := &webTemplateData{
		Title:    fmt.Sprintf("%s/%s - Branches", ns.Path, repo.Path),
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  data,
	}
	s.renderer.renderPage(w, s.serverName, "branches", pageData)
}

// -----------------------------------------------------------------------------
// Tag list
// -----------------------------------------------------------------------------

type webTagListData struct {
	Namespace *database.Namespace
	Repo      *database.Repository
	Tags      []*database.Tag
}

func (s *Server) handleWebTags(w http.ResponseWriter, r *http.Request) {
	u := webUserFromContext(r)
	ns, repo, ok := s.webOpenRepo(w, r, u)
	if !ok {
		return
	}
	tags, err := s.db.ListTags(r.Context(), repo.ID)
	if err != nil {
		logrus.Errorf("web: list tags error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if tags == nil {
		tags = []*database.Tag{}
	}
	data := &webTagListData{Namespace: ns, Repo: repo, Tags: tags}
	pageData := &webTemplateData{
		Title:    fmt.Sprintf("%s/%s - Tags", ns.Path, repo.Path),
		Username: u.UserName,
		IsAdmin:  u.Administrator,
		Content:  data,
	}
	s.renderer.renderPage(w, s.serverName, "tags", pageData)
}
