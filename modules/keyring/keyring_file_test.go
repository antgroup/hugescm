//go:build darwin || dragonfly || freebsd || linux || netbsd || openbsd || windows

package keyring

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDeriveOrValidateKeyRawStringFallback(t *testing.T) {
	key, err := deriveOrValidateKey("password")
	if err != nil {
		t.Fatalf("deriveOrValidateKey returned error: %v", err)
	}

	expected := hashKey("password")
	if !bytes.Equal(key, expected) {
		t.Fatalf("unexpected key derivation for raw string")
	}
}

func TestCredentialStorageEraseIsIdempotent(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "credentials")
	storage, err := newCredentialStorage("my-secret-key", storagePath)
	if err != nil {
		t.Fatalf("newCredentialStorage failed: %v", err)
	}

	cred := &Cred{Protocol: "https", Server: "example.com", UserName: "u", Password: "p"}

	if err := storage.Erase(t.Context(), cred); err != nil {
		t.Fatalf("Erase on non-existing credential should succeed, got: %v", err)
	}

	if err := storage.Store(t.Context(), cred); err != nil {
		t.Fatalf("Store failed: %v", err)
	}
	if err := storage.Erase(t.Context(), cred); err != nil {
		t.Fatalf("Erase failed: %v", err)
	}
	if _, err := storage.Get(t.Context(), cred); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound after erase, got: %v", err)
	}
}

func TestAcquireFileLockTimeout(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "credentials")
	storage, err := newCredentialStorage("my-secret-key", storagePath)
	if err != nil {
		t.Fatalf("newCredentialStorage failed: %v", err)
	}

	lockPath := storagePath + ".lock"
	if err := os.WriteFile(lockPath, []byte("busy"), 0600); err != nil {
		t.Fatalf("failed to create lock file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err = storage.acquireFileLock(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected context deadline exceeded, got: %v", err)
	}
}

func TestAcquireFileLockBreaksStaleLock(t *testing.T) {
	storagePath := filepath.Join(t.TempDir(), "credentials")
	storage, err := newCredentialStorage("my-secret-key", storagePath)
	if err != nil {
		t.Fatalf("newCredentialStorage failed: %v", err)
	}

	lockPath := storagePath + ".lock"
	if err := os.WriteFile(lockPath, []byte("stale"), 0600); err != nil {
		t.Fatalf("failed to create stale lock file: %v", err)
	}

	old := time.Now().Add(-lockStaleAfter - time.Second)
	if err := os.Chtimes(lockPath, old, old); err != nil {
		t.Fatalf("failed to set stale lock file mtime: %v", err)
	}

	release, err := storage.acquireFileLock(t.Context())
	if err != nil {
		t.Fatalf("acquireFileLock failed for stale lock: %v", err)
	}
	release()

	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Fatalf("expected lock file to be removed, got: %v", err)
	}
}
