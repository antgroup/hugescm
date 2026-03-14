package keyring

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/text/encoding/unicode"
	"golang.org/x/text/transform"
)

const (
	service  = "test-service"
	testuser = "test-user"
	password = "test-password"
)

func TestBuildTargetName(t *testing.T) {
	tests := []struct {
		name     string
		cred     *Cred
		expected string
	}{
		{
			name: "basic https",
			cred: &Cred{
				Protocol: "https",
				Server:   "example.com",
			},
			expected: "zeta+https://example.com",
		},
		{
			name: "with port",
			cred: &Cred{
				Protocol: "https",
				Server:   "example.com",
				Port:     8080,
			},
			expected: "zeta+https://example.com:8080",
		},
		{
			name: "with path",
			cred: &Cred{
				Protocol: "https",
				Server:   "example.com",
				Path:     "/repo",
			},
			expected: "zeta+https://example.com/repo",
		},
		{
			name: "with port and path",
			cred: &Cred{
				Protocol: "https",
				Server:   "example.com",
				Port:     8080,
				Path:     "/repo",
			},
			expected: "zeta+https://example.com:8080/repo",
		},
		{
			name: "empty protocol defaults to https",
			cred: &Cred{
				Server: "example.com",
			},
			expected: "zeta+https://example.com",
		},
		{
			name: "http protocol",
			cred: &Cred{
				Protocol: "http",
				Server:   "example.com",
			},
			expected: "zeta+http://example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildTargetName(tt.cred)
			if result != tt.expected {
				t.Errorf("buildTargetName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestParseTargetName(t *testing.T) {
	tests := []struct {
		name     string
		target   string
		expected *Cred
	}{
		{
			name:   "basic https",
			target: "zeta+https://example.com",
			expected: &Cred{
				Protocol: "https",
				Server:   "example.com",
			},
		},
		{
			name:   "with port",
			target: "zeta+https://example.com:8080",
			expected: &Cred{
				Protocol: "https",
				Server:   "example.com",
				Port:     8080,
			},
		},
		{
			name:   "with path",
			target: "zeta+https://example.com/repo",
			expected: &Cred{
				Protocol: "https",
				Server:   "example.com",
				Path:     "/repo",
			},
		},
		{
			name:   "with port and path",
			target: "zeta+https://example.com:8080/repo",
			expected: &Cred{
				Protocol: "https",
				Server:   "example.com",
				Port:     8080,
				Path:     "/repo",
			},
		},
		{
			name:   "http protocol",
			target: "zeta+http://example.com",
			expected: &Cred{
				Protocol: "http",
				Server:   "example.com",
			},
		},
		{
			name:   "invalid format without zeta prefix",
			target: "example.com",
			expected: &Cred{
				Server: "example.com",
			},
		},
		{
			name:   "invalid format without protocol separator",
			target: "zeta+example.com",
			expected: &Cred{
				Server: "zeta+example.com",
			},
		},
		{
			name:   "ipv6 address",
			target: "zeta+https://[::1]:8080",
			expected: &Cred{
				Protocol: "https",
				Server:   "::1",
				Port:     8080,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTargetName(tt.target)
			if result.Protocol != tt.expected.Protocol {
				t.Errorf("Protocol = %q, want %q", result.Protocol, tt.expected.Protocol)
			}
			if result.Server != tt.expected.Server {
				t.Errorf("Server = %q, want %q", result.Server, tt.expected.Server)
			}
			if result.Port != tt.expected.Port {
				t.Errorf("Port = %d, want %d", result.Port, tt.expected.Port)
			}
			if result.Path != tt.expected.Path {
				t.Errorf("Path = %q, want %q", result.Path, tt.expected.Path)
			}
		})
	}
}

func TestBuildAndParseTargetName(t *testing.T) {
	tests := []struct {
		name string
		cred *Cred
	}{
		{
			name: "basic https",
			cred: &Cred{
				Protocol: "https",
				Server:   "example.com",
			},
		},
		{
			name: "with port",
			cred: &Cred{
				Protocol: "https",
				Server:   "example.com",
				Port:     8080,
			},
		},
		{
			name: "with path",
			cred: &Cred{
				Protocol: "https",
				Server:   "example.com",
				Path:     "/repo/project",
			},
		},
		{
			name: "with port and path",
			cred: &Cred{
				Protocol: "https",
				Server:   "git.example.com",
				Port:     22,
				Path:     "/org/repo.git",
			},
		},
		{
			name: "http protocol",
			cred: &Cred{
				Protocol: "http",
				Server:   "localhost",
				Port:     3000,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := buildTargetName(tt.cred)
			result := parseTargetName(target)

			if result.Protocol != tt.cred.Protocol {
				t.Errorf("Protocol = %q, want %q", result.Protocol, tt.cred.Protocol)
			}
			if result.Server != tt.cred.Server {
				t.Errorf("Server = %q, want %q", result.Server, tt.cred.Server)
			}
			if result.Port != tt.cred.Port {
				t.Errorf("Port = %d, want %d", result.Port, tt.cred.Port)
			}
			if result.Path != tt.cred.Path {
				t.Errorf("Path = %q, want %q", result.Path, tt.cred.Path)
			}
		})
	}
}

// TestStore tests setting a user and password in keyring.
func TestStore(t *testing.T) {
	cred := NewCredFromURL("https://" + service)
	cred.UserName = testuser
	cred.Password = password
	err := Store(t.Context(), cred)
	if err != nil {
		t.Errorf("Should not fail, got: %s", err)
	}
}

func TestEncodePassword(t *testing.T) {
	encoder := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewEncoder()
	encodedCred, _, err := transform.Bytes(encoder, []byte("My Password 你好 🦚"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "my password: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "%x\n", encodedCred)
	dec := unicode.UTF16(unicode.LittleEndian, unicode.IgnoreBOM).NewDecoder()
	password, _, err := transform.Bytes(dec, encodedCred)
	if err != nil {
		fmt.Fprintf(os.Stderr, "my password: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "Password: %v\n", string(password))
}
