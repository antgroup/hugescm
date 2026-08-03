// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
)

// WebRouter registers all web UI routes on a dedicated chi router.
func (s *Server) WebRouter() chi.Router {
	r := chi.NewRouter()

	// Static assets — served with ETag + long cache, before any auth middleware
	staticHandler := newStaticHandler(staticFileSystem())
	r.Handle("/static/*", http.StripPrefix("/static/", staticHandler))

	// Login/logout — public, no auth required
	r.Get("/login", s.handleWebLoginGet)
	r.Post("/login", s.handleWebLoginPost)
	r.Get("/logout", s.handleWebLogout)

	// Authenticated routes
	r.Group(func(auth chi.Router) {
		auth.Use(s.webAuthMiddleware)
		auth.Get("/", s.handleWebIndex)
		auth.Get("/account", s.handleWebAccount)
		auth.Post("/account", s.handleWebAccountEdit)
		auth.Post("/account/password", s.handleWebAccountPassword)
		auth.Get("/account/keys", s.handleWebAccountKeys)
		auth.Get("/my-repos", s.handleWebMyRepos)
		auth.Post("/account/keys", s.handleWebAccountAddKey)
		auth.Post("/account/keys/{kid:[0-9]+}/delete", s.handleWebAccountDeleteKey)
		auth.Get("/repos", s.handleWebRepos)
		auth.Get("/repos/new", s.handleWebNewRepo)
		auth.Post("/repos/new", s.handleWebNewRepo)
		auth.Get("/namespaces", s.handleWebNamespaces)
		auth.Get("/namespaces/new", s.handleWebNewNamespace)
		auth.Post("/namespaces/new", s.handleWebNewNamespace)
		auth.Get("/namespaces/delete", s.handleWebNamespaceDelete)
		auth.Post("/namespaces/delete", s.handleWebNamespaceDelete)
		auth.Get("/{namespace}", s.handleWebNamespace)
		auth.Get("/{namespace}/{repo}", s.handleWebRepo)
		auth.Get("/{namespace}/{repo}/tree", s.handleWebTree)
		auth.Get("/{namespace}/{repo}/blob/{rev}/*", s.handleWebBlob)
		auth.Get("/{namespace}/{repo}/commits", s.handleWebCommits)
		auth.Get("/{namespace}/{repo}/commit/{hash}", s.handleWebCommit)
		auth.Get("/{namespace}/{repo}/branches", s.handleWebBranches)
		auth.Get("/{namespace}/{repo}/tags", s.handleWebTags)

		// Repository settings — members + description/visibility edit.
		// Access is enforced per-handler via RepoAccessLevel (master to view,
		// owner to mutate), mirroring the API member handlers.
		auth.Get("/{namespace}/{repo}/settings", s.handleWebRepoSettings)
		auth.Post("/{namespace}/{repo}/settings", s.handleWebRepoSettingsUpdate)
		auth.Post("/{namespace}/{repo}/settings/default-branch", s.handleWebRepoSetDefaultBranch)
		auth.Post("/{namespace}/{repo}/settings/members", s.handleWebRepoAddMember)
		auth.Post("/{namespace}/{repo}/settings/members/{member_uid:[0-9]+}", s.handleWebRepoUpdateMember)
		auth.Post("/{namespace}/{repo}/settings/members/{member_uid:[0-9]+}/remove", s.handleWebRepoRemoveMember)

		// Admin user management — guarded by webAdminMiddleware (admin only).
		auth.Route("/admin/users", func(admin chi.Router) {
			admin.Use(webAdminMiddleware)
			admin.Get("/", s.handleWebAdminUsers)
			admin.Get("/new", s.handleWebAdminNewUser)
			admin.Post("/new", s.handleWebAdminNewUser)
			admin.Get("/{uid:[0-9]+}", s.handleWebAdminUserDetail)
			admin.Post("/{uid:[0-9]+}", s.handleWebAdminEditUser)
			admin.Post("/{uid:[0-9]+}/lock", s.handleWebAdminLockUser)
			admin.Post("/{uid:[0-9]+}/unlock", s.handleWebAdminUnlockUser)
			admin.Post("/{uid:[0-9]+}/promote", s.handleWebAdminPromoteUser)
			admin.Post("/{uid:[0-9]+}/demote", s.handleWebAdminDemoteUser)
			admin.Post("/{uid:[0-9]+}/delete", s.handleWebAdminDeleteUser)
			admin.Post("/{uid:[0-9]+}/password", s.handleWebAdminResetPassword)
			admin.Get("/{uid:[0-9]+}/keys", s.handleWebAdminUserKeys)
			admin.Post("/{uid:[0-9]+}/keys", s.handleWebAdminAddKeyForUser)
			admin.Post("/{uid:[0-9]+}/keys/{kid:[0-9]+}/delete", s.handleWebAdminDeleteKeyForUser)
		})
	})

	return r
}

// staticHandler serves embedded static files with ETag, 304 Not Modified,
// long-lived Cache-Control, and correct Content-Type by extension.
type staticHandler struct {
	fs      fs.FS
	fileSrv http.Handler
}

func newStaticHandler(fsys fs.FS) *staticHandler {
	return &staticHandler{
		fs:      fsys,
		fileSrv: http.FileServer(http.FS(fsys)),
	}
}

var staticContentTypes = map[string]string{
	".css":   "text/css; charset=utf-8",
	".js":    "application/javascript; charset=utf-8",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".svg":   "image/svg+xml",
	".webp":  "image/webp",
	".ico":   "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
	".eot":   "application/vnd.ms-fontobject",
	".html":  "text/html; charset=utf-8",
	".json":  "application/json; charset=utf-8",
	".map":   "application/json; charset=utf-8",
}

func contentTypeForPath(path string) string {
	lower := strings.ToLower(path)
	for ext, ct := range staticContentTypes {
		if strings.HasSuffix(lower, ext) {
			return ct
		}
	}
	return ""
}

func (h *staticHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle CORS preflight
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "If-None-Match")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Only allow GET and HEAD
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read file content (embedded FS, reads are fast)
	f, err := h.fs.Open(r.URL.Path)
	if err != nil {
		h.fileSrv.ServeHTTP(w, r) // fallback to FileServer for 404
		return
	}
	data, err := io.ReadAll(f)
	_ = f.Close()
	if err != nil {
		h.fileSrv.ServeHTTP(w, r)
		return
	}

	// Compute ETag from content hash
	sum := sha256.Sum256(data)
	etag := `"` + hex.EncodeToString(sum[:16]) + `"`

	// Set Content-Type by extension
	if ct := contentTypeForPath(r.URL.Path); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		w.Header().Set("Content-Type", http.DetectContentType(data))
	}

	// Set caching headers
	w.Header().Set("ETag", etag)
	w.Header().Set("Cache-Control", "public, max-age=86400, must-revalidate")

	// Check If-None-Match for 304
	if match := r.Header.Get("If-None-Match"); match != "" {
		if match == etag || match == `*` {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	// Write content
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	if r.Method == http.MethodHead {
		w.WriteHeader(http.StatusOK)
		return
	}
	_, _ = w.Write(data)
}

func isWebRequest(r *http.Request) bool {
	return !strings.HasPrefix(r.URL.Path, "/api/v1/")
}

var _ http.Handler = (*staticHandler)(nil)
