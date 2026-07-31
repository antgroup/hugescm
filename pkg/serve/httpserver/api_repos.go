// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/mime"
	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

// handleListRepos returns a paginated list of repositories visible to the current user.
func (s *Server) handleListRepos(w http.ResponseWriter, r *http.Request) {
	_, err := s.doUserAuth(w, r)
	if err != nil {
		return
	}
	page, perPage, _ := paginationParams(r)

	// Filter by namespace path if provided
	if nsPath := r.URL.Query().Get("namespace_path"); len(nsPath) != 0 {
		ns, err := s.db.FindNamespaceByPath(r.Context(), nsPath)
		if err != nil {
			s.renderErrorRaw(w, r, err)
			return
		}
		repos, _, err := s.db.ListRepositoriesByNamespace(r.Context(), ns.ID, page, perPage)
		if err != nil {
			s.renderErrorRaw(w, r, err)
			return
		}
		if repos == nil {
			repos = []*database.Repository{}
		}
		writePaginationHeaders(w, int64(len(repos)), perPage)
		JsonEncode(w, repos)
		return
	}

	repos, total, err := s.db.ListRepositories(r.Context(), page, perPage)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	if repos == nil {
		repos = []*database.Repository{}
	}
	writePaginationHeaders(w, total, perPage)
	JsonEncode(w, repos)
}

// handleGetRepo returns repository metadata.
func (s *Server) handleGetRepo(w http.ResponseWriter, r *Request) {
	JsonEncode(w, r.R)
}

// treeEntryResponse is the JSON-serializable form of a tree entry.
type treeEntryResponse struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Hash string `json:"hash"`
	Size int64  `json:"size"`
	Mode string `json:"mode"`
}

// handleRepoTree returns the file tree at a given path and revision.
// Query params: ?rev=<branch/tag/hash>&path=<directory-path>
func (s *Server) handleRepoTree(w http.ResponseWriter, r *Request) {
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close() //nolint:errcheck

	rev := r.URL.Query().Get("rev")
	if len(rev) == 0 {
		rev = "HEAD"
	}
	ro, err := rr.ParseRev(r.Context(), rev)
	if err != nil {
		s.renderError(w, r, err)
		return
	}

	rootTree, err := ro.Target.Root(r.Context())
	if err != nil {
		s.renderError(w, r, err)
		return
	}

	dirPath := r.URL.Query().Get("path")
	var targetTree *object.Tree
	if len(dirPath) == 0 {
		targetTree = rootTree
	} else {
		targetTree, err = rootTree.Tree(r.Context(), dirPath)
		if err != nil {
			s.renderError(w, r, err)
			return
		}
	}

	entries := make([]treeEntryResponse, 0, len(targetTree.Entries))
	for _, e := range targetTree.Entries {
		entries = append(entries, treeEntryResponse{
			Name: e.Name,
			Type: e.Type().String(),
			Hash: e.Hash.String(),
			Size: e.Size,
			Mode: e.Mode.String(),
		})
	}
	JsonEncode(w, entries)
}

