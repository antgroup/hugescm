// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/antgroup/hugescm/pkg/serve/argon2id"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

// doUserAuth authenticates the user without looking up a namespace/repository.
// Unlike doAuth (which requires {namespace}/{repo} in the URL path), this middleware
// only validates the user credentials and returns a *Request with U set (N and R are nil).
func (s *Server) doUserAuth(w http.ResponseWriter, r *http.Request) (*Request, error) {
	cred := r.Header.Get(AUTHORIZATION)
	bearerToken, ok := parseBearerToken(cred)
	if !ok {
		return s.userBasicAuth(w, r, cred)
	}
	return s.userBearerAuth(w, r, bearerToken)
}

func (s *Server) userBasicAuth(w http.ResponseWriter, r *http.Request, cred string) (*Request, error) {
	user, password, ok := parseBasicAuth(cred)
	if !ok {
		renderFailure(w, r, http.StatusUnauthorized, "missing credential")
		return nil, ErrStop
	}
	if allowedTokenUserName[user] {
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
	u.Guard()
	return &Request{
		Request: r,
		U:       u,
	}, nil
}

func (s *Server) userBearerAuth(w http.ResponseWriter, r *http.Request, bearerToken string) (*Request, error) {
	u, _, err := s.ParseJWT(w, r, bearerToken)
	if err != nil {
		return nil, err
	}
	return &Request{
		Request: r,
		U:       u,
	}, nil
}

// requireAdmin checks if the current user is an administrator.
// Returns true and writes nothing if admin; writes 403 and returns false otherwise.
func requireAdmin(w http.ResponseWriter, r *Request) bool {
	if r.U.Administrator {
		return true
	}
	renderFailureFormat(w, r.Request, http.StatusForbidden, "admin access required, current user: %s", r.U.UserName)
	return false
}

// requireSelfOrAdmin checks if the current user is the target user or an administrator.
// Returns true and writes nothing if authorized; writes 403 and returns false otherwise.
func requireSelfOrAdmin(w http.ResponseWriter, r *Request, targetUID int64) bool {
	if r.U.ID == targetUID || r.U.Administrator {
		return true
	}
	renderFailureFormat(w, r.Request, http.StatusForbidden, "access denied, current user: %s cannot access target user: %d", r.U.UserName, targetUID)
	return false
}

// requireTargetUser extracts {uid} from the request, looks up the target user, and checks
// if the current user has access (self or admin). Returns the target user and true on success.
func (s *Server) requireTargetUser(w http.ResponseWriter, r *Request) (*database.User, bool) {
	uidStr := chi.URLParam(r.Request, "uid")
	uid, err := strconv.ParseInt(uidStr, 10, 64)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusBadRequest, "invalid user id: %s", uidStr)
		return nil, false
	}
	target, err := s.db.FindUser(r.Context(), uid)
	if err != nil {
		s.renderErrorRaw(w, r.Request, err)
		return nil, false
	}
	if !requireSelfOrAdmin(w, r, uid) {
		return nil, false
	}
	return target, true
}

// requireRepoAdmin checks if the current user has OwnerAccess on the repository.
// Returns true and writes nothing if owner; writes 403 and returns false otherwise.
func (s *Server) requireRepoAdmin(w http.ResponseWriter, r *Request) bool {
	_, accessLevel, err := s.db.RepoAccessLevel(r.Context(), r.R, r.U)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusInternalServerError, "check access error: %v", err)
		return false
	}
	if accessLevel >= database.OwnerAccess {
		return true
	}
	renderFailureFormat(w, r.Request, http.StatusForbidden, "owner access required, current user: %s", r.U.UserName)
	return false
}

// requireRepoMaster checks if the current user has MasterAccess or higher on the repository.
// Returns true if authorized; writes 403 and returns false otherwise.
func (s *Server) requireRepoMaster(w http.ResponseWriter, r *Request) bool {
	_, accessLevel, err := s.db.RepoAccessLevel(r.Context(), r.R, r.U)
	if err != nil {
		renderFailureFormat(w, r.Request, http.StatusInternalServerError, "check access error: %v", err)
		return false
	}
	if accessLevel.Sudo() {
		return true
	}
	renderFailureFormat(w, r.Request, http.StatusForbidden, "master access required, current user: %s", r.U.UserName)
	return false
}
