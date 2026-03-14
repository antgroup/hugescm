package keyring

import (
	"errors"
	"net"
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
	// ErrStorageDisabled indicates that credential storage is disabled.
	ErrStorageDisabled = errors.New("credential storage is disabled")
	// ErrNoEncryptionKey indicates that encryption key is required but not provided.
	ErrNoEncryptionKey = errors.New("encryption key is required for file storage")
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

// Option is a functional option for configuring keyring behavior.
// This is primarily used on Linux to configure credential storage backend.
// On macOS and Windows, the native keychain is always used and options are ignored.
type Option func(*Options)

// Options holds configuration for keyring operations.
type Options struct {
	// Storage specifies the credential storage backend (Linux only).
	// Options: "auto", "secret-service", "file", "none"
	// Default: "none" (credentials are not stored by default on Linux)
	Storage string

	// EncryptionKey specifies the key for encrypting credentials in file storage.
	// Required when Storage="file".
	EncryptionKey string

	// StoragePath specifies the path for encrypted credential file.
	// Only used when Storage="file".
	// Default: ~/.config/zeta/credentials.enc
	StoragePath string
}

// WithStorage sets the credential storage backend.
// This option is only effective on Linux systems.
func WithStorage(storage string) Option {
	return func(o *Options) {
		o.Storage = storage
	}
}

// WithEncryptionKey sets the encryption key for file-based credential storage.
// This option is only effective on Linux systems with Storage="file".
func WithEncryptionKey(key string) Option {
	return func(o *Options) {
		o.EncryptionKey = key
	}
}

// WithStoragePath sets the path for encrypted credential file.
// This option is only effective on Linux systems with Storage="file".
func WithStoragePath(path string) Option {
	return func(o *Options) {
		o.StoragePath = path
	}
}

// NewCredFromURL creates a Cred from a URL, extracting protocol, server, and port.
// If the URL specifies a default port for the protocol (e.g., 443 for https),
// the port is not stored to ensure consistent credential lookup.
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

	// Extract port, but skip default ports to ensure consistent credential lookup
	if u.Port() != "" {
		if port, err := strconv.Atoi(u.Port()); err == nil {
			if defaultPorts[u.Scheme] != port {
				cred.Port = port
			}
		}
	}
	return cred
}

// defaultPorts maps protocols to their default ports.
var defaultPorts = map[string]int{
	"http":  80,
	"https": 443,
	"ftp":   21,
	"ssh":   22,
}

// buildTargetName constructs a unique target name for storing credentials.
// Format: "zeta+<protocol>://<server>[:<port>][<path>]"
func buildTargetName(cred *Cred) string {
	protocol := cred.Protocol
	if protocol == "" {
		protocol = "https"
	}

	var host string
	if cred.Port != 0 {
		host = net.JoinHostPort(cred.Server, strconv.Itoa(cred.Port))
	} else {
		host = cred.Server
	}

	u := &url.URL{
		Scheme: "zeta+" + protocol,
		Host:   host,
		Path:   cred.Path,
	}
	return u.String()
}

// parseTargetName parses a target name back into a Cred struct
// Format: "zeta+<protocol>://<server>[:<port>][<path>]"
func parseTargetName(target string) *Cred {
	u, err := url.Parse(target)
	if err != nil {
		return &Cred{Server: target}
	}

	// Extract protocol from "zeta+<protocol>" scheme
	scheme := u.Scheme
	protocol, found := parseSchemePrefix(scheme, "zeta+")
	if !found {
		return &Cred{Server: target}
	}

	cred := &Cred{
		Protocol: protocol,
		Server:   u.Hostname(),
		Path:     u.Path,
	}

	if u.Port() != "" {
		if port, err := strconv.Atoi(u.Port()); err == nil {
			cred.Port = port
		}
	}
	return cred
}

// parseSchemePrefix parses a scheme like "zeta+https" and returns the protocol part
func parseSchemePrefix(scheme, prefix string) (protocol string, found bool) {
	if len(scheme) <= len(prefix) {
		return "", false
	}
	if scheme[:len(prefix)] != prefix {
		return "", false
	}
	return scheme[len(prefix):], true
}