// handleRepoRaw returns the raw content of a file at a given path and revision.
// Path: /api/v1/repos/{namespace}/{repo}/blob/{rev}/{path:.*}
// Supports byte range requests via the Range header.
// Uses content sniffing via modules/mime for accurate MIME type detection.
func (s *Server) handleRepoRaw(w http.ResponseWriter, r *Request) {
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close() //nolint:errcheck

	rev := chi.URLParam(r.Request, "rev")
	filePath := chi.URLParam(r.Request, "*")

	ro, err := rr.ParseRev(r.Context(), rev)
	if err != nil {
		s.renderError(w, r, err)
		return
	}

	f, err := ro.Target.File(r.Context(), filePath)
	if err != nil {
		s.renderError(w, r, err)
		return
	}

	// Parse Range header for partial content support
	rg, err := protocol.ParseRangeEx(r.Request)
	if err != nil {
		renderFailure(w, r.Request, http.StatusBadRequest, err.Error())
		return
	}

	rd, _, err := f.OriginReader(r.Context())
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	defer rd.Close() //nolint:errcheck

	// Detect MIME type via content sniffing when reading from the start of the file.
	// For range requests that begin mid-file, fall back to extension-based detection.
	contentType := "application/octet-stream"
	var sniffData []byte
	statusCode := http.StatusOK
	var contentLength int64

	if rg.Start > 0 {
		// Range request starting mid-file: cannot sniff from beginning, use extension fallback
		contentType = detectContentType(filePath)
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		discarded, err := streamio.Copy(io.Discard, io.LimitReader(rd, rg.Start))
		if err != nil {
			s.renderError(w, r, err)
			return
		}
		remaining := f.Size - discarded
		if rg.Length > 0 && rg.Length < remaining {
			contentLength = rg.Length
			rd = io.NopCloser(io.LimitReader(rd, rg.Length))
		} else {
			contentLength = remaining
		}
		newRange := protocol.Range{Start: rg.Start, Length: contentLength}
		w.Header().Set("Content-Range", newRange.ContentRange(f.Size))
		statusCode = http.StatusPartialContent
	} else {
		// Full content request: sniff first bytes for accurate MIME detection
		sniffData = make([]byte, 3072)
		n, sniffErr := io.ReadFull(rd, sniffData)
		if sniffErr != nil && n == 0 {
			s.renderError(w, r, sniffErr)
			return
		}
		sniffData = sniffData[:n]

		if mt := detectMIMEType(sniffData, filePath); mt != "" {
			contentType = mt
		}

		contentLength = f.Size
		// Reconstruct reader: sniffed bytes first, then remaining stream
		rd = io.NopCloser(io.MultiReader(bytes.NewReader(sniffData), rd))
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Accept-Ranges", "bytes")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(contentLength, 10))
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(statusCode)
	if _, err := streamio.Copy(w, rd); err != nil {
		logrus.Errorf("raw file copy error: %v", err)
	}
}

// detectMIMEType uses content sniffing via modules/mime for accurate detection,
// falling back to extension-based detection for text types not recognized by the sniffer.
func detectMIMEType(data []byte, filePath string) string {
	mt := mime.Detect(data)
	if mt == nil {
		return detectContentType(filePath)
	}
	mimeStr := mt.String()
	// For text/* types, the mime package may append charset info; we keep it as-is
	if mimeStr != "application/octet-stream" {
		return mimeStr
	}
	// Content sniffing returned unknown; try extension-based fallback
	if ct := detectContentType(filePath); ct != "" {
		return ct
	}
	return mimeStr
}

// detectContentType returns a MIME type based on file extension, or empty string if unknown.
func detectContentType(path string) string {
	// Common text/code file extensions
	textExts := map[string]bool{
		".txt": true, ".md": true, ".markdown": true, ".rst": true,
		".go": true, ".rs": true, ".c": true, ".h": true, ".cpp": true, ".hpp": true,
		".py": true, ".rb": true, ".js": true, ".ts": true, ".jsx": true, ".tsx": true,
		".java": true, ".kt": true, ".swift": true, ".scala": true,
		".sh": true, ".bash": true, ".zsh": true, ".fish": true,
		".yaml": true, ".yml": true, ".toml": true, ".ini": true, ".cfg": true, ".conf": true,
		".json": true, ".xml": true, ".html": true, ".htm": true, ".css": true, ".scss": true,
		".sql": true, ".proto": true, ".lua": true, ".vim": true,
		".dockerfile": true, ".makefile": true, ".cmake": true,
		".gitignore": true, ".gitattributes": true, ".editorconfig": true,
		".env": true,
	}
	// Binary/media extensions
	extMap := map[string]string{
		".png":   "image/png",
		".jpg":   "image/jpeg",
		".jpeg":  "image/jpeg",
		".gif":   "image/gif",
		".svg":   "image/svg+xml",
		".webp":  "image/webp",
		".ico":   "image/x-icon",
		".bmp":   "image/bmp",
		".pdf":   "application/pdf",
		".zip":   "application/zip",
		".gz":    "application/gzip",
		".tar":   "application/x-tar",
		".mp4":   "video/mp4",
		".webm":  "video/webm",
		".mp3":   "audio/mpeg",
		".wav":   "audio/wav",
		".ogg":   "audio/ogg",
		".woff":  "font/woff",
		".woff2": "font/woff2",
		".ttf":   "font/ttf",
		".otf":   "font/otf",
	}

	lower := strings.ToLower(path)
	for i := len(lower) - 1; i >= 0; i-- {
		if lower[i] == '.' {
			ext := lower[i:]
			if ct, ok := extMap[ext]; ok {
				return ct
			}
			if textExts[ext] {
				return "text/plain; charset=utf-8"
			}
			break
		}
	}
	return ""
}

// commitResponse is the JSON-serializable form of a commit.
type commitResponse struct {
	Hash      string        `json:"hash"`
	Author    signatureResp `json:"author"`
	Committer signatureResp `json:"committer"`
	Message   string        `json:"message"`
	Parents   []string      `json:"parents"`
	Tree      string        `json:"tree"`
}

type signatureResp struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	When  string `json:"when"`
}

func toCommitResponse(c *object.Commit) commitResponse {
	parents := make([]string, 0, len(c.Parents))
	for _, p := range c.Parents {
		parents = append(parents, p.String())
	}
	if parents == nil {
		parents = []string{}
	}
	return commitResponse{
		Hash: c.Hash.String(),
		Author: signatureResp{
			Name:  c.Author.Name,
			Email: c.Author.Email,
			When:  c.Author.When.Format("2006-01-02T15:04:05Z"),
		},
		Committer: signatureResp{
			Name:  c.Committer.Name,
			Email: c.Committer.Email,
			When:  c.Committer.When.Format("2006-01-02T15:04:05Z"),
		},
		Message: c.Message,
		Parents: parents,
		Tree:    c.Tree.String(),
	}
}

