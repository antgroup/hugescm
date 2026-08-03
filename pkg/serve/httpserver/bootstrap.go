// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package httpserver

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/pkg/serve/argon2id"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/sirupsen/logrus"
)

// bootstrapAdmin seeds the first administrator account from the [admin] config
// section, if present. It is idempotent: when the configured username already
// exists the seed is skipped and the existing record is never modified (no
// password reset, no privilege escalation), so it is safe to run on every
// start. A missing [admin] section or empty username is a no-op (log only);
// the server can still start and the operator can fall back to the management
// API. Only a non-nil error from an unexpected DB failure surfaces; even then
// the caller treats it as non-fatal.
func (s *Server) bootstrapAdmin(ctx context.Context) error {
	if s.Admin == nil || s.Admin.Username == "" {
		logrus.Info("no [admin] configured, skipping admin seed")
		return nil
	}
	if s.Admin.Password == "" {
		return fmt.Errorf("admin seed: password is empty for username %q", s.Admin.Username)
	}
	// Idempotency: skip when the configured username already exists.
	if _, err := s.db.SearchUser(ctx, s.Admin.Username); err == nil {
		logrus.Infof("admin user %q already exists, skipping seed", s.Admin.Username)
		return nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("admin seed: lookup %q: %w", s.Admin.Username, err)
	}
	passwd, err := argon2id.CreateHash(s.Admin.Password, argon2id.DefaultParams)
	if err != nil {
		return fmt.Errorf("admin seed: hash password: %w", err)
	}
	if _, err := s.db.NewUser(ctx, &database.User{
		UserName:       s.Admin.Username,
		Name:           s.Admin.Username,
		Administrator:  true,
		Email:          s.Admin.Email,
		Password:       passwd,
		SignatureToken: strengthen.NewRID(),
	}); err != nil {
		return fmt.Errorf("admin seed: create user: %w", err)
	}
	logrus.Infof("seeded admin user %q", s.Admin.Username)
	return nil
}
