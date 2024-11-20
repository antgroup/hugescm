// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptrace"
	"os"
	"strings"
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

func wrapRequest(req *http.Request) *http.Request {
	trace := &httptrace.ClientTrace{
		DNSStart: func(di httptrace.DNSStartInfo) {
			fmt.Fprintf(os.Stderr, "\x1b[33mResolve %s\x1b[0m", di.Host)
		},
		DNSDone: func(dnsinfo httptrace.DNSDoneInfo) {
			if dnsinfo.Err == nil {
				fmt.Fprintf(os.Stderr, "\x1b[33m to %s\x1b[0m\n", flatAddress(dnsinfo.Addrs))
			}
		},
		ConnectDone: func(network, addr string, err error) {
			if err == nil {
				fmt.Fprintf(os.Stderr, "\x1b[33mConnecting to %s connected\x1b[0m\n", addr)
			}
		},
		TLSHandshakeDone: func(state tls.ConnectionState, err error) {
			if err == nil {
				fmt.Fprintf(os.Stderr, "\x1b[33mSSL connection using %s/%s\x1b[0m\n", tlsVersionName(state.Version), tls.CipherSuiteName(state.CipherSuite))
			}
		},
		WroteHeaderField: func(key string, value []string) {
			for _, v := range value {
				fmt.Fprintf(os.Stderr, "\x1b[33m< \x1b[36m%s: \x1b[38;2;254;225;64m%s\x1b[0m\n", key, redactedHeader(key, v))
			}
		},
	}
	return req.WithContext(httptrace.WithClientTrace(req.Context(), trace))
}

func (c *client) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if c.verbose {
		fmt.Fprintf(os.Stderr, "\x1b[38;2;249;212;35m%s %s\x1b[0m\n", resp.Proto, resp.Status)
		for key, value := range resp.Header {
			for _, v := range value {
				fmt.Fprintf(os.Stderr, "\x1b[33m> \x1b[34m%s: \x1b[38;2;254;225;64m%s\x1b[0m\n", key, redactedHeader(key, v))
			}
		}
	}
	return resp, nil
}

func (c *downloader) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.Client.Do(req)
	if err != nil {
		return nil, err
	}
	if c.verbose {
		fmt.Fprintf(os.Stderr, "\x1b[38;2;249;212;35m%s %s\x1b[0m\n", resp.Proto, resp.Status)
		for key, value := range resp.Header {
			for _, v := range value {
				fmt.Fprintf(os.Stderr, "\x1b[33m> \x1b[34m%s: \x1b[38;2;254;225;64m%s\x1b[0m\n", key, redactedHeader(key, v))
			}
		}
	}
	return resp, nil
}

func (c *client) DbgPrint(format string, args ...any) {
	if !c.verbose {
		return
	}
	message := fmt.Sprintf(format, args...)
	var buffer bytes.Buffer
	for _, s := range strings.Split(message, "\n") {
		_, _ = buffer.WriteString("\x1b[38;2;254;225;64m* ")
		_, _ = buffer.WriteString(s)
		_, _ = buffer.WriteString("\x1b[0m\n")
	}
	_, _ = os.Stderr.Write(buffer.Bytes())
}

func (c *downloader) DbgPrint(format string, args ...any) {
	if !c.verbose {
		return
	}
	message := fmt.Sprintf(format, args...)
	var buffer bytes.Buffer
	for _, s := range strings.Split(message, "\n") {
		_, _ = buffer.WriteString("\x1b[38;2;254;225;64m* ")
		_, _ = buffer.WriteString(s)
		_, _ = buffer.WriteString("\x1b[0m\n")
	}
	_, _ = os.Stderr.Write(buffer.Bytes())
}