// handleRepoCommits returns the commit history for a repository, optionally filtered by path.
// Query params: ?rev= (branch/tag/hash, default HEAD), ?path= (file/dir filter), ?page=, ?per_page=
func (s *Server) handleRepoCommits(w http.ResponseWriter, r *Request) {
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close() //nolint:errcheck

	rev := r.URL.Query().Get("rev")
	if len(rev) == 0 {
		rev = "HEAD"
	}
	ro, err := rr.ParseRev(r.Context(), rev)
	if err != nil {
		s.renderError(w, r, err)
		return
	}

	_, perPage, offset := paginationParams(r.Request)

	// Walk the commit history using pre-order (DFS, commit before parents).
	// This gives us first-parent-first walk which is what most UIs want.
	iter := object.NewCommitPreorderIter(ro.Target, nil, nil)
	defer iter.Close()

	commits := make([]commitResponse, 0, perPage)
	skipped := 0
	counted := 0

	for {
		c, err := iter.Next(r.Context())
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			s.renderError(w, r, err)
			return
		}

		// Skip commits that don't touch the specified path
		if filePath := r.URL.Query().Get("path"); len(filePath) != 0 {
			_, err := c.File(r.Context(), filePath)
			if err != nil {
				continue // file not changed in this commit, skip it
			}
		}

		if skipped < offset {
			skipped++
			continue
		}

		commits = append(commits, toCommitResponse(c))
		counted++
		if counted >= perPage {
			break
		}
	}

	if commits == nil {
		commits = []commitResponse{}
	}
	writePaginationHeaders(w, int64(len(commits)), perPage)
	JsonEncode(w, commits)
}

// handleRepoCommit returns a single commit by branch, tag, or hash.
// Ref can be a branch (including nested like feature/a/b), tag, or commit hash.
func (s *Server) handleRepoCommit(w http.ResponseWriter, r *Request) {
	rr, err := s.open(w, r)
	if err != nil {
		return
	}
	defer rr.Close() //nolint:errcheck

	ref := chi.URLParam(r.Request, "*")

	ro, err := rr.ParseRev(r.Context(), ref)
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	JsonEncode(w, toCommitResponse(ro.Target))
}

// branchResponse is the JSON-serializable form of a branch.
type branchResponse struct {
	Name            string `json:"name"`
	Hash            string `json:"hash"`
	ProtectionLevel int    `json:"protection_level"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

// handleRepoBranches returns all branches for a repository.
// Query param: ?search= (prefix match, optional)
func (s *Server) handleRepoBranches(w http.ResponseWriter, r *Request) {
	branches, err := s.db.ListBranches(r.Context(), r.R.ID)
	if err != nil {
		s.renderError(w, r, err)
		return
	}

	search := r.URL.Query().Get("search")

	resp := make([]branchResponse, 0, len(branches))
	for _, b := range branches {
		if len(search) != 0 {
			if len(b.Name) < len(search) || b.Name[:len(search)] != search {
				continue
			}
		}
		resp = append(resp, branchResponse{
			Name:            b.Name,
			Hash:            b.Hash,
			ProtectionLevel: b.ProtectionLevel,
			CreatedAt:       b.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:       b.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	if resp == nil {
		resp = []branchResponse{}
	}
	JsonEncode(w, resp)
}

// handleRepoBranch returns a single branch by name.
func (s *Server) handleRepoBranch(w http.ResponseWriter, r *Request) {
	name := chi.URLParam(r.Request, "name")

	b, err := s.db.FindBranch(r.Context(), r.R.ID, name)
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	JsonEncode(w, branchResponse{
		Name:            b.Name,
		Hash:            b.Hash,
		ProtectionLevel: b.ProtectionLevel,
		CreatedAt:       b.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:       b.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// tagResponse is the JSON-serializable form of a tag.
type tagResponse struct {
	Name        string `json:"name"`
	Hash        string `json:"hash"`
	Subject     string `json:"subject"`
	Description string `json:"description"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// handleRepoTags returns all tags for a repository.
// Query param: ?search= (prefix match, optional)
func (s *Server) handleRepoTags(w http.ResponseWriter, r *Request) {
	tags, err := s.db.ListTags(r.Context(), r.R.ID)
	if err != nil {
		s.renderError(w, r, err)
		return
	}

	search := r.URL.Query().Get("search")

	resp := make([]tagResponse, 0, len(tags))
	for _, t := range tags {
		if len(search) != 0 {
			if len(t.Name) < len(search) || t.Name[:len(search)] != search {
				continue
			}
		}
		resp = append(resp, tagResponse{
			Name:        t.Name,
			Hash:        t.Hash,
			Subject:     t.Subject,
			Description: t.Description,
			CreatedAt:   t.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:   t.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}
	if resp == nil {
		resp = []tagResponse{}
	}
	JsonEncode(w, resp)
}

// handleRepoTag returns a single tag by name.
func (s *Server) handleRepoTag(w http.ResponseWriter, r *Request) {
	name := chi.URLParam(r.Request, "name")

	t, err := s.db.FindTag(r.Context(), r.R.ID, name)
	if err != nil {
		s.renderError(w, r, err)
		return
	}
	JsonEncode(w, tagResponse{
		Name:        t.Name,
		Hash:        t.Hash,
		Subject:     t.Subject,
		Description: t.Description,
		CreatedAt:   t.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:   t.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}
