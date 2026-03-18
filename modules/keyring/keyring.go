package keyring

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
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
// This is used to configure credential storage backend on platforms that support multiple backends.
// On macOS and Windows, the default backend is always used unless explicitly overridden.
type Option func(*Options)

// Options holds configuration for keyring operations.
type Options struct {
	// Storage specifies the credential storage backend.
	//
	// Platform-specific behavior:
	//   - macOS: Default uses Security.framework; "security" uses /usr/bin/security CLI; "file" uses encrypted file
	//   - Windows: Default uses Credential Manager; "file" uses encrypted file
	//   - Linux: Default is "none"; "secret-service" uses Secret Service API; "file" uses encrypted file
	Storage string

	// EncryptionKey specifies the key for encrypting credentials in file storage.
	// Required when Storage="file".
	EncryptionKey string

	// StoragePath specifies the path for encrypted credential file.
	// Only used when Storage="file".
	// Default: ~/.config/zeta/credentials
	StoragePath string
}

// WithStorage sets the credential storage backend.
// Valid values depend on the platform:
//   - macOS: "security" (/usr/bin/security CLI), "file"
//   - Windows: "file"
//   - Linux: "secret-service", "file"
func WithStorage(storage string) Option {
	return func(o *Options) {
		o.Storage = storage
	}
}

// WithEncryptionKey sets the encryption key for file-based credential storage.
// Required when Storage="file".
func WithEncryptionKey(key string) Option {
	return func(o *Options) {
		o.EncryptionKey = key
	}
}

// WithStoragePath sets the path for encrypted credential file.
// Only used when Storage="file".
func WithStoragePath(path string) Option {
	return func(o *Options) {
		o.StoragePath = path
	}
}

// applyOptions applies the given options to an Options struct.
func applyOptions(opts ...Option) *Options {
	o := &Options{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// resolveStorageMode determines the storage mode from options.
// Default is "auto" which uses platform-specific default storage.
func resolveStorageMode(opts ...Option) string {
	options := applyOptions(opts...)
	mode := strings.ToLower(strings.TrimSpace(options.Storage))
	if mode == "" {
		return storageAuto
	}
	return mode
}

// Storage mode constants used across platforms
const (
	storageAuto = "auto"
	storageFile = "file"
	storageNone = "none"
)

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
