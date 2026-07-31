// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/antgroup/hugescm/pkg/serve"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/antgroup/hugescm/pkg/serve/repo"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

type HandlerFunc func(http.ResponseWriter, *Request)

type Server struct {
	*ServerConfig
	srv        *http.Server
	zr         chi.Router
	ar         chi.Router
	wr         chi.Router // web UI router
	db         database.DB
	hub        repo.Repositories
	serverName string
	renderer   *webRenderer
}

// ProtocolZ1Router registers the Zeta Protocol v1 (Z1) routes on a dedicated chi
// router. The router is only consulted by ServeHTTP when the incoming request
// carries the "Zeta-Protocol: z1" header, so per-route Z1/accept matchers are no
// longer needed — every route registered here is implicitly Z1-scoped. Catch-all
// `*` placeholders replace gorilla/mux's `{name:.*}` because chi's regex params
// cannot match `/`.
//
// Route map (all paths live directly under /{namespace}/{repo}/...):
//
//	POST /authorization                                  -> ShareAuthorization
//	GET  /reference/*                                    -> LsReference            (refname = *)
//	POST /reference/*                                    -> z1ReferencePostDispatch (Push or BatchCheck)
//	PUT  /reference/*                                    -> PutObject               (oid = trailing segment)
//	POST /metadata/batch                                 -> BatchMetadata
//	GET  /metadata/*                                     -> FetchMetadata           (revision = *)
//	POST /metadata/*                                     -> GetSparseMetadata       (revision = *)
//	POST /objects/batch                                  -> BatchObjects
//	POST /objects/share                                  -> ShareObjects
//	GET  /objects/{oid}                                  -> GetObject
func (s *Server) ProtocolZ1Router(r chi.Router) {
	// AUTH: shard signature auth
	r.Post("/{namespace}/{repo}/authorization", s.ShareAuthorization)

	// Zeta Protocol: FETCH APIs
	r.Get("/{namespace}/{repo}/reference/*", s.OnFunc(s.LsReference, protocol.DOWNLOAD))       // CHECKOUT: fetch reference
	r.Post("/{namespace}/{repo}/metadata/batch", s.OnFunc(s.BatchMetadata, protocol.DOWNLOAD)) // CHECKOUT: batch metadata for FUSE
	r.Get("/{namespace}/{repo}/metadata/*", s.OnFunc(s.FetchMetadata, protocol.DOWNLOAD))      // CHECKOUT: download commit and tree/subtrees metadata
	r.Post("/{namespace}/{repo}/metadata/*", s.OnFunc(s.GetSparseMetadata, protocol.DOWNLOAD)) // CHECKOUT: sparse checkout
	r.Post("/{namespace}/{repo}/objects/batch", s.OnFunc(s.BatchObjects, protocol.DOWNLOAD))   // ENHANCED: batch objects (zeta->git)
	r.Post("/{namespace}/{repo}/objects/share", s.OnFunc(s.ShareObjects, protocol.DOWNLOAD))   // CHECKOUT: shared signed oss urls
	r.Get("/{namespace}/{repo}/objects/{oid}", s.OnFunc(s.GetObject, protocol.DOWNLOAD))       // ENHANCED: download object (zeta->git)

	// Zeta Protocol: PUSH APIs
	r.Post("/{namespace}/{repo}/reference/*", s.OnFunc(s.z1ReferencePostDispatch, protocol.UPLOAD)) // PUSH: batch-check large objects OR push commit
	r.Put("/{namespace}/{repo}/reference/*", s.OnFunc(s.PutObject, protocol.UPLOAD))                // PUSH: PUT one large object
}

// z1ReferencePostDispatch routes POST /{namespace}/{repo}/reference/* to either
// BatchCheck (when the captured portion ends in "/objects/batch") or Push.
// gorilla/mux used a per-route MatcherFunc on the Accept header to disambiguate
// these two operations on overlapping paths; chi lacks that mechanism, so the
// dispatch moves into the handler where the URL suffix is the discriminator.
// Real Z1 clients hit POST .../reference/{ref} for push and POST .../reference/{ref}/objects/batch
// for batch-check, so the suffix check aligns with actual client behaviour.
func (s *Server) z1ReferencePostDispatch(w http.ResponseWriter, r *Request) {
	rest := chi.URLParam(r.Request, "*")
	if rest == "" {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, r.W("missing refname in URL %s"), r.URL.Path)
		return
	}
	if strings.HasSuffix(rest, "/objects/batch") {
		s.BatchCheck(w, r)
		return
	}
	s.Push(w, r)
}

