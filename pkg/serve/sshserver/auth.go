// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sshserver

import (
	"database/sql"
	"errors"

	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
)

func (s *Server) checkAccessForDeployKey(e *Session, repoPath string, operation protocol.Operation) int {
	switch operation {
	case protocol.DOWNLOAD:
		ok, err := s.db.IsDeployKeyEnabled(e.Context(), e.RID, e.KID)
		if err != nil {
			e.WriteError("find repo '%s' error: %v", repoPath, err)
			return 500
		}
		if !ok {
			e.WriteError("Deploy Key not enabled for '%s'", repoPath)
			return 403
		}
	default:
		e.WriteError("Deploy Key no %s access", operation)
		return 403
	}
	return 0
}

func checkRepoReadable(u *database.User, repo *database.Repository, accessLevel database.AccessLevel) bool {
	if accessLevel.Readable() {
		return true
	}
	return repo.IsPublic() || (repo.IsInternal() && u.Type != database.UserTypeRemoteUser)
}

func (s *Server) doPermissionCheck(e *Session, repoPath string, operation protocol.Operation) int {
	repoParts := strengthen.SplitPath(repoPath)
	if len(repoParts) < 2 {
		e.WriteError("bad repo relative path '%s'", repoPath)
		return 400
	}
	namespacePath, repoName := repoParts[0], repoParts[1]
	ns, repo, err := s.db.FindRepositoryByPath(e.Context(), namespacePath, repoName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			e.WriteError("repo '%s' not found", repoPath)
			return 404
		}
		e.WriteError("find repo '%s' error: %v", repoPath, err)
		return 500
	}
	e.NamespacePath = ns.Path
	e.RepoPath = repo.Path
	e.RID = repo.ID
	e.DefaultBranch = repo.DefaultBranch
	e.CompressionAlgo = repo.CompressionAlgo
	e.HashAlgo = repo.HashAlgo
	if e.IsDeployKey {
		return s.checkAccessForDeployKey(e, repoPath, operation)
	}
	u, err := s.db.FindUser(e.Context(), e.UID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			e.WriteError("user-%d not found", e.UID)
			return 404
		}
		e.WriteError("find user-%d error: %v", e.UID, err)
		return 500
	}
	if !u.LockedAt.IsZero() {
		e.WriteError("User '%s' locked at %v", u.UserName, u.LockedAt)
		return 403
	}
	e.IsAdministrator = u.Administrator
	if u.Administrator {
		return 0
	}
	_, accessLevel, err := s.db.RepoAccessLevel(e.Context(), repo, u)
	if err != nil {
		e.WriteError("check user's access for repository error: %v", err)
		return 500
	}
	switch operation {
	case protocol.DOWNLOAD:
		if !checkRepoReadable(u, repo, accessLevel) {
			e.WriteError("[DOWNLOAD] access denied, current user: %s", u.UserName)
			return 403
		}
	case protocol.UPLOAD:
		if !accessLevel.Writeable() {
			e.WriteError("[UPLOAD] access denied, current user: %s", u.UserName)
			return 403
		}
	default:
		e.WriteError("bad operation: %s", operation)
		return 400
	}
	return 0
}
