// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/protocol"
	"github.com/golang-jwt/jwt/v5"
)

const (
	BearerPrefix = "Bearer "
)

type BearerMD struct {
	UID                  int64              `json:"uid,string"`
	RID                  int64              `json:"rid,string"`
	Operation            protocol.Operation `json:"operation"`
	jwt.RegisteredClaims                    // v5 new
}

func (t *BearerMD) Match(op protocol.Operation) bool {
	if t.Operation == protocol.PSEUDO {
		return true
	}
	switch op {
	case protocol.DOWNLOAD:
		return t.Operation == protocol.DOWNLOAD || t.Operation == protocol.UPLOAD
	case protocol.UPLOAD:
		return t.Operation == protocol.UPLOAD
	}
	return false
}

func GenerateJWT(u *database.User, rid int64, op protocol.Operation, expiresAt time.Time) (string, error) {
	now := time.Now()
	claims := BearerMD{
		UID:       u.ID,
		RID:       rid,
		Operation: op,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt), // expiresAt
			IssuedAt:  jwt.NewNumericDate(now),       // issued
			NotBefore: jwt.NewNumericDate(now),       // not before
		},
	}
	// HS256
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := t.SignedString([]byte(u.SignatureToken))
	return s, err
}

func (s *Server) ParseJWT(w http.ResponseWriter, r *http.Request, bearerToken string) (*database.User, *BearerMD, error) {
	var u *database.User
	var claims *BearerMD
	_, err := jwt.ParseWithClaims(bearerToken, &BearerMD{}, func(token *jwt.Token) (any, error) {
		var ok bool
		if claims, ok = token.Claims.(*BearerMD); !ok {
			return nil, jwt.ErrTokenMalformed
		}
		var sqlErr error
		if u, sqlErr = s.db.FindUser(r.Context(), claims.UID); sqlErr != nil {
			return nil, sqlErr
		}
		return []byte(u.SignatureToken), nil
	})
	if err != nil {
		switch {
		case errors.Is(err, jwt.ErrTokenMalformed):
			renderFailureFormat(w, r, http.StatusBadRequest, "malformed token: %s", err)
		case errors.Is(err, jwt.ErrTokenSignatureInvalid):
			renderFailureFormat(w, r, http.StatusForbidden, "invalid token: %s", err)
		case errors.Is(err, jwt.ErrTokenExpired) || errors.Is(err, jwt.ErrTokenNotValidYet):
			renderFailureFormat(w, r, http.StatusForbidden, "expired token: %s", err)
		case errors.Is(err, sql.ErrNoRows):
			renderFailureFormat(w, r, http.StatusNotFound, "user not found: %v", err)
		default:
			renderFailureFormat(w, r, http.StatusInternalServerError, "parse token error: %s", err)
		}
		return nil, nil, err
	}
	u.Guard()
	return u, claims, nil
}

func parseBearerToken(auth string) (string, bool) {
	if len(auth) < len(BearerPrefix) || !EqualFold(auth[:len(BearerPrefix)], BearerPrefix) {
		return "", false
	}
	return auth[len(BearerPrefix):], true
}
