// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/antgroup/hugescm/pkg/serve/argon2id"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

var (
	ErrStop         = errors.New("stop")
	ErrAccessDenied = errors.New("access denied")
)

// EqualFold is strings.EqualFold, ASCII only. It reports whether s and t
// are equal, ASCII-case-insensitively.
func EqualFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		if lower(s[i]) != lower(t[i]) {
			return false
		}
	}
	return true
}

// lower returns the ASCII lowercase version of b.
func lower(b byte) byte {
	if 'A' <= b && b <= 'Z' {
		return b + ('a' - 'A')
	}
	return b
}

// parseBasicAuth parses an HTTP Basic Authentication string.
// "Basic QWxhZGRpbjpvcGVuIHNlc2FtZQ==" returns ("Aladdin", "open sesame", true).
func parseBasicAuth(auth string) (username, password string, ok bool) {
	const prefix = "Basic "
	// Case insensitive prefix match. See Issue 22736.
	if len(auth) < len(prefix) || !EqualFold(auth[:len(prefix)], prefix) {
		return "", "", false
	}
	c, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return "", "", false
	}
	cs := string(c)
	username, password, ok = strings.Cut(cs, ":")
	if !ok {
		return "", "", false
	}
	return username, password, true
}

func (s *Server) MakeAuthenticateHeader(r *http.Request) string {
	return "Basic realm=" + r.Host
}

var (
	allowedTokenUserName = map[string]bool{
		"zeta":      true,
		"git":       true,
		"gitlab-ci": true,
	}
)

func (s *Server) basicAuth(w http.ResponseWriter, r *http.Request, operation protocol.Operation, cred string) (*Request, error) {
	user, password, ok := parseBasicAuth(cred)
	if !ok {
		renderFailure(w, r, http.StatusUnauthorized, "missing credential")
		return nil, ErrStop
	}
	if allowedTokenUserName[user] {
		// TODO: token
		renderFailure(w, r, http.StatusUnauthorized, "unsupported token")
		return nil, ErrStop
	}
	u, err := s.db.SearchUser(r.Context(), user)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderFailureFormat(w, r, http.StatusUnauthorized, "user '%s' not found", err)
			return nil, err
		}
		renderFailure(w, r, http.StatusInternalServerError, "internal server error")
		logrus.Errorf("find user '%s' error: %v", user, err)
		return nil, err
	}
	if ok, err = argon2id.ComparePasswordAndHash(password, u.Password); err != nil {
		renderFailure(w, r, http.StatusInternalServerError, "broken salted password")
		return nil, err
	}
	if !ok {
		renderFailure(w, r, http.StatusUnauthorized, "password unmatched")
		return nil, ErrStop
	}
	if !u.LockedAt.IsZero() {
		renderFailureFormat(w, r, http.StatusForbidden, "user '%s' is locked at: %v", u.UserName, u.LockedAt)
		return nil, ErrStop
	}
	// cleanup
	u.Guard()
	mv := mux.Vars(r)
	namespacePath, repoPath := mv["namespace"], mv["repo"]
	ns, repo, err := s.db.FindRepositoryByPath(r.Context(), namespacePath, repoPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderFailureFormat(w, r, http.StatusNotFound, "repo '%s/%s' not found", namespacePath, repoPath)
			return nil, ErrStop
		}
		renderFailureFormat(w, r, http.StatusInternalServerError, "search repo '%s/%s' error: %v", namespacePath, repoPath, err)
		return nil, ErrStop
	}
	if _, err = s.checkAccess(w, r, operation, repo, u); err != nil {
		return nil, err
	}
	return &Request{
		Request: r,
		U:       u,
		N:       ns,
		R:       repo,
	}, nil
}

func (s *Server) doAuth(w http.ResponseWriter, r *http.Request, operation protocol.Operation) (*Request, error) {
	cred := r.Header.Get(AUTHORIZATION)
	bearerToken, ok := parseBearerToken(cred)
	if !ok {
		return s.basicAuth(w, r, operation, cred)
	}
	u, m, err := s.ParseJWT(w, r, bearerToken)
	if err != nil {
		return nil, err
	}
	if !m.Match(operation) {
		renderFailureFormat(w, r, http.StatusForbidden, "access denied, bearer token operation '%s' not match request operation: '%s'", m.Operation, operation)
		return nil, ErrStop
	}
	mv := mux.Vars(r)
	namespacePath, repoPath := mv["namespace"], mv["repo"]
	ns, repo, err := s.db.FindRepositoryByPath(r.Context(), namespacePath, repoPath)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			renderFailureFormat(w, r, http.StatusNotFound, "repo '%s/%s' not found", namespacePath, repoPath)
			return nil, ErrStop
		}
		renderFailureFormat(w, r, http.StatusInternalServerError, "search repo '%s/%s' error: %v", namespacePath, repoPath, err)
		return nil, ErrStop
	}
	if _, err = s.checkAccess(w, r, operation, repo, u); err != nil {
		return nil, err
	}
	return &Request{
		Request: r,
		U:       u,
		N:       ns,
		R:       repo,
	}, nil
}

func (s *Server) OnFunc(fn HandlerFunc, operation protocol.Operation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		req, err := s.doAuth(w, r, operation)
		if err != nil {
			return
		}
		fn(w, req)
	}
}

func checkRepoReadable(u *database.User, repo *database.Repository, accessLevel database.AccessLevel) bool {
	if accessLevel.Readable() {
		return true
	}
	return repo.IsPublic() || (repo.IsInternal() && u.Type != database.UserTypeRemoteUser)
}

func (s *Server) checkAccess(w http.ResponseWriter, r *http.Request, operation protocol.Operation, repo *database.Repository, u *database.User) (database.AccessLevel, error) {
	if u.Administrator {
		return database.OwnerAccess, nil
	}
	_, accessLevel, err := s.db.RepoAccessLevel(r.Context(), repo, u)
	if err != nil {
		logrus.Errorf("%s check repo access_level error: %v", r.RequestURI, err)
		renderFailureFormat(w, r, http.StatusInternalServerError, "check user's access for repository error: %v", err)
		return database.NoneAccess, err
	}
	switch operation {
	case protocol.DOWNLOAD:
		if !checkRepoReadable(u, repo, accessLevel) {
			renderFailureFormat(w, r, http.StatusForbidden, "[DOWNLOAD] access denied, current user: %s", u.UserName)
			return accessLevel, ErrAccessDenied
		}
	case protocol.UPLOAD:
		if !accessLevel.Writeable() {
			renderFailureFormat(w, r, http.StatusForbidden, "[UPLOAD] access denied, current user: %s", u.UserName)
			return accessLevel, ErrAccessDenied
		}
	case protocol.SUDO:
		if !accessLevel.Sudo() {
			renderFailureFormat(w, r, http.StatusForbidden, "[SUDO] access denied, current user: %s", u.UserName)
			return accessLevel, ErrAccessDenied
		}
	default:
		renderFailureFormat(w, r, http.StatusBadRequest, "bad operation name '%s'", operation)
		return accessLevel, fmt.Errorf("bad operation name '%s'", operation)
	}
	return accessLevel, nil
}
