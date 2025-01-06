// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/antgroup/hugescm/modules/streamio"
	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/transport/proxy"
	"github.com/antgroup/hugescm/pkg/version"
)

const (
	Z1 = "Z1" // Zeta Protocol Version
	// Zeta HTTP Header
	AUTHORIZATION           = "Authorization"
	ZETA_PROTOCOL           = "Zeta-Protocol"
	ZETA_COMMAND_OLDREV     = "X-Zeta-Command-OldRev"
	ZETA_COMMAND_NEWREV     = "X-Zeta-Command-NewRev"
	ZETA_TERMINAL           = "X-Zeta-Terminal"
	ZETA_OBJECTS_STATS      = "X-Zeta-Objects-Stats"
	ZETA_COMPRESSED_SIZE    = "X-Zeta-Compressed-Size"
	ZETA_PUSH_OPTION_COUNT  = "X-Zeta-Push-Option-Count"
	ZETA_PUSH_OPTION_PREFIX = "X-Zeta-Push-Option-"
	// ZETA Protocol Content Type
	ZETA_MIME_BLOB              = "application/x-zeta-blob"
	ZETA_MIME_BLOBS             = "application/x-zeta-blobs"
	ZETA_MIME_MULTI_OBJECTS     = "application/x-zeta-multi-objects"
	ZETA_MIME_METADATA          = "application/x-zeta-metadata"
	ZETA_MIME_COMPRESS_METADATA = "application/x-zeta-compress-metadata"
	ZETA_MIME_REPORT_RESULT     = "application/x-zeta-report-result"
	ZETA_MIME_JSON_METADATA     = "application/vnd.zeta+json"
)

var (
	W      = tr.W
	dialer = net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}
)

type client struct {
	*http.Client
	baseURL      *url.URL
	extraHeader  map[string]string
	credentials  *Credentials // User Credentials
	tokenPayload *transport.SASPayload
	userAgent    string
	language     string
	termEnv      string
	verbose      bool
}

func (c *client) hasAuth() bool {
	if _, ok := c.extraHeader["authorization"]; ok {
		return true
	}
	return false
}

func cloneURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	u2 := new(url.URL)
	*u2 = *u
	if u.User != nil {
		u2.User = new(url.Userinfo)
		*u2.User = *u.User
	}
	return u2
}

func newClient(ctx context.Context, endpoint *transport.Endpoint, operation transport.Operation, verbose bool) (*client, error) {
	if endpoint == nil || endpoint.Base == nil {
		return nil, errors.New("bad endpoint")
	}
	base := cloneURL(endpoint.Base)
	c := &client{
		Client: &http.Client{
			Transport: &http.Transport{
				Proxy:                 proxy.ProxyFromEnvironment,
				DialContext:           dialer.DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: endpoint.InsecureSkipTLS,
				},
			},
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		baseURL:     base,
		extraHeader: endpoint.ExtraHeader,
		userAgent:   "Zeta/" + version.GetVersion(),
		language:    tr.Language(),
		termEnv:     os.Getenv("TERM"),
		verbose:     verbose,
	}
	if c.extraHeader == nil {
		c.extraHeader = make(map[string]string)
	}
	if c.hasAuth() {
		return c, nil
	}
	if err := c.authorize(ctx, operation); err != nil {
		return nil, err
	}
	return c, nil
}

// NewTransport: new transport
func NewTransport(ctx context.Context, endpoint *transport.Endpoint, operation transport.Operation, verbose bool) (transport.Transport, error) {
	return newClient(ctx, endpoint, operation, verbose)
}

func (c *client) authGuard(req *http.Request) {
	if c.hasAuth() {
		return
	}
	// If the repository allows anonymous access, the SAS interface can return an empty response and function normally regardless of which branch is hit.
	if c.tokenPayload == nil || c.tokenPayload.IsExpired() {
		if c.credentials != nil {
			req.Header.Set(AUTHORIZATION, c.credentials.BasicAuth())
		}
		return
	}
	for k, v := range c.tokenPayload.Header {
		req.Header.Set(k, v)
	}
}

func (c *client) newRequest(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, err
	}
	c.authGuard(req)
	for h, v := range c.extraHeader {
		req.Header.Set(h, v)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept-Language", c.language)
	req.Header.Set(ZETA_PROTOCOL, Z1)
	if len(c.termEnv) != 0 {
		req.Header.Set(ZETA_TERMINAL, c.termEnv)
	}
	c.DbgPrint("%s %s", method, url)
	if c.verbose {
		return wrapRequest(req), nil
	}
	return req, nil
}

type ErrorCode struct {
	status  int    `json:"-"`
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

func (e *ErrorCode) Error() string {
	return e.Message
}

func (e *ErrorCode) Status() int {
	return e.status
}

func checkUnauthorized(err error, showErr bool) bool {
	if err == nil {
		return false
	}
	ec, ok := err.(*ErrorCode)
	if !ok {
		return false
	}
	if ec.status == http.StatusUnauthorized {
		if showErr {
			fmt.Fprintf(os.Stderr, "auth: \x1b[31m%s\x1b[0m\n", ec.Message)
		}
		return true
	}
	return false
}

func parseError(resp *http.Response) error {
	contentType := resp.Header.Get("Content-Type")
	m, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		return fmt.Errorf("parse mime '%s' error: %v", contentType, err)
	}
	if strings.HasPrefix(m, "application/json") {
		ec := &ErrorCode{status: resp.StatusCode}
		if err := json.NewDecoder(resp.Body).Decode(ec); err != nil {
			return fmt.Errorf("decode json error: %w", err)
		}
		ec.Message = strings.TrimRightFunc(ec.Message, unicode.IsSpace)
		return ec
	}
	b, err := streamio.ReadMax(resp.Body, 1024)
	if err != nil {
		return &ErrorCode{status: resp.StatusCode, Message: fmt.Sprintf("%d %s\nError: %v", resp.StatusCode, resp.Status, err)}
	}
	body := strings.TrimRightFunc(string(b), unicode.IsSpace)
	return &ErrorCode{status: resp.StatusCode, Message: fmt.Sprintf("%s\n%s", resp.Status, body)}
}

type sessionReader struct {
	io.Reader
	io.Closer
}

func (r *sessionReader) LastError() error {
	return nil
}
