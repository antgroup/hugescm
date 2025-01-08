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
	"time"

	"github.com/antgroup/hugescm/pkg/serve"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/antgroup/hugescm/pkg/serve/repo"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type HandlerFunc func(http.ResponseWriter, *Request)

type Server struct {
	*ServerConfig
	srv        *http.Server
	r          *mux.Router
	db         database.DB
	hub        repo.Repositories
	serverName string
}

func Z1Matcher(r *http.Request, m *mux.RouteMatch) bool {
	return r.Header.Get(ZETA_PROTOCOL) == protocol.PROTOCOL_Z1
}

func (s *Server) ProtocolZ1Router(r *mux.Router) {
	r.HandleFunc("/{namespace}/{repo}/authorization", s.ShareAuthorization).Methods("POST").MatcherFunc(Z1Matcher) // AUTH: shard siganture auth
	// Zeta Protocol: FETCH APIs
	r.HandleFunc("/{namespace}/{repo}/reference/{refname:.*}", s.OnFunc(s.LsReference, protocol.DOWNLOAD)).Methods("GET").MatcherFunc(Z1Matcher)        // CHECKOUT: fetch reference
	r.HandleFunc("/{namespace}/{repo}/metadata/{revision:.*}", s.OnFunc(s.FetchMetadata, protocol.DOWNLOAD)).Methods("GET").MatcherFunc(Z1Matcher)      // CHECKOUT: download commit and tree/subtrees metadata ...
	r.HandleFunc("/{namespace}/{repo}/metadata/{revision:.*}", s.OnFunc(s.GetSparseMetadata, protocol.DOWNLOAD)).Methods("POST").MatcherFunc(Z1Matcher) // CHECKOUT: sparse checkout
	r.HandleFunc("/{namespace}/{repo}/metadata/batch", s.OnFunc(s.BatchMetadata, protocol.DOWNLOAD)).Methods("POST").MatcherFunc(Z1Matcher)             // CHECKOUT: batch metadata for FUSE
	r.HandleFunc("/{namespace}/{repo}/objects/batch", s.OnFunc(s.BatchObjects, protocol.DOWNLOAD)).Methods("POST").MatcherFunc(Z1Matcher)               // ENHANCED: batch objects Required to migrate from zeta to git
	r.HandleFunc("/{namespace}/{repo}/objects/share", s.OnFunc(s.ShareObjects, protocol.DOWNLOAD)).Methods("POST").MatcherFunc(Z1Matcher)               // CHECKOUT: shared signed oss urls
	r.HandleFunc("/{namespace}/{repo}/objects/{oid}", s.OnFunc(s.GetObject, protocol.DOWNLOAD)).Methods("GET").MatcherFunc(Z1Matcher)                   // ENHANCED: download object Required to migrate from zeta to git
	// Zeta Protocol: PUSH APIs
	r.HandleFunc("/{namespace}/{repo}/reference/{refname:.*}/objects/batch", s.OnFunc(s.BatchCheck, protocol.UPLOAD)).Methods("POST").MatcherFunc(Z1Matcher) // PUSH: batch check large objects
	r.HandleFunc("/{namespace}/{repo}/reference/{refname:.*}/objects/{oid}", s.OnFunc(s.PutObject, protocol.UPLOAD)).Methods("PUT").MatcherFunc(Z1Matcher)   // PUSH: PUT one large object
	r.HandleFunc("/{namespace}/{repo}/reference/{refname:.*}", s.OnFunc(s.Push, protocol.UPLOAD)).Methods("POST").MatcherFunc(Z1Matcher)                     // PUSH: push local commit to zeta server
}

func (s *Server) initialize() error {
	r := mux.NewRouter().UseEncodedPath()
	s.ProtocolZ1Router(r)
	s.ManagementRouter(r)
	s.r = r
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
		// default behavie
	}
	logrus.Infof("[%s] %s %s status: %d received: %d written: %d spent: %v", hw.F1RemoteAddr(), r.Method, r.RequestURI, hw.StatusCode(), tr.received, hw.Written(), spent)
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
	s.r.ServeHTTP(hw, r)
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
