// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/antgroup/hugescm/pkg/tr"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/transport/proxy"
	"github.com/antgroup/hugescm/pkg/version"
)

// Error represents an error in an operation with OSS.
type Error struct {
	StatusCode int    // HTTP status code (200, 403, ...)
	Code       string // OSS error code ("UnsupportedOperation", ...)
	Message    string // The human-oriented error message
	BucketName string
	RequestId  string
	HostId     string
}

func (e *Error) Error() string {
	return fmt.Sprintf("Aliyun API Error: RequestId: %s Status Code: %d Code: %s Message: %s", e.RequestId, e.StatusCode, e.Code, e.Message)
}

// ServiceError contains fields of the error response from Oss Service REST API.
type ServiceError struct {
	XMLName    xml.Name `xml:"Error"`
	Code       string   `xml:"Code"`      // The error code returned from OSS to the caller
	Message    string   `xml:"Message"`   // The detail error message from OSS
	RequestID  string   `xml:"RequestId"` // The UUID used to uniquely identify the request
	HostID     string   `xml:"HostId"`    // The OSS server cluster's Id
	Endpoint   string   `xml:"Endpoint"`
	Ec         string   `xml:"EC"`
	RawMessage string   // The raw messages from OSS
	StatusCode int      // HTTP status code

}

// Error implements interface error
func (e *ServiceError) Error() string {
	errorMessage := fmt.Sprintf("oss: service returned error: StatusCode=%d, ErrorCode=%s, ErrorMessage=\"%s\", RequestId=%s", e.StatusCode, e.Code, e.Message, e.RequestID)
	if len(e.Endpoint) > 0 {
		errorMessage = fmt.Sprintf("%s, Endpoint=%s", errorMessage, e.Endpoint)
	}
	if len(e.Ec) > 0 {
		errorMessage = fmt.Sprintf("%s, Ec=%s", errorMessage, e.Ec)
	}
	return errorMessage
}

func readResponseBody(resp *http.Response) ([]byte, error) {
	out, err := io.ReadAll(resp.Body)
	if err == io.EOF {
		err = nil
	}
	return out, err
}

func serviceErrFromXML(body []byte, statusCode int, requestID string) (*ServiceError, error) {
	var se ServiceError

	if err := xml.Unmarshal(body, &se); err != nil {
		return nil, err
	}

	se.StatusCode = statusCode
	se.RequestID = requestID
	se.RawMessage = string(body)
	return &se, nil
}

func readOssError(resp *http.Response) error {
	if resp.StatusCode >= 400 && resp.StatusCode <= 505 {
		b, err := readResponseBody(resp)
		if err != nil {
			return err
		}
		if len(b) == 0 && len(resp.Header.Get("X-Oss-Err")) != 0 {
			if e, err := base64.StdEncoding.DecodeString(resp.Header.Get("X-Oss-Err")); err == nil {
				b = e
			}
		}
		if len(b) > 0 {
			if se, err := serviceErrFromXML(b, resp.StatusCode, resp.Header.Get("X-Oss-Request-Id")); err == nil {
				return se
			}
		}
	}
	return &ServiceError{StatusCode: resp.StatusCode, RequestID: resp.Header.Get("X-Oss-Request-Id"), Ec: resp.Header.Get("X-Oss-Ec")}
}

type Downloader interface {
	Download(ctx context.Context, o *transport.Representation, offset int64) (transport.SizeReader, error)
}

type downloader struct {
	*http.Client
	userAgent string
	proxyURL  string
	language  string
	termEnv   string
	verbose   bool
}

func proxyFromURL(externalProxyURL string) func(req *http.Request) (*url.URL, error) {
	if len(externalProxyURL) != 0 {
		// cfg := &httpproxy.Config{
		// 	HTTPProxy:  externalProxyURL,
		// 	HTTPSProxy: externalProxyURL,
		// }
		// proxyFuncValue := cfg.ProxyFunc()
		// return func(req *http.Request) (*url.URL, error) {
		// 	fmt.Fprintf(os.Stderr, "proxy: %s\n", req.URL)
		// 	return proxyFuncValue(req.URL)
		// }
		if proxyURL, err := url.Parse(externalProxyURL); err == nil {
			return http.ProxyURL(proxyURL)
		}
	}
	return proxy.ProxyFromEnvironment
}

func NewDownloader(verbose bool, insecure bool, proxyURL string) Downloader {
	return &downloader{
		Client: &http.Client{
			Transport: &http.Transport{
				Proxy:                 proxyFromURL(proxyURL),
				DialContext:           dialer.DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: insecure,
				},
			},
		},
		userAgent: "Zeta/" + version.GetVersion(),
		language:  tr.Language(),
		proxyURL:  proxyURL,
		termEnv:   os.Getenv("TERM"),
		verbose:   verbose,
	}
}

func readError(resp *http.Response) error {
	m, _, err := mime.ParseMediaType(resp.Header.Get("Content-Type"))
	if err != nil {
		return fmt.Errorf("response parse mime error: %v", err)
	}
	if m == "application/json" {
		ec := &ErrorCode{status: resp.StatusCode}
		if err := json.NewDecoder(resp.Body).Decode(ec); err != nil {
			return fmt.Errorf("json decode error: %v", err)
		}
		return ec
	}
	return readOssError(resp)
}

func (c *downloader) Download(ctx context.Context, o *transport.Representation, offset int64) (transport.SizeReader, error) {
	if _, err := url.Parse(o.Href); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", o.Href, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept-Language", c.language)
	if len(c.termEnv) != 0 {
		req.Header.Set(ZETA_TERMINAL, c.termEnv)
	}
	c.DbgPrint("GET %s", o.Href)
	if c.verbose {
		req = wrapRequest(req)
	}
	if o.Header != nil {
		for k, v := range o.Header {
			req.Header.Set(k, v)
		}
	}
	if offset > 0 {
		// https://developer.mozilla.org/zh-CN/docs/Web/HTTP/Headers/Range
		// Range: <unit>=<range-start>-
		// Range: <unit>=<range-start>-<range-end>
		// Range: <unit>=<range-start>-<range-end>, <range-start>-<range-end>
		// Range: <unit>=<range-start>-<range-end>, <range-start>-<range-end>, <range-start>-<range-end>
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	resp, err := c.Do(req)
	if err != nil {
		fmt.Fprintf(os.Stderr, "download blobs error: %v\n", err)
		return nil, err
	}
	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		defer resp.Body.Close()
		return nil, readError(resp)
	}
	sr := &sizeReader{
		Reader: resp.Body,
		closer: resp.Body,
		size:   o.CompressedSize,
	}
	if resp.StatusCode != http.StatusPartialContent {
		// Don't close resp.Body, need to return SizeReader
		return sr, nil
	}
	sr.offset, err = func() (int64, error) {
		rangeHdr := resp.Header.Get("Content-Range")
		if rangeHdr == "" {
			return 0, errors.New("missing Content-Range header in response")
		}
		match := rangeRegex.FindStringSubmatch(rangeHdr)
		if len(match) == 0 {
			return 0, fmt.Errorf("badly formatted Content-Range header: %q", rangeHdr)
		}
		contentStart, _ := strconv.ParseInt(match[1], 10, 64)
		if contentStart != offset {
			return 0, fmt.Errorf("error: Content-Range start byte incorrect: %s expected %d", match[1], offset)
		}
		return contentStart, nil
	}()
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return sr, nil
}
