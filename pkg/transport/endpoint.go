// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"bytes"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

var (
	isSchemeRegExp = regexp.MustCompile(`^[^:]+://`)

	// Ref: https://github.com/git/git/blob/master/Documentation/urls.txt#L37
	scpLikeUrlRegExp = regexp.MustCompile(`^(?:(?P<user>[^@]+)@)?(?P<host>[^:\s]+):(?:(?P<port>[0-9]{1,5}):)?(?P<path>[^\\].*)$`)
)

func isValidSchemeChar(c rune) bool {
	return unicode.IsLetter(c) || unicode.IsDigit(c) || c == '+' || c == '-' || c == '.'
}

func hasScheme(u string) bool {
	for i, c := range u {
		if c == ':' && i+2 < len(u) && u[i+1] == '/' && u[i+2] == '/' {
			return true
		}
		if !isValidSchemeChar(c) {
			break
		}
	}
	return false
}

// MatchesScheme returns true if the given string matches a URL-like
// format scheme.
func MatchesScheme(url string) bool {
	return isSchemeRegExp.MatchString(url)
}

// MatchesScpLike returns true if the given string matches an SCP-like
// format scheme.
func MatchesScpLike(url string) bool {
	return scpLikeUrlRegExp.MatchString(url)
}

// FindScpLikeComponents returns the user, host, port and path of the
// given SCP-like URL.
func FindScpLikeComponents(url string) (user, host, port, path string) {
	m := scpLikeUrlRegExp.FindStringSubmatch(url)
	return m[1], m[2], m[3], m[4]
}

// IsLocalEndpoint returns true if the given URL string specifies a
// local file endpoint.  For example, on a Linux machine,
// `/home/user/src/go-git` would match as a local endpoint, but
// `https://github.com/src-d/go-git` would not.
func IsLocalEndpoint(url string) bool {
	return !MatchesScheme(url) && !MatchesScpLike(url)
}

func IsRemoteEndpoint(url string) bool {
	return MatchesScheme(url) || MatchesScpLike(url)
}

func parseSCPLike(endpoint string) (*Endpoint, bool) {
	if MatchesScheme(endpoint) || !MatchesScpLike(endpoint) {
		return nil, false
	}

	user, host, portStr, path := FindScpLikeComponents(endpoint)
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 22
	}

	return &Endpoint{
		Protocol: "ssh",
		User:     user,
		Host:     host,
		Port:     port,
		Path:     path,
		raw:      endpoint,
	}, true
}

// Endpoint represents a Git URL in any supported protocol.
type Endpoint struct {
	// Protocol is the protocol of the endpoint (e.g. git, https, file).
	Protocol string
	// User is the user.
	User string
	// Password is the password.
	Password string
	// Host is the host.
	Host string
	// Port is the port to connect, if 0 the default port for the given protocol
	// wil be used.
	Port int
	// Path is the repository path.
	Path string
	// Base URL only http/https
	Base *url.URL
	// InsecureSkipTLS skips ssl verify if protocol is https
	InsecureSkipTLS bool
	// ExtraHeader extra header
	ExtraHeader map[string]string
	// ExtraEnv extra env
	ExtraEnv map[string]string
	// raw endpoint: only scp like url --> zeta@domain.com:namespace/repo
	raw string
}

type Options struct {
	InsecureSkipTLS bool
	ExtraHeader     []string
	ExtraEnv        []string
}

func (opts *Options) parseExtraHeader() map[string]string {
	m := make(map[string]string)
	for _, h := range opts.ExtraHeader {
		k, v, ok := strings.Cut(h, ":")
		if !ok {
			continue
		}
		m[strings.ToLower(k)] = strings.TrimLeftFunc(v, unicode.IsSpace)
	}
	return m
}

func (opts *Options) parseExtraEnv() map[string]string {
	m := make(map[string]string)
	for _, e := range opts.ExtraEnv {
		k, v, ok := strings.Cut(e, "=")
		if !ok {
			continue
		}
		m[k] = v
	}
	return m
}

func NewEndpoint(endpoint string, opts *Options) (*Endpoint, error) {
	if e, ok := parseSCPLike(endpoint); ok {
		return e, nil
	}

	// if e, ok := parseFile(endpoint); ok {
	// 	return e, nil
	// }

	return parseURL(endpoint, opts)
}

func getPort(u *url.URL) int {
	p := u.Port()
	if p == "" {
		return 0
	}

	i, err := strconv.Atoi(p)
	if err != nil {
		return 0
	}

	return i
}

func getPath(u *url.URL) string {
	res := u.Path
	if u.RawQuery != "" {
		res += "?" + u.RawQuery
	}

	if u.Fragment != "" {
		res += "#" + u.Fragment
	}

	return res
}

func parseURL(endpoint string, opts *Options) (*Endpoint, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if !u.IsAbs() {
		return nil, fmt.Errorf("invalid endpoint: %s", endpoint)
	}

	var user, pass string
	if u.User != nil {
		user = u.User.Username()
		pass, _ = u.User.Password()
	}

	host := u.Hostname()
	if strings.Contains(host, ":") {
		// IPv6 address
		host = "[" + host + "]"
	}

	e := &Endpoint{
		Protocol: u.Scheme,
		User:     user,
		Password: pass,
		Host:     host,
		Port:     getPort(u),
		Path:     getPath(u),
		Base:     u,
	}
	if opts != nil {
		e.InsecureSkipTLS = opts.InsecureSkipTLS
		e.ExtraHeader = opts.parseExtraHeader()
		e.ExtraEnv = opts.parseExtraEnv()
	}
	return e, nil
}

var defaultPorts = map[string]int{
	"http":  80,
	"https": 443,
	"ssh":   22,
}

// String returns a string representation of the zeta URL.
func (u *Endpoint) String() string {
	if len(u.raw) != 0 {
		return u.raw
	}
	var buf bytes.Buffer
	if u.Protocol != "" {
		buf.WriteString(u.Protocol)
		buf.WriteByte(':')
	}

	if u.Protocol != "" || u.Host != "" || u.User != "" || u.Password != "" {
		buf.WriteString("//")

		if u.User != "" || u.Password != "" {
			buf.WriteString(url.PathEscape(u.User))
			if u.Password != "" {
				buf.WriteByte(':')
				buf.WriteString(url.PathEscape(u.Password))
			}

			buf.WriteByte('@')
		}

		if u.Host != "" {
			buf.WriteString(u.Host)

			if u.Port != 0 {
				port, ok := defaultPorts[strings.ToLower(u.Protocol)]
				if !ok || ok && port != u.Port {
					fmt.Fprintf(&buf, ":%d", u.Port)
				}
			}
		}
	}

	if u.Path != "" && u.Path[0] != '/' && u.Host != "" {
		buf.WriteByte('/')
	}

	buf.WriteString(u.Path)
	return buf.String()
}
