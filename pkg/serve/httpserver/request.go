// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"fmt"
	"net/http"

	"github.com/antgroup/hugescm/pkg/serve"
	"github.com/antgroup/hugescm/pkg/serve/database"
)

type Request struct {
	*http.Request
	U *database.User
	N *database.Namespace
	R *database.Repository
}

func (r *Request) W(message string) string {
	return serve.W(r.Request, message)
}

func resolveScheme(r *http.Request) string {
	if scheme := r.Header.Get("X-Forwarded-Proto"); len(scheme) != 0 {
		return scheme
	}
	if scheme := r.Header.Get("X-Real-Scheme"); len(scheme) != 0 {
		return scheme
	}
	if scheme := r.Header.Get("X-Client-Scheme"); len(scheme) != 0 {
		return scheme
	}
	return "http"
}

func (r *Request) makeRemoteURL() string {
	return fmt.Sprintf("%s://%s/%s/%s", resolveScheme(r.Request), r.Host, r.N.Path, r.R.Path)
}
