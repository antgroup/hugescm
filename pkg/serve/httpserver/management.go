// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

// WARING: The management API is mainly used for testing and adding users. Do not use it in a production environment.

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/pkg/serve/argon2id"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/gorilla/mux"
	"golang.org/x/crypto/ssh"
)

type NewUser struct {
	UserName      string `json:"username"`
	Name          string `json:"name,omitempty"`
	Administrator bool   `json:"administrator"`
	Email         string `json:"email"`
	Password      string `json:"password"`
}

func (s *Server) NewUser(w http.ResponseWriter, r *http.Request) {
	var newUser NewUser
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		fmt.Println(err)
		renderFailureFormat(w, r, http.StatusBadRequest, "input body error: %v", err)
		return
	}
	if len(newUser.UserName) == 0 || len(newUser.Password) == 0 {
		renderFailure(w, r, http.StatusBadRequest, "username or password is empty")
		return
	}
	if len(newUser.Name) == 0 {
		newUser.Name = newUser.UserName
	}
	passwd, err := argon2id.CreateHash(newUser.Password, argon2id.DefaultParams)
	if err != nil {
		renderFailureFormat(w, r, http.StatusInternalServerError, "gen salt password error: %v", err)
		return
	}
	u, err := s.db.NewUser(r.Context(), &database.User{
		UserName:       newUser.UserName,
		Name:           newUser.Name,
		Administrator:  newUser.Administrator,
		Email:          newUser.Email,
		Password:       passwd,
		SignatureToken: strengthen.NewRID(),
	})
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	JsonEncode(w, u)
}

type NewRepo struct {
	Name          string `json:"name,omitempty"`
	Path          string `json:"path"`
	Description   string `json:"description"`
	VisibleLevel  int    `json:"visible_level,omitempty"`
	DefaultBranch string `json:"default_branch,omitempty"` // current branch
	UserName      string `json:"username,omitempty"`
	UID           int64  `json:"uid,omitempty"`
	NamespacePath string `json:"namespace_path,omitempty"`
	NamespaceID   int64  `json:"namespace_id,omitempty"`
	Empty         bool   `json:"empty,omitempty"`
}

func (s *Server) NewRepo(w http.ResponseWriter, r *http.Request) {
	var newRepo NewRepo
	if err := json.NewDecoder(r.Body).Decode(&newRepo); err != nil {
		fmt.Println(err)
		renderFailureFormat(w, r, http.StatusBadRequest, "input body error: %v", err)
		return
	}
	var u *database.User
	var err error
	switch {
	case len(newRepo.UserName) != 0:
		if u, err = s.db.SearchUser(r.Context(), newRepo.UserName); err != nil {
			s.renderErrorRaw(w, r, err)
			return
		}
	case newRepo.UID != 0:
		if u, err = s.db.FindUser(r.Context(), newRepo.UID); err != nil {
			s.renderErrorRaw(w, r, err)
			return
		}
	default:
		renderFailure(w, r, http.StatusBadRequest, "username or uid not given")
		return
	}
	var n *database.Namespace
	switch {
	case len(newRepo.NamespacePath) != 0:
		if n, err = s.db.FindNamespaceByPath(r.Context(), newRepo.NamespacePath); err != nil {
			s.renderErrorRaw(w, r, err)
			return
		}
	case newRepo.NamespaceID != 0:
		if n, err = s.db.FindNamespaceByID(r.Context(), newRepo.UID); err != nil {
			s.renderErrorRaw(w, r, err)
			return
		}
	default:
		renderFailure(w, r, http.StatusBadRequest, "namespace_path or namespace_id not given")
		return
	}
	repo, err := s.hub.New(r.Context(), &database.Repository{
		NamespaceID:   n.ID,
		Name:          newRepo.Name,
		Path:          newRepo.Path,
		Description:   newRepo.Description,
		VisibleLevel:  newRepo.VisibleLevel,
		DefaultBranch: newRepo.DefaultBranch,
	}, u, newRepo.Empty)
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	JsonEncode(w, repo)
}

type NewKey struct {
	UserName string `json:"username,omitempty"`
	UID      int64  `json:"uid,omitempty"`
	Title    string `json:"title"`
	Content  string `json:"content"`
}

func (s *Server) NewKey(w http.ResponseWriter, r *http.Request) {
	var newKey NewKey
	if err := json.NewDecoder(r.Body).Decode(&newKey); err != nil {
		fmt.Println(err)
		renderFailureFormat(w, r, http.StatusBadRequest, "input body error: %v", err)
		return
	}
	var u *database.User
	var err error
	switch {
	case len(newKey.UserName) != 0:
		if u, err = s.db.SearchUser(r.Context(), newKey.UserName); err != nil {
			s.renderErrorRaw(w, r, err)
			return
		}
	case newKey.UID != 0:
		if u, err = s.db.FindUser(r.Context(), newKey.UID); err != nil {
			s.renderErrorRaw(w, r, err)
			return
		}
	default:
		renderFailure(w, r, http.StatusBadRequest, "username or uid not given")
		return
	}
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(newKey.Content))
	if err != nil {
		renderFailureFormat(w, r, http.StatusBadRequest, "bad public key: %v", err)
		return
	}
	k, err := s.db.AddKey(r.Context(), &database.Key{
		UID:         u.ID,
		Content:     newKey.Content,
		Title:       newKey.Title,
		Type:        database.BasicKey,
		Fingerprint: ssh.FingerprintSHA256(pk),
	})
	if err != nil {
		s.renderErrorRaw(w, r, err)
		return
	}
	JsonEncode(w, k)
}

func (s *Server) ManagementRouter(r *mux.Router) {
	r.HandleFunc("/api/v1/user", s.NewUser).Methods("POST")
	r.HandleFunc("/api/v1/key", s.NewKey).Methods("POST")
	r.HandleFunc("/api/v1/repo", s.NewRepo).Methods("POST")
}
