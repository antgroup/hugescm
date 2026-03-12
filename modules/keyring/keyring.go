package keyring

import (
	"errors"
	"net/url"
	"strconv"
)

var (
	// ErrNotFound is the expected error if the secret isn't found in the keyring.
	ErrNotFound = errors.New("secret not found in keyring")
	// ErrSetDataTooBig is returned if Set was called with too much data.
	// On macOS: The combination of service, username & password should not exceed ~3000 bytes
	// On Windows: The service is limited to 32KiB while the password is limited to 2560 bytes
	// On Linux/Unix: There is no theoretical limit but performance suffers with big values (>100KiB)
	ErrSetDataTooBig = errors.New("data passed to Set was too big")
)

// Cred represents credentials for a server.
// This design follows git-credential-osxkeychain pattern where
// credentials are identified by (protocol, host, username) tuple.
type Cred struct {
	UserName string
	Password string
	// Protocol specifies protocol type (http, https, imap, smtp, ftp, etc.)
	Protocol string
	// Server specifies the server name or IP address (without port)
	Server string
	// Path specifies the path component (optional, for some protocols)
	Path string
	// Port specifies the port number (optional, 0 means use default)
	Port int
}

// NewCredFromURL creates a Cred from a URL, extracting protocol, server, and port.
func NewCredFromURL(targetURL string) *Cred {
	u, err := url.Parse(targetURL)
	if err != nil {
		return &Cred{
			Server: targetURL,
		}
	}

	cred := &Cred{
		Protocol: u.Scheme,
		Server:   u.Hostname(),
		Path:     u.Path,
	}

	// Extract port
	if u.Port() != "" {
		if port, err := strconv.Atoi(u.Port()); err == nil {
			cred.Port = port
		}
	}
	return cred
}
