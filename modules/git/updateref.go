// Copyright (c) 2016-present GitLab Inc.
// SPDX-License-Identifier: MIT
package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/antgroup/hugescm/modules/command"
)

var (
	errClosed = errors.New("closed")
)

// state represents a possible state the updater can be in.
type state string

const (
	// stateIdle means the updater is ready for a new transaction to start.
	stateIdle state = "idle"
	// stateStarted means the updater has an open transaction and accepts
	// new reference changes.
	stateStarted state = "started"
	// statePrepared means the updater has prepared a transaction and no longer
	// accepts reference changes until the current transaction is committed and
	// a new one started.
	statePrepared state = "prepared"
)

type RefUpdater struct {
	cmd       *command.Command
	closeErr  error
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	reader    *bufio.Reader
	stderr    *bytes.Buffer
	shaFormat HashFormat

	ctx context.Context
	// state tracks the current state of the updater to ensure correct calling semantics.
	state state
}

func NewRefUpdater(ctx context.Context, repoPath string, environ []string, noDeref bool) (*RefUpdater, error) {
	shaFormat := HashFormatOK(repoPath)
	psArgs := []string{"update-ref", "-z", "--stdin"}
	if noDeref {
		psArgs = append(psArgs, "--no-deref")
	}
	// repoPath, environ
	var stderr bytes.Buffer
	cmd := command.NewFromOptions(ctx,
		&command.RunOpts{
			Environ:  environ,
			RepoPath: repoPath,
			Stderr:   &stderr,
		}, "git", psArgs...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return nil, err
	}
	u := &RefUpdater{
		cmd:       cmd,
		stdout:    stdout,
		stdin:     stdin,
		stderr:    &stderr,
		reader:    bufio.NewReader(stdout),
		shaFormat: shaFormat,
		ctx:       ctx,
		state:     stateIdle,
	}
	return u, nil
}

// expectState returns an error and closes the updater if it is not in the expected state.
func (u *RefUpdater) expectState(expected state) error {
	if u.closeErr != nil {
		return u.closeErr
	}

	if err := u.checkState(expected); err != nil {
		return u.closeWithError(err)
	}

	return nil
}

// checkState returns an error if the updater is not in the expected state.
func (u *RefUpdater) checkState(expected state) error {
	if u.state != expected {
		return fmt.Errorf("expected state %q but it was %q", expected, u.state)
	}

	return nil
}

// Start begins a new reference transaction. The reference changes are not performed until Commit
// is explicitly called.
func (u *RefUpdater) Start() error {
	if err := u.expectState(stateIdle); err != nil {
		return err
	}

	u.state = stateStarted

	return u.setState("start")
}

// Update commands the reference to be updated to point at the object ID specified in newOID. If
// newOID is the zero OID, then the branch will be deleted. If oldOID is a non-empty string, then
// the reference will only be updated if its current value matches the old value. If the old value
// is the zero OID, then the branch must not exist.
//
// A reference transaction must be started before calling Update.
func (u *RefUpdater) Update(reference ReferenceName, newRev, oldRev string) error {
	if err := u.expectState(stateStarted); err != nil {
		return err
	}

	return u.write("update %s\x00%s\x00%s\x00", reference.String(), newRev, oldRev)
}

// UpdateSymbolicReference is used to do a symbolic reference update. We can potentially provide the oldTarget
// or the oldOID.
func (u *RefUpdater) UpdateSymbolicReference(reference, newTarget ReferenceName) error {
	if err := u.expectState(stateStarted); err != nil {
		return err
	}

	return u.write("symref-update %s\x00%s\x00\x00\x00", reference.String(), newTarget.String())
}

// Create commands the reference to be created with the given object ID. The ref must not exist.
//
// A reference transaction must be started before calling Create.
func (u *RefUpdater) Create(reference ReferenceName, oid string) error {
	return u.Update(reference, oid, u.shaFormat.ZeroOID())
}

// Delete commands the reference to be removed from the repository. This command will ignore any old
// state of the reference and just force-remove it.
//
// A reference transaction must be started before calling Delete.
func (u *RefUpdater) Delete(reference ReferenceName) error {
	return u.Update(reference, u.shaFormat.ZeroOID(), "")
}

// Prepare prepares the reference transaction by locking all references and determining their
// current values. The updates are not yet committed and will be rolled back in case there is no
// call to `Commit()`. This call is optional.
func (u *RefUpdater) Prepare() error {
	if err := u.expectState(stateStarted); err != nil {
		return err
	}

	u.state = statePrepared

	return u.setState("prepare")
}

// Commit applies the commands specified in other calls to the Updater. Commit finishes the
// reference transaction and another one must be started before further changes can be staged.
func (u *RefUpdater) Commit() error {
	// Commit can be called without preparing the transactions.
	if err := u.checkState(statePrepared); err != nil {
		if err := u.expectState(stateStarted); err != nil {
			return err
		}
	}

	u.state = stateIdle

	if err := u.setState("commit"); err != nil {
		return err
	}

	return nil
}

// Close closes the updater and aborts a possible open transaction. No changes will be written
// to disk, all lockfiles will be cleaned up and the process will exit.
func (u *RefUpdater) Close() error {
	return u.closeWithError(nil)
}

func (u *RefUpdater) teardown() {
	if u.stdin != nil {
		_ = u.stdin.Close()
	}
	if u.stdout != nil {
		_ = u.stdout.Close()
	}
}

func (u *RefUpdater) closeWithError(closeErr error) error {
	if u.closeErr != nil {
		return u.closeErr
	}
	u.teardown() // close input/output
	if err := u.cmd.Wait(); err != nil {
		u.closeErr = fmt.Errorf("close error: %w stderr: %s", err, u.stderr.String())
		return err
	}
	if u.ctx.Err() != nil {
		u.closeErr = u.ctx.Err()
		return u.closeErr
	}

	if closeErr != nil {
		u.closeErr = closeErr
		return closeErr
	}

	u.closeErr = errClosed
	return nil
}

func (u *RefUpdater) write(format string, args ...any) error {
	if _, err := fmt.Fprintf(u.stdin, format, args...); err != nil {
		return u.closeWithError(err)
	}

	return nil
}

func (u *RefUpdater) setState(state string) error {
	if err := u.write("%s\x00", state); err != nil {
		return err
	}

	// For each state-changing command, git-update-ref(1) will report successful execution via
	// "<command>: ok" lines printed to its stdout. Ideally, we should thus verify here whether
	// the command was successfully executed by checking for exactly this line, otherwise we
	// cannot be sure whether the command has correctly been processed by Git or if an error was
	// raised.
	line, err := u.reader.ReadString('\n')
	if err != nil {
		return u.closeWithError(fmt.Errorf("state update to %q failed: %w", state, err))
	}

	if line != fmt.Sprintf("%s: ok\n", state) {
		return u.closeWithError(fmt.Errorf("state update to %q not successful: expected ok, got %q", state, line))
	}

	return nil
}
