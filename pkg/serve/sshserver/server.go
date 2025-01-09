// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package sshserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"

	"github.com/antgroup/hugescm/pkg/serve"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/repo"
	"github.com/antgroup/hugescm/pkg/serve/sshserver/rainbow"
	"github.com/gliderlabs/ssh"
	"github.com/sirupsen/logrus"
	gossh "golang.org/x/crypto/ssh"
)

const (
	DefaultUser       = "zeta"
	ServeCommand      = "zeta-serve"
	AnonymousUserName = "Anonymous"
)

//	ls-remote --reference=$REFNAME
//	metadata --commit=$COMMIT [--depth=N] [--deepen-from|--deepen] [--batch]
//	objects [--oid=$OID|--batch|--share]
//	push --reference $REFNAME [--oid $OID|--batch-check]

// zeta co zeta@zeta.io:zeta-dev/zeta
type Server struct {
	*ServerConfig
	srv        *ssh.Server
	db         database.DB
	hub        repo.Repositories
	serverName string
	uniqueID   int64
}

func NewServer(sc *ServerConfig) (*Server, error) {
	s := &Server{
		ServerConfig: sc,
		serverName:   sc.BannerVersion,
	}
	cfg, err := sc.DB.MakeConfig()
	if err != nil {
		return nil, err
	}
	if s.db, err = database.NewDB(cfg); err != nil {
		return nil, err
	}
	if s.hub, err = repo.NewRepositories(sc.Repositories, sc.PersistentOSS, sc.Cache, s.db); err != nil {
		_ = s.db.Close()
		return nil, err
	}
	srv := &ssh.Server{
		Addr:             sc.Listen,
		MaxTimeout:       sc.MaxTimeout.Duration,
		IdleTimeout:      sc.IdleTimeout.Duration,
		Version:          sc.BannerVersion,
		PublicKeyHandler: s.OnKey,
		Handler:          s.OnSession,
	}
	for _, pk := range sc.HostPrivateKeys {
		addHostKeyInternal(srv, []byte(pk))
	}
	s.srv = srv
	return s, nil
}

func addHostKeyInternal(srv *ssh.Server, pemBytes []byte) {
	key, err := gossh.ParsePrivateKey(pemBytes)
	if err != nil {
		logrus.Errorf("Parse HostKey error: %v", err)
		return
	}
	srv.AddHostKey(key)
	logrus.Infof("Load HostKey <%s> Fingerprint: %v", key.PublicKey().Type(), gossh.FingerprintSHA256(key.PublicKey()))
}

func (s *Server) ListenAndServe() error {
	if err := serve.RegisterLanguageMatcher(); err != nil {
		logrus.Errorf("register languages matcher error: %v", err)
	}
	logrus.Infof("Zeta SSH Server listen: %v", s.Listen)
	return s.srv.ListenAndServe()
}

func (s *Server) OnKey(ctx ssh.Context, key ssh.PublicKey) bool {
	fingerprint := gossh.FingerprintSHA256(key)
	k, err := s.db.SearchKey(ctx, fingerprint)
	if errors.Is(err, sql.ErrNoRows) {
		return false
	}
	if err != nil {
		logrus.Errorf("PublicKeyHandle: auth failed for key %s: %v", fingerprint, err)
		return false
	}
	ctx.SetValue(connMetadataKey, &SessionCtx{
		KID:           k.ID,
		UID:           k.UID,
		RemoteAddress: netAddrToAddr(ctx.RemoteAddr()),
		LocalAddress:  netAddrToAddr(ctx.LocalAddr()),
		SessionID:     ctx.SessionID(),
		UniqueID:      atomic.AddInt64(&s.uniqueID, 1),
		ClientVersion: ctx.ClientVersion(),
		KeyType:       key.Type(),
		Fingerprint:   fingerprint,
		IsDeployKey:   k.Type == database.DeployKey,
	})
	return true
}

func (s *Server) OnSession(sess ssh.Session) {
	se, err := s.NewSession(sess)
	if err != nil {
		fmt.Fprintf(sess.Stderr(), "bad ssh session")
		logrus.Errorf("bad ssh session: %v", err)
		_ = sess.Exit(1)
		return
	}
	exitCode := s.handleSession(se)
	// TODO log request
	_ = se.Exit(exitCode)
}

func (s *Server) displayUser(e *Session) int {
	displayName := AnonymousUserName
	if e.UID != 0 {
		u, err := s.db.FindUser(e.Context(), e.UID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				e.WriteError("user[id %d] not found", e.UID)
				return 1
			}
			e.WriteError("internal server error: %v", err)
			return 1
		}
		if !u.LockedAt.IsZero() {
			e.WriteError("User '%s' locked at %v", u.UserName, u.LockedAt)
			return 1
		}
		displayName = u.Name
	}
	if pty, _, ok := e.Pty(); ok {
		rainbow.Display(e, &rainbow.DisplayOpts{
			UserName:    displayName,
			Width:       pty.Window.Width,
			Fingerprint: e.Fingerprint,
			KeyType:     e.KeyType,
		})
		return 0
	}
	rainbow.Display(e.Stderr(), &rainbow.DisplayOpts{
		UserName:    displayName,
		Width:       80,
		Fingerprint: e.Fingerprint,
		KeyType:     e.KeyType,
	})
	return 0
}

func (s *Server) handleSession(e *Session) int {
	if e.User() != DefaultUser {
		e.WriteError("supports only username '\x1b[33mzeta\x1b[0m', current '\x1b[31m%s\x1b[0m'\n", e.User())
		return 1
	}
	args := e.Command()
	if len(args) == 0 {
		return s.displayUser(e)
	}
	if args[0] != ServeCommand {
		e.WriteError("unsupport command '\x1b[31m%s\x1b[0m'", args[0])
		return 1
	}
	logrus.Infof("new command: %s user-agent: %s", e.RawCommand(), strings.TrimPrefix(e.ClientVersion, "SSH-2.0-"))
	cmd, err := NewCommand(args[1:])
	if err != nil {
		e.WriteError("fatal: \x1b[31m%v\x1b[0m", err)
		return 1
	}
	return cmd.Exec(&RunCtx{
		S:       s,
		Session: e,
	})
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil || s.srv == nil {
		return nil
	}
	if err := s.srv.Shutdown(ctx); err != nil {
		logrus.Errorf("shutdown ssh server %v", err)
	}
	if s.db != nil {
		s.db.Close()
	}
	return nil
}

func (s *Server) open(e *Session) (repo.Repository, error) {
	rr, err := s.hub.Open(e.Context(), e.RID, e.CompressionAlgo, e.DefaultBranch)
	if err != nil {
		return nil, err
	}
	return rr, nil
}