// OpenAPIRouter registers RESTful Open API routes on an /api/v1 subrouter.
// These routes provide user management, SSH key management, repository browsing,
// namespace management, and member management capabilities.
func (s *Server) OpenAPIRouter(r chi.Router) {
	r.Route("/api/v1", func(api chi.Router) {
		// User management
		api.Get("/users", s.handleListUsers)
		api.Post("/users", s.handleCreateUser)
		api.Get("/users/{uid:[0-9]+}", s.handleGetUser)
		api.Put("/users/{uid:[0-9]+}", s.handleUpdateUser)
		api.Delete("/users/{uid:[0-9]+}", s.handleDeleteUser)
		api.Put("/users/{uid:[0-9]+}/lock", s.handleLockUser)
		api.Put("/users/{uid:[0-9]+}/unlock", s.handleUnlockUser)
		api.Get("/users/me", s.handleCurrentUserProfile)
		api.Put("/users/me/password", s.handleChangePassword)

		// SSH key management
		api.Get("/users/{uid:[0-9]+}/keys", s.handleListKeys)
		api.Post("/users/{uid:[0-9]+}/keys", s.handleCreateKey)
		api.Get("/users/{uid:[0-9]+}/keys/{kid:[0-9]+}", s.handleGetKey)
		api.Delete("/users/{uid:[0-9]+}/keys/{kid:[0-9]+}", s.handleDeleteKey)

		// Repository browsing (uses existing OnFunc/doAuth for namespace/repo-scoped routes)
		api.Get("/repos", s.handleListRepos)
		api.Get("/repos/{namespace}/{repo}", s.OnFunc(s.handleGetRepo, protocol.DOWNLOAD))
		api.Get("/repos/{namespace}/{repo}/tree", s.OnFunc(s.handleRepoTree, protocol.DOWNLOAD))
		api.Get("/repos/{namespace}/{repo}/blob/{rev}/*", s.OnFunc(s.handleRepoRaw, protocol.DOWNLOAD))
		api.Get("/repos/{namespace}/{repo}/commits", s.OnFunc(s.handleRepoCommits, protocol.DOWNLOAD))
		api.Get("/repos/{namespace}/{repo}/commits/*", s.OnFunc(s.handleRepoCommit, protocol.DOWNLOAD))
		api.Get("/repos/{namespace}/{repo}/branches", s.OnFunc(s.handleRepoBranches, protocol.DOWNLOAD))
		api.Get("/repos/{namespace}/{repo}/branches/{name}", s.OnFunc(s.handleRepoBranch, protocol.DOWNLOAD))
		api.Get("/repos/{namespace}/{repo}/tags", s.OnFunc(s.handleRepoTags, protocol.DOWNLOAD))
		api.Get("/repos/{namespace}/{repo}/tags/{name}", s.OnFunc(s.handleRepoTag, protocol.DOWNLOAD))

		// Member management
		api.Get("/repos/{namespace}/{repo}/members", s.OnFunc(s.handleListMembers, protocol.DOWNLOAD))
		api.Post("/repos/{namespace}/{repo}/members", s.OnFunc(s.handleAddMember, protocol.SUDO))
		api.Put("/repos/{namespace}/{repo}/members/{member_uid:[0-9]+}", s.OnFunc(s.handleUpdateMember, protocol.SUDO))
		api.Delete("/repos/{namespace}/{repo}/members/{member_uid:[0-9]+}", s.OnFunc(s.handleRemoveMember, protocol.SUDO))

		// Namespace management
		api.Get("/namespaces", s.handleListNamespaces)
		api.Post("/namespaces", s.handleCreateNamespace)

		// Management routes (testing/seed): POST /api/v1/user, /api/v1/key, /api/v1/repo
		api.Post("/user", s.NewUser)
		api.Post("/key", s.NewKey)
		api.Post("/repo", s.NewRepo)
	})
}

func (s *Server) initialize() error {
	// Three chi routers:
	//   zr — Zeta Protocol v1 (Z1) repo-scoped wire-protocol routes
	//   ar — RESTful Open API (/api/v1) + management routes
	//   wr — Web UI routes (HTML pages, HTMX endpoints, static assets)
	// ServeHTTP dispatches by header (Z1) then path prefix (/api/v1 -> ar, else wr).
	s.zr = chi.NewRouter()
	s.ProtocolZ1Router(s.zr)

	s.ar = chi.NewRouter()
	s.OpenAPIRouter(s.ar)

	s.renderer = newWebRenderer()
	s.wr = s.WebRouter()

	s.srv.Handler = s
	return nil
}

