// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/term"
)

const (
	zetaAuthPrefix = "Zeta Credential="
)

var (
	redactedHeaderKey = map[string]bool{
		"x-zeta-authorization": true,
		"authorization":        true,
	}
)

func redactedHeader(name string, v string) string {
	if !redactedHeaderKey[strings.ToLower(name)] {
		return v
	}
	if strings.HasPrefix(v, zetaAuthPrefix) {
		return zetaAuthPrefix + "<redacted>"
	}
	if prefix, _, ok := strings.Cut(v, " "); ok {
		return prefix + " <redacted>"
	}
	return "<redacted>"
}

func tlsVersionName(i uint16) string {
	switch int(i) {
	case tls.VersionTLS13:
		return "TLSv1.3"
	case tls.VersionTLS12:
		return "TLSv1.2"
	case tls.VersionTLS11:
		return "TLSv1.1"
	}
	return "unsupported version"
}

func flatAddress(addrs []net.IPAddr) string {
	if len(addrs) == 0 {
		return "<empty>"
	}
	ss := make([]string, 0, len(addrs))
	for _, s := range addrs {
		ss = append(ss, s.String())
	}
	return strings.Join(ss, "|")
}

func wroteHeaderField(key string, value []string) {
	switch term.StderrLevel {
	case term.Level256:
		for _, v := range value {
			_, _ = fmt.Fprintf(os.Stderr, "\x1b[33m< \x1b[36m%s: \x1b[33m%s\x1b[0m\n", key, redactedHeader(key, v))
		}
	case term.Level16M:
		for _, v := range value {
			_, _ = fmt.Fprintf(os.Stderr, "\x1b[33m< \x1b[38;2;86;182;194m%s: \x1b[38;2;254;225;64m%s\x1b[0m\n", key, redactedHeader(key, v))
		}
	default:
		for _, v := range value {
			_, _ = fmt.Fprintf(os.Stderr, "< %s: %s\n", key, redactedHeader(key, v))
		}
	}
}

func wrapRequest(req *http.Request) *http.Request {
	trace := &httptrace.ClientTrace{
		DNSStart: func(di httptrace.DNSStartInfo) {
			_, _ = term.Fprintf(os.Stderr, "\x1b[33mResolve %s\x1b[0m", di.Host)
		},
		DNSDone: func(dnsInfo httptrace.DNSDoneInfo) {
			if dnsInfo.Err == nil {
				_, _ = term.Fprintf(os.Stderr, "\x1b[33m to %s\x1b[0m\n", flatAddress(dnsInfo.Addrs))
			}
		},
		ConnectDone: func(network, addr string, err error) {
			if err == nil {
				_, _ = term.Fprintf(os.Stderr, "\x1b[33mConnecting to %s connected\x1b[0m\n", addr)
			}
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			if err == nil {
				_, _ = term.Fprintf(os.Stderr, "\x1b[33mSSL connection using %s/%s\x1b[0m\n", tlsVersionName(state.Version), tls.CipherSuiteName(state.CipherSuite))
			}
		},
		WroteHeaderField: wroteHeaderField,
	}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
}

func traceResponse(resp *http.Response) {
	switch term.StderrLevel {
	case term.Level256:
		fmt.Fprintf(os.Stderr, "\x1b[33m%s %s\x1b[0m\n", resp.Proto, resp.Status)
		for key, value := range resp.Header {
			for _, v := range value {
				fmt.Fprintf(os.Stderr, "\x1b[33m> \x1b[34m%s: \x1b[33m%s\x1b[0m\n", key, redactedHeader(key, v))
			}
		}
	case term.Level16M:
		fmt.Fprintf(os.Stderr, "\x1b[38;2;249;212;35m%s %s\x1b[0m\n", resp.Proto, resp.Status)
		for key, value := range resp.Header {
			for _, v := range value {
				fmt.Fprintf(os.Stderr, "\x1b[33m> \x1b[38;2;97;175;239m%s: \x1b[38;2;254;225;64m%s\x1b[0m\n", key, redactedHeader(key, v))
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "%s %s\n", resp.Proto, resp.Status)
		for key, value := range resp.Header {
			for _, v := range value {
				fmt.Fprintf(os.Stderr, "> %s: %s\n", key, redactedHeader(key, v))
			}
		}
	}
}

func (c *client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if c.verbose {
		traceResponse(resp)
	}
	return resp, nil
}

func (c *downloader) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if c.verbose {
		traceResponse(resp)
	}
	return resp, nil
}
