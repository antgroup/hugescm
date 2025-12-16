// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package transport

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode"
)

var (
	isSchemeRegExp = regexp.MustCompile(`^[^:]+://`)

	// Ref: https://github.com/git/git/blob/master/Documentation/urls.txt#L37
	scpLikeUrlRegExp = regexp.MustCompile(`^(?:(?P<user>[^@]+)@)?(?P<host>[^:\s]+):(?:(?P<port>[0-9]{1,5}):)?(?P<path>[^\\].*)$`)
)

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

func IsRemoteEndpoint(url string) bool {
	return MatchesScheme(url) || MatchesScpLike(url)
}

func parseSCPLike(endpoint string, opts *Options) (*Endpoint, bool) {
	if MatchesScheme(endpoint) || !MatchesScpLike(endpoint) {
		return nil, false
	}

	user, host, port, path := FindScpLikeComponents(endpoint)
	if port != "" {
		host = net.JoinHostPort(host, port)
	}
	e := &Endpoint{
		URL: url.URL{
			Scheme: "ssh",
			User:   url.User(user),
			Host:   host,
			Path:   path,
		},
		origin: endpoint,
	}
	if opts != nil {
		// SSH protocol only support parseExtraEnv
		e.ExtraEnv = opts.parseExtraEnv()
	}
	return e, true
}

// Endpoint represents a zeta URL in any supported protocol.
type Endpoint struct {
	url.URL
	// InsecureSkipTLS skips ssl verify if protocol is https
	InsecureSkipTLS bool
	// ExtraHeader extra header
	ExtraHeader map[string]string
	// ExtraEnv extra env
	ExtraEnv map[string]string
	// origin endpoint: only scp like url --> zeta@domain.com:namespace/repo
	origin string
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
	if e, ok := parseSCPLike(endpoint, opts); ok {
		return e, nil
	}
	return parseURL(endpoint, opts)
}

func parseURL(endpoint string, opts *Options) (*Endpoint, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}

	if !u.IsAbs() {
		return nil, fmt.Errorf("invalid endpoint: %s", endpoint)
	}

	e := &Endpoint{
		URL: *u,
	}
	if opts != nil {
		e.InsecureSkipTLS = opts.InsecureSkipTLS
		e.ExtraHeader = opts.parseExtraHeader()
		e.ExtraEnv = opts.parseExtraEnv()
	}
	return e, nil
}

// String returns a string representation of the zeta URL.
func (u *Endpoint) String() string {
	if len(u.origin) != 0 {
		return u.origin
	}
	return u.URL.String()
}
