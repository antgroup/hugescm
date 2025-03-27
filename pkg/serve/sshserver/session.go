// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sshserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/zeta/backend"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/serve"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/gliderlabs/ssh"
	"github.com/sirupsen/logrus"
)

const (
	connMetadataKey = "X-Conn-Metadata"
)

var (
	ErrRequiredContext = errors.New("required context")
)

type Addr struct {
	IP   string
	Path string
	Port int
}

func netAddrToAddr(addr net.Addr) *Addr {
	switch addr := addr.(type) {
	case *net.IPAddr:
		return &Addr{IP: addr.IP.String(), Port: 0}
	case *net.TCPAddr:
		return &Addr{IP: addr.IP.String(), Port: addr.Port}
	case *net.UDPAddr:
		return &Addr{IP: addr.IP.String(), Port: addr.Port}
	case *net.UnixAddr:
		return &Addr{Path: addr.Name}
	}
	return &Addr{}
}

type SessionCtx struct {
	UserName        string
	DisplayName     string
	UID             int64
	KID             int64
	SessionID       string
	ClientVersion   string
	UniqueID        int64
	KeyType         string
	Fingerprint     string
	IsDeployKey     bool
	RemoteAddress   *Addr
	LocalAddress    *Addr
	IsAdministrator bool
}

type request struct {
	RID             int64
	NamespacePath   string
	RepoPath        string
	DefaultBranch   string
	CompressionAlgo string
	HashAlgo        string
}

type Session struct {
	ssh.Session
	*SessionCtx
	*request
	env      map[string]string
	language string
	written  int64
	received int64
	start    time.Time
}

func (s *Server) NewSession(se ssh.Session) (*Session, error) {
	I := se.Context().Value(connMetadataKey)
	if I == nil {
		return nil, ErrRequiredContext
	}
	meta, ok := I.(*SessionCtx)
	if !ok {
		return nil, ErrRequiredContext
	}
	e := &Session{
		Session:    se,
		SessionCtx: meta,
		request:    &request{},
		env:        make(map[string]string),
		start:      time.Now(),
	}
	e.initializeEnv()
	e.language = serve.ParseLangEnv(e.Getenv("LANG"))
	return e, nil
}

func envKV(s string) (string, string) {
	if k, v, ok := strings.Cut(s, "="); ok {
		return k, v
	}
	return s, ""
}

func (e *Session) initializeEnv() {
	for _, envLine := range e.Environ() {
		k, v := envKV(envLine)
		e.env[k] = v
	}
}

func (e *Session) HasEnv(k string) bool {
	_, ok := e.env[k]
	return ok
}

func (e *Session) LookupEnv(k string) (string, bool) {
	v, ok := e.env[k]
	return v, ok
}

func (e *Session) Getenv(k string) string {
	return e.env[k]
}

// Read reads up to len(data) bytes from the channel.
func (e *Session) Read(data []byte) (int, error) {
	n, err := e.Session.Read(data)
	e.received += int64(n)
	return n, err
}

// Write writes len(data) bytes to the channel.
func (e *Session) Write(data []byte) (int, error) {
	n, err := e.Session.Write(data)
	e.written += int64(n)
	return n, err
}

// WriteError: format error after write to session.Stderr
func (e *Session) WriteError(format string, args ...any) {
	message := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintln(e.Stderr(), strings.TrimRightFunc(message, unicode.IsSpace))
}

func (e *Session) makeRemoteURL(endpoint string) string {
	return fmt.Sprintf("zeta@%s:%s/%s", endpoint, e.NamespacePath, e.RepoPath)
}

func (e *Session) W(message string) string {
	return serve.Translate(e.language, message)
}

func (e *Session) ExitError(err error) int {
	switch {
	case errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.Canceled):
		// canceled
		return 200
	case plumbing.IsNoSuchObject(err), plumbing.IsErrRevNotFound(err), os.IsNotExist(err),
		database.IsNotFound(err), object.IsErrDirectoryNotFound(err), object.IsErrEntryNotFound(err):
		e.WriteError("resource not found:  %s\n", err)
		return 404
	case backend.IsErrMismatchedObjectType(err), database.IsErrExist(err), os.IsExist(err):
		e.WriteError("resource conflict:  %s\n", err)
		return 409
	default:
		logrus.Errorf("access %s/%s internal server error: %v", e.NamespacePath, e.RepoPath, err)
		e.WriteError("%s", e.W("internal server error")) //nolint:govet
	}
	return 500
}

func (e *Session) ExitFormat(code int, format string, a ...any) int {
	e.WriteError(format, a...)
	return code
}
