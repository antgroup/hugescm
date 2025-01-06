// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/antgroup/hugescm/modules/env"
	"github.com/antgroup/hugescm/modules/keyring"
	"github.com/antgroup/hugescm/modules/survey"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/version"
)

const (
	PseudoUserName = "ZetaPseudo"
)

var (
	ErrNoValidCredentials = errors.New("no valid credentials")
	ErrRedirect           = errors.New("redirect")
)

type Credentials struct {
	UserName string
	Password string
}

// See 2 (end of page 4) https://www.ietf.org/rfc/rfc2617.txt
// "To receive authorization, the client sends the userid and password,
// separated by a single colon (":") character, within a base64
// encoded string in the credentials."
// It is not meant to be urlencoded.
func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (cred *Credentials) BasicAuth() string {
	if cred == nil {
		return ""
	}
	// Compatible with the old Keyring mechanism
	if cred.UserName == PseudoUserName && strings.HasPrefix(cred.Password, "Basic ") {
		return cred.Password
	}
	return "Basic " + basicAuth(cred.UserName, cred.Password)
}

func (c *client) readCredentialsFromNetrc() (*Credentials, error) {
	netrc, _ := readNetrc()
	if len(netrc) == 0 {
		return nil, nil
	}
	host := c.baseURL.Host
	if host == "" {
		host = c.baseURL.Hostname()
	}
	for _, n := range netrc {
		if n.machine == host {
			c.DbgPrint("Got credentials from netrc, username: %s", n.login)
			return &Credentials{UserName: n.login, Password: n.password}, nil
		}
	}
	return nil, os.ErrNotExist
}

func (c *client) baseCredentailsURL() string {
	u := cloneURL(c.baseURL)
	u.Path = ""
	u.User = nil
	return u.String()
}

func (c *client) readCredentials0(ctx context.Context) (*Credentials, error) {
	if cred, err := keyring.Find(ctx, c.baseCredentailsURL()); err == nil {
		c.DbgPrint("Got credentials from keyring, username: %s", cred.UserName)
		return &Credentials{UserName: cred.UserName, Password: cred.Password}, nil
	}
	return c.readCredentialsFromNetrc()
}

func (c *client) storeCredentials(ctx context.Context, cred *Credentials) error {
	return keyring.Store(ctx, c.baseCredentailsURL(), &keyring.Cred{UserName: cred.UserName, Password: cred.Password})
}

func (c *client) credentialAskOne() (*Credentials, error) {
	if !env.ZETA_TERMINAL_PROMPT.SimpleAtob(true) {
		c.DbgPrint("terminal prompts disabled")
		return nil, errors.New("terminal prompts disabled")
	}
	var username string
	if c.baseURL.User != nil {
		username = c.baseURL.User.Username()
	} else {
		pu := &survey.Input{
			Message: fmt.Sprintf("Username for '%s://%s':", c.baseURL.Scheme, c.baseURL.Host),
		}
		if err := survey.AskOne(pu, &username); err != nil {
			return nil, err
		}
	}
	var password string
	prompt := &survey.Password{
		Message: fmt.Sprintf("Password for '%s://%s@%s':", c.baseURL.Scheme, url.PathEscape(username), c.baseURL.Host),
	}
	if err := survey.AskOne(prompt, &password); err != nil {
		return nil, err
	}
	return &Credentials{UserName: username, Password: password}, nil
}

func (c *client) readCredentials(ctx context.Context) (*Credentials, error) {
	if u := c.baseURL.User; u != nil {
		if password, ok := u.Password(); ok {
			c.DbgPrint("Got credentials from userinfo, username: %s", u.Username())
			return &Credentials{UserName: u.Username(), Password: password}, nil
		}
	}
	return c.readCredentials0(ctx)
}

func (c *client) authorize(ctx context.Context, operation transport.Operation) error {
	cred, err := c.readCredentials(ctx)
	if err == nil {
		ok, err := c.checkAuth(ctx, cred, operation)
		if ok {
			c.credentials = cred
			return nil
		}
		showErr := cred != nil
		if !checkUnauthorized(err, showErr) {
			return err
		}
	}
	for i := 0; i < 3; i++ {
		cred, err := c.credentialAskOne()
		if err != nil {
			return err
		}
		ok, err := c.checkAuth(ctx, cred, operation)
		if ok {
			_ = c.storeCredentials(ctx, cred)
			c.credentials = cred
			return nil
		}
		if !checkUnauthorized(err, true) {
			return err
		}
	}
	fmt.Fprintln(os.Stderr, W("Too many failed attempts"))
	return ErrNoValidCredentials
}

func (c *client) checkAuthRedirect(ctx context.Context, cred *Credentials, operation transport.Operation) (*http.Response, error) {
	var br bytes.Buffer
	if err := json.NewEncoder(&br).Encode(&transport.SASHandeshake{
		Operation: operation,
		Version:   version.GetVersion(),
	}); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL.JoinPath("authorization").String(), bytes.NewReader(br.Bytes()))
	if err != nil {
		return nil, err
	}
	if c.verbose {
		req = wrapRequest(req)
	}
	c.DbgPrint("%s %s", req.Method, req.URL.String())
	if cred != nil {
		req.Header.Add(AUTHORIZATION, cred.BasicAuth())
	}
	for h, v := range c.extraHeader {
		req.Header.Set(h, v)
	}
	req.Header.Add(ZETA_PROTOCOL, Z1)
	req.Header.Add("User-Agent", c.userAgent)
	req.Header.Set("Accept-Language", c.language)
	if len(c.termEnv) != 0 {
		req.Header.Set(ZETA_TERMINAL, c.termEnv)
	}
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
		defer resp.Body.Close()
		location, err := resp.Location()
		if err != nil {
			return nil, err
		}
		newBaseURL := cloneURL(location)
		newBaseURL.Path = c.baseURL.Path
		c.baseURL = newBaseURL
		fmt.Fprintf(os.Stderr, W("Redirecting %s\n"), newBaseURL.String())
		return nil, ErrRedirect
	}
	return resp, nil
}

func remoteNotify(notice string) {
	lines := strings.Split(notice, "\n")
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "remote notice: %s\n", line)
	}
}

func (c *client) checkAuth(ctx context.Context, cred *Credentials, operation transport.Operation) (bool, error) {
	var resp *http.Response
	var err error
	for i := 0; i < 10; i++ {
		resp, err = c.checkAuthRedirect(ctx, cred, operation)
		if err != ErrRedirect {
			break
		}
	}
	if err == ErrRedirect {
		return false, &ErrorCode{Code: http.StatusFound, Message: W("too many redirects")}
	}
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		var sa transport.SASPayload
		if err := json.NewDecoder(resp.Body).Decode(&sa); err != nil {
			return false, fmt.Errorf("decode json error: %v", err)
		}
		if len(sa.Notice) != 0 {
			remoteNotify(sa.Notice)
		}
		c.tokenPayload = &sa
		return true, nil
	}
	return false, parseError(resp)
}