func NewServer(sc *ServerConfig) (*Server, error) {
	if sc.DB == nil || sc.PersistentOSS == nil {
		fmt.Fprintf(os.Stderr, "DB or OSS not configured\n")
		return nil, errors.New("missing config")
	}
	srv := &Server{
		ServerConfig: sc,
		srv: &http.Server{
			Addr:         sc.Listen,
			ReadTimeout:  sc.ReadTimeout.Duration,
			IdleTimeout:  sc.IdleTimeout.Duration,
			WriteTimeout: sc.WriteTimeout.Duration,
		},
		serverName: sc.BannerVersion,
	}
	if err := srv.initialize(); err != nil {
		return nil, err
	}
	cfg, err := sc.DB.MakeConfig()
	if err != nil {
		return nil, err
	}
	if srv.db, err = database.NewDB(cfg); err != nil {
		return nil, err
	}
	if srv.hub, err = repo.NewRepositories(sc.Repositories, sc.PersistentOSS, sc.Cache, srv.db); err != nil {
		_ = srv.db.Close()
		return nil, err
	}
	return srv, nil
}

func (s *Server) ListenAndServe() error {
	if err := serve.RegisterLanguageMatcher(); err != nil {
		logrus.Errorf("register languages matcher error: %v", err)
	}
	logrus.Infof("Listen %s", s.Listen)
	return s.srv.ListenAndServe()
}

func logResponse(hw *ResponseWriter, r *http.Request, tr *trackedReader, spent time.Duration) {
	message := r.Header.Get(ErrorMessageKey)
	switch statusCode := hw.StatusCode(); {
	default:
		logrus.Errorf("[%s] %s %s status: %d received: %d written: %d spent: %v message: %s", hw.F1RemoteAddr(), r.Method, r.RequestURI, hw.StatusCode(), tr.received, hw.Written(), spent, message)
		return
		// 200 --- 300
	case statusCode == http.StatusFound:
		logrus.Infof("[%s] %s %s status: %d received: %d written: %d spent: %v", hw.F1RemoteAddr(), r.Method, r.RequestURI, hw.StatusCode(), tr.received, hw.Written(), spent)
		return
	case statusCode >= http.StatusOK && statusCode <= http.StatusPermanentRedirect:
		if len(message) != 0 {
			logrus.Errorf("[%s] %s %s status: %d received: %d written: %d spent: %v message: %s", hw.F1RemoteAddr(), r.Method, r.RequestURI, hw.StatusCode(), tr.received, hw.Written(), spent, message)
			return
		}
		logrus.Infof("[%s] %s %s status: %d received: %d written: %d spent: %v", hw.F1RemoteAddr(), r.Method, r.RequestURI, hw.StatusCode(), tr.received, hw.Written(), spent)
		return
	case statusCode == http.StatusNotFound:
		logrus.Errorf("[%s] %s %s status: %d received: %d written: %d spent: %v message: %s", hw.F1RemoteAddr(), r.Method, r.RequestURI, hw.StatusCode(), tr.received, hw.Written(), spent, message)
		return
	case statusCode == http.StatusUnauthorized || statusCode == http.StatusBadRequest || statusCode == http.StatusForbidden:
		// default behavior
	}
	logrus.Infof("[%s] %s %s status: %d received: %d written: %d spent: %v", hw.F1RemoteAddr(), r.Method, r.RequestURI, hw.StatusCode(), tr.received, hw.Written(), spent)
}

// Z1 reports whether the request carries the Zeta-Protocol: Z1 marker, i.e.
// it was dispatched by a Zeta client speaking the v1 wire protocol. Used by
// ServeHTTP to pick the protocol router, and by handlers/middleware that want
// the same check without re-reading the header.
func Z1(r *http.Request) bool {
	return r.Header.Get(ZETA_PROTOCOL) == protocol.PROTOCOL_Z1
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// remove multiple slash and ./..
	if r.URL != nil {
		r.URL.Path = path.Clean(r.URL.Path)
	}

	w.Header().Set("Server", s.serverName)
	tr := newTrackedReader(r.Body)
	r.Body = tr
	now := time.Now()
	hw := NewResponseWriter(w, r)
	// Dispatch:
	//   1. Z1 header → protocol router (zr)
	//   2. /api/v1 prefix → OpenAPI router (ar)
	//   3. Everything else → web UI router (wr)
	if Z1(r) {
		s.zr.ServeHTTP(hw, r)
	} else if isWebRequest(r) {
		s.wr.ServeHTTP(hw, r)
	} else {
		s.ar.ServeHTTP(hw, r)
	}
	spent := time.Since(now)
	logResponse(hw, r, tr, spent)
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}
	if err := s.srv.Shutdown(ctx); err != nil {
		logrus.Errorf("shutdown ssh server %v", err)
	}
	if s.db != nil {
		_ = s.db.Close()
	}
	return nil
}

func (s *Server) open(w http.ResponseWriter, r *Request) (repo.Repository, error) {
	rr, err := s.hub.Open(r.Context(), r.R.ID, r.R.CompressionAlgo, r.R.DefaultBranch)
	if err != nil {
		s.renderError(w, r, err)
		return nil, err
	}
	return rr, nil
}
