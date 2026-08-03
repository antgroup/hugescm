// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/antgroup/hugescm/pkg/serve/argon2id"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/golang-jwt/jwt/v5"
	"github.com/sirupsen/logrus"
)

// webSessionClaims is the JWT payload for web session cookies.
// Unlike BearerMD (which is repo-scoped and operation-specific),
// the web session only tracks the UID — a general-purpose session.
type webSessionClaims struct {
	UID int64 `json:"uid,string"`
	jwt.RegisteredClaims
}

// generateWebSessionJWT issues a JWT for a web session and returns the value
// to be set as a cookie. It reuses the user's SignatureToken as the HS256 key
// (consistent with the existing BearerMD workflow in bearer.go).
func (s *Server) generateWebSessionJWT(u *database.User) (string, error) {
	now := time.Now()
	expiresAt := now.Add(time.Duration(webSessionExpiry) * time.Second)
	claims := webSessionClaims{
		UID: u.ID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    webJwtIssuer,
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return t.SignedString([]byte(u.SignatureToken))
}

// parseWebSessionJWT verifies a web session JWT and returns the authenticated user.
func (s *Server) parseWebSessionJWT(ctx context.Context, tokenStr string) (*database.User, error) {
	var u *database.User
	_, err := jwt.ParseWithClaims(tokenStr, &webSessionClaims{}, func(token *jwt.Token) (any, error) {
		claims, ok := token.Claims.(*webSessionClaims)
		if !ok {
			return nil, jwt.ErrTokenMalformed
		}
		var sqlErr error
		u, sqlErr = s.db.FindUser(ctx, claims.UID)
		if sqlErr != nil {
			return nil, sqlErr
		}
		return []byte(u.SignatureToken), nil
	})
	if err != nil {
		return nil, err
	}
	u.Guard()
	return u, nil
}

// webAuthMiddleware extracts the web session JWT from the cookie and sets
// the authenticated user in the request context. If no valid session is found,
// it redirects to the login page. Compatible with chi.Use.
func (s *Server) webAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(webSessionCookieName)
		if err != nil || cookie.Value == "" {
			redirectToLogin(w, r)
			return
		}
		u, err := s.parseWebSessionJWT(r.Context(), cookie.Value)
		if err != nil {
			logrus.Debugf("web session token invalid: %v", err)
			redirectToLogin(w, r)
			return
		}
		if !u.LockedAt.IsZero() {
			clearSessionCookie(w)
			redirectToLogin(w, r)
			return
		}
		ctx := context.WithValue(r.Context(), webUserKey{}, u)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// webUserKey is the context key for the web session user.
type webUserKey struct{}

// webAdminMiddleware guards admin-only web routes. It runs after the parent
// webAuthMiddleware, which has already authenticated the user and injected it
// into the request context, so this only checks the administrator flag.
// Non-admins get a 403, mirroring requireAdmin (auth_open_api.go).
func webAdminMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		u := webUserFromContext(r)
		if u == nil || !u.Administrator {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// webUserFromContext returns the authenticated user from the request context.
func webUserFromContext(r *http.Request) *database.User {
	if u, ok := r.Context().Value(webUserKey{}).(*database.User); ok {
		return u
	}
	return nil
}

// handleWebLoginGet renders the login page.
func (s *Server) handleWebLoginGet(w http.ResponseWriter, r *http.Request) {
	data := &webTemplateData{
		Title: "Login",
	}
	s.renderer.renderPage(w, s.serverName, "login", data)
}

// handleWebLoginPost verifies credentials and sets the session cookie.
func (s *Server) handleWebLoginPost(w http.ResponseWriter, r *http.Request) {
	// HTML <form> posts as application/x-www-form-urlencoded; parse that first
	// so we don't consume r.Body with json.Decoder and lose the data.
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	username := r.PostFormValue("username")
	password := r.PostFormValue("password")

	if username == "" || password == "" {
		data := &webTemplateData{
			Title:   "Login",
			Content: map[string]string{"Error": "username and password are required"},
		}
		s.renderer.renderPage(w, s.serverName, "login", data)
		return
	}

	u, err := s.db.SearchUser(r.Context(), username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			data := &webTemplateData{
				Title:   "Login",
				Content: map[string]string{"Error": "user not found"},
			}
			s.renderer.renderPage(w, s.serverName, "login", data)
			return
		}
		logrus.Errorf("web login: search user error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	ok, err := argon2id.ComparePasswordAndHash(password, u.Password)
	if err != nil {
		logrus.Errorf("web login: password verify error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if !ok {
		data := &webTemplateData{
			Title:   "Login",
			Content: map[string]string{"Error": "password mismatch"},
		}
		s.renderer.renderPage(w, s.serverName, "login", data)
		return
	}

	if !u.LockedAt.IsZero() {
		data := &webTemplateData{
			Title:   "Login",
			Content: map[string]string{"Error": "this user account is locked"},
		}
		s.renderer.renderPage(w, s.serverName, "login", data)
		return
	}

	token, err := s.generateWebSessionJWT(u)
	if err != nil {
		logrus.Errorf("web login: generate JWT error: %v", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	// Set HTTP-only cookie
	http.SetCookie(w, &http.Cookie{
		Name:     webSessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   webSessionExpiry,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	// Redirect to return path or repos list
	returnPath := r.URL.Query().Get("return")
	if returnPath == "" {
		returnPath = "/"
	}
	http.Redirect(w, r, returnPath, http.StatusSeeOther)
}

// handleWebLogout clears the session cookie and redirects to login.
func (s *Server) handleWebLogout(w http.ResponseWriter, r *http.Request) {
	clearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

// clearSessionCookie deletes the web session cookie.
func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     webSessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

// redirectToLogin sends a 302 to /login with a return path.
func redirectToLogin(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	if r.URL.RawQuery != "" {
		path += "?" + r.URL.RawQuery
	}
	http.Redirect(w, r, "/login?return="+path, http.StatusFound)
}
