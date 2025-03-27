// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package repo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/plumbing/format/pktline"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/zeta"
	"github.com/antgroup/hugescm/modules/zeta/object"
	"github.com/antgroup/hugescm/pkg/serve"
	"github.com/antgroup/hugescm/pkg/serve/database"
	"github.com/antgroup/hugescm/pkg/serve/odb"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

const (
	ansiRegex = "[\u001B\u009B][[\\]()#;?]*(?:(?:(?:[a-zA-Z\\d]*(?:;[a-zA-Z\\d]*)*)?\u0007)|(?:(?:\\d{1,4}(?:;\\d{0,4})*)?[\\dA-PRZcf-ntqry=><~]))"
)

var (
	trimAnsiRegex    = regexp.MustCompile(ansiRegex)
	ErrReportStarted = errors.New("protocol: reporter start")
)

func StripAnsi(s string) string {
	return trimAnsiRegex.ReplaceAllString(s, "")
}

func messageSplit(message string) (string, string) {
	if i := strings.IndexAny(message, "\r\n"); i != -1 {
		return message[0:i], message[i+1:]
	}
	return message, ""
}

type Command struct {
	RID           int64                  `json:"rid"`
	UID           int64                  `json:"uid"`
	ReferenceName plumbing.ReferenceName `json:"reference_name"`
	OldRev        string                 `json:"old_rev"`
	NewRev        string                 `json:"new_rev"`
	Language      string                 // language
	Terminal      string                 // term
	M             int
	B             int
}

func (c *Command) W(message string) string {
	return serve.Translate(c.Language, message)
}

func (c *Command) UpdateStats(s string) {
	kv := strengthen.StrSplitSkipEmpty(s, ';', 2)
	for _, k := range kv {
		if a, b, ok := strings.Cut(k, "-"); ok {
			i, err := strconv.Atoi(b)
			if err != nil {
				continue
			}
			if a == "m" {
				c.M = i
				continue
			}
			if a == "b" {
				c.B = i
			}
		}
	}
}

type reporter struct {
	pktline.Encoder
}

func newReporter(w io.Writer) *reporter {
	return &reporter{Encoder: *pktline.NewEncoder(w)}
}

func (r *reporter) close() error {
	return r.Flush()
}

func (r *reporter) rate(format string, a ...any) error {
	message := fmt.Sprintf(format, a...)
	return r.Encodef("rate %s", message)
}

func (r *reporter) status(format string, a ...any) error {
	message := fmt.Sprintf(format, a...)
	return r.Encodef("status %s", message)
}

func (r *reporter) ok(cmd *Command, newRev string) error {
	return r.Encodef("ok %s %s", cmd.ReferenceName, newRev)
}

func (r *reporter) ng(cmd *Command, format string, a ...any) error {
	message := fmt.Sprintf(format, a...)
	logrus.Errorf("[%s] %s", cmd.ReferenceName, message)
	return r.Encodef("ng %s %s", cmd.ReferenceName, message)
}

type QR struct {
	*odb.QuarantineDB
	seen      map[plumbing.Hash]bool
	commits   []plumbing.Hash
	forcePush bool
}

func NewQR(o *odb.ODB, quarantineDir string) (*QR, error) {
	d, err := odb.NewQuarantineDB(o, quarantineDir)
	if err != nil {
		return nil, err
	}
	return &QR{QuarantineDB: d, seen: make(map[plumbing.Hash]bool), forcePush: true}, nil
}

func (r *QR) Close() error {
	if r.QuarantineDB != nil {
		return r.QuarantineDB.Close()
	}
	return nil
}

func (r *QR) checkTreeIntegrity(ctx context.Context, cmd *Command, rr *reporter, oid plumbing.Hash) error {
	if r.seen[oid] {
		// checked
		return nil
	}
	tree, err := r.Tree(ctx, oid)
	if err != nil {
		_ = rr.ng(cmd, "resolve tree '%s' error: %v", oid, err)
		return err
	}
	for _, e := range tree.Entries {
		switch e.Type() {
		case object.TreeObject:
			if err := r.checkTreeIntegrity(ctx, cmd, rr, e.Hash); err != nil {
				return err
			}
		case object.FragmentsObject:
			ff, err := r.Fragments(ctx, e.Hash)
			if err != nil {
				_ = rr.ng(cmd, "fragments '%s' not exists", e.Hash)
				return err
			}
			for _, fe := range ff.Entries {
				if err := r.Exists(ctx, fe.Hash, false); err != nil {
					_ = rr.ng(cmd, "blob '%s' not exists", fe.Hash)
					return zeta.NewErrNotExist("blob", fe.Hash.String())
				}
			}
		case object.BlobObject:
			if err := r.Exists(ctx, e.Hash, false); err != nil {
				_ = rr.ng(cmd, "blob '%s' not exists", e.Hash)
				return zeta.NewErrNotExist("blob", e.Hash.String())
			}
		default:
			return fmt.Errorf("unsupported object type %v", e.Type())
		}
	}
	r.seen[oid] = true
	return nil
}

