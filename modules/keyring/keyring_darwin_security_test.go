//go:build darwin

package keyring

import (
	"context"
	"testing"
)

func TestSecurityCLI(t *testing.T) {
	ctx := context.Background()

	// Test credential
	cred := &Cred{
		Server:   "test.example.com",
		Protocol: "https",
		UserName: "testuser",
		Password: "testpassword123",
	}

	// Test 1: Store credential
	t.Run("Store", func(t *testing.T) {
		err := storeToSecurityCLI(ctx, cred)
		if err != nil {
			t.Fatalf("storeToSecurityCLI failed: %v", err)
		}
		t.Log("Store: OK")
	})

	// Test 2: Get credential
	t.Run("Get", func(t *testing.T) {
		got, err := getFromSecurityCLI(ctx, cred)
		if err != nil {
			t.Fatalf("getFromSecurityCLI failed: %v", err)
		}
		if got.UserName != cred.UserName {
			t.Errorf("username mismatch: got %q, want %q", got.UserName, cred.UserName)
		}
		if got.Password != cred.Password {
			t.Errorf("password mismatch: got %q, want %q", got.Password, cred.Password)
		}
		t.Logf("Get: OK - username=%q, password=%q", got.UserName, got.Password)
	})

	// Test 3: Erase credential
	t.Run("Erase", func(t *testing.T) {
		err := eraseFromSecurityCLI(ctx, cred)
		if err != nil {
			t.Fatalf("eraseFromSecurityCLI failed: %v", err)
		}
		t.Log("Erase: OK")
	})

	// Test 4: Get after erase (should return ErrNotFound)
	t.Run("GetAfterErase", func(t *testing.T) {
		_, err := getFromSecurityCLI(ctx, cred)
		if err != ErrNotFound {
			t.Errorf("expected ErrNotFound, got: %v", err)
		} else {
			t.Log("GetAfterErase: OK - returned ErrNotFound as expected")
		}
	})

	// Test 5: Erase again (should be idempotent, return nil)
	t.Run("EraseAgain", func(t *testing.T) {
		err := eraseFromSecurityCLI(ctx, cred)
		if err != nil {
			t.Errorf("eraseFromSecurityCLI should be idempotent, got error: %v", err)
		} else {
			t.Log("EraseAgain: OK - idempotent deletion returned nil")
		}
	})
}

func TestSecurityCLIWithHTTP(t *testing.T) {
	ctx := context.Background()

	cred := &Cred{
		Server:   "http.example.com",
		Protocol: "http",
		UserName: "httpuser",
		Password: "httppassword",
	}

	// Store and verify
	err := storeToSecurityCLI(ctx, cred)
	if err != nil {
		t.Fatalf("storeToSecurityCLI failed: %v", err)
	}
	t.Log("Store (HTTP): OK")

	got, err := getFromSecurityCLI(ctx, cred)
	if err != nil {
		t.Fatalf("getFromSecurityCLI failed: %v", err)
	}
	t.Logf("Get (HTTP): OK - username=%q, password=%q", got.UserName, got.Password)

	// Cleanup
	_ = eraseFromSecurityCLI(ctx, cred)
}

func TestSecurityCLIWithSpecialChars(t *testing.T) {
	ctx := context.Background()

	cred := &Cred{
		Server:   "special.example.com",
		Protocol: "https",
		UserName: "user with spaces",
		Password: "p@ssw0rd!#$%^&*()",
	}

	// Store and verify
	err := storeToSecurityCLI(ctx, cred)
	if err != nil {
		t.Fatalf("storeToSecurityCLI failed: %v", err)
	}
	t.Log("Store (special chars): OK")

	got, err := getFromSecurityCLI(ctx, cred)
	if err != nil {
		t.Fatalf("getFromSecurityCLI failed: %v", err)
	}
	if got.UserName != cred.UserName {
		t.Errorf("username mismatch: got %q, want %q", got.UserName, cred.UserName)
	}
	if got.Password != cred.Password {
		t.Errorf("password mismatch: got %q, want %q", got.Password, cred.Password)
	}
	t.Logf("Get (special chars): OK - username=%q, password=%q", got.UserName, got.Password)

	// Cleanup
	_ = eraseFromSecurityCLI(ctx, cred)
}