func (r *QR) checkCommitIntegrity(ctx context.Context, cmd *Command, rr *reporter, oid plumbing.Hash) error {
	cc, isolated, err := r.ParseRev(ctx, oid)
	if err != nil {
		_ = rr.ng(cmd, "peeled object '%s' error: %v", oid, err)
		return err
	}
	_ = rr.rate("check '%s' integrity", oid)
	if oid.String() == cmd.OldRev {
		r.forcePush = false
		return nil
	}
	if r.seen[oid] {
		return nil
	}
	// The commit already exists on the server, so we don't need to continue with the integrity check.
	if isolated {
		if err := r.checkTreeIntegrity(ctx, cmd, rr, cc.Tree); err != nil {
			return err
		}
		r.commits = append(r.commits, oid)
	}
	r.seen[oid] = true
	for _, p := range cc.Parents {
		if err := r.checkCommitIntegrity(ctx, cmd, rr, p); err != nil {
			return err
		}
	}
	return nil
}

func (r *QR) checkIntegrity(ctx context.Context, cmd *Command, rr *reporter) error {
	if cmd.NewRev == plumbing.ZERO_OID {
		return nil
	}
	return r.checkCommitIntegrity(ctx, cmd, rr, plumbing.NewHash(cmd.NewRev))
}

func (r *repository) DoPush(ctx context.Context, cmd *Command, reader io.Reader, w io.Writer) error {
	ro := newReporter(w)
	// remove branch or tag
	if cmd.NewRev == plumbing.ZERO_OID {
		if cmd.ReferenceName.IsBranch() && cmd.ReferenceName.BranchName() == r.defaultBranch {
			_ = ro.ng(cmd, "\x1b[31merror\x1b[0m: %s%s", cmd.W("refusing to delete the current branch: "), cmd.ReferenceName)
			return ErrReportStarted
		}
		newReference, err := r.mdb.DoReferenceUpdate(ctx, &database.Command{
			ReferenceName: cmd.ReferenceName,
			NewRev:        cmd.NewRev,
			OldRev:        cmd.OldRev,
			RID:           cmd.RID,
			UID:           cmd.UID,
		})
		if database.IsErrAlreadyLocked(err) {
			_ = ro.ng(cmd, cmd.W("reference is already locked: %s"), cmd.ReferenceName)
			return ErrReportStarted
		}
		if err != nil {
			_ = ro.ng(cmd, cmd.W("update reference error: %v"), err)
			return ErrReportStarted
		}
		_ = ro.ok(cmd, newReference.Hash)
		return nil
	}
	recvObjects, err := r.odb.Unpack(ctx, reader, &odb.OStats{M: cmd.M, B: cmd.B}, func(ctx context.Context, quarantineDir string, o *odb.Objects) error {
		if err := ro.EncodeString("unpack ok"); err != nil {
			_ = ro.close()
			return ErrReportStarted
		}
		qr, err := NewQR(r.odb, quarantineDir)
		if err != nil {
			_ = ro.ng(cmd, cmd.W("check integrity error: %v"), err)
			return err
		}
		defer qr.Close() // nolint

		if err = qr.checkIntegrity(ctx, cmd, ro); err != nil {
			_ = ro.close()
			return ErrReportStarted
		}
		if qr.forcePush && cmd.OldRev != plumbing.ZERO_OID {
			logrus.Infof("Force push, oldRev %s --> newRev %s", cmd.OldRev, cmd.NewRev)
		}
		return nil
	})
	if err != nil {
		return err
	}
	logrus.Infof("objects %d", len(recvObjects.Commits))
	defer ro.close() // nolint
	if err := r.odb.Reload(); err != nil {
		_ = ro.ng(cmd, "reload odb error: %v", err)
		return err
	}
	var g errgroup.Group
	g.Go(func() error {
		if err := r.odb.BatchObjects(ctx, recvObjects.Objects, 50); err != nil {
			logrus.Errorf("batch upload blobs error: %v", err)
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := r.odb.BatchMetaObjects(ctx, recvObjects.MetaObjects); err != nil {
			logrus.Errorf("batch encode metadata objects error: %v", err)
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := r.odb.BatchTrees(ctx, recvObjects.Trees); err != nil {
			logrus.Errorf("batch encode trees error: %v", err)
			return err
		}
		return nil
	})
	g.Go(func() error {
		if err := r.odb.BatchCommits(ctx, recvObjects.Commits); err != nil {
			logrus.Errorf("batch encode commits error: %v", err)
			return err
		}
		return nil
	})
	if err := g.Wait(); err != nil {
		_ = ro.ng(cmd, "store object error: %v", err)
		return ErrReportStarted
	}
	_ = ro.status("%s", cmd.W("objects verified")) //nolint:govet
	change := &database.Command{
		ReferenceName: cmd.ReferenceName,
		NewRev:        cmd.NewRev,
		OldRev:        cmd.OldRev,
		RID:           cmd.RID,
		UID:           cmd.UID,
	}
	if cmd.ReferenceName.IsTag() {
		if to, err := r.odb.Tag(ctx, plumbing.NewHash(cmd.NewRev)); err == nil {
			message, _ := to.Extract()
			change.Subject, change.Description = messageSplit(message)
		}
	}
	newReference, err := r.mdb.DoReferenceUpdate(ctx, change)
	if database.IsErrAlreadyLocked(err) {
		_ = ro.ng(cmd, cmd.W("reference is already locked: %s"), cmd.ReferenceName)
		return ErrReportStarted
	}
	if err != nil {
		_ = ro.ng(cmd, cmd.W("update reference error: %v"), err)
		return ErrReportStarted
	}
	_ = ro.ok(cmd, newReference.Hash)
	return nil
}
