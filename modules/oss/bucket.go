// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package oss

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// Stat
// https://www.alibabacloud.com/help/zh/oss/developer-reference/headobject
func (b *bucket) Stat(ctx context.Context, resourcePath string) (*Stat, error) {
	u := &url.URL{
		Scheme: b.scheme,
		Host:   b.bucketEndpoint,
		Path:   resourcePath,
	}
	req, err := http.NewRequestWithContext(ctx, "HEAD", u.String(), nil)
	if err != nil {
		return nil, err
	}
	resource := b.getResourceV2(resourcePath, "")
	b.signature(req, resource)
	resp, err := b.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, os.ErrNotExist
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, readOssError(resp)
	}
	size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64)
	if err != nil {
		return nil, err
	}
	return &Stat{Size: size, Crc64: resp.Header.Get("X-Oss-Hash-Crc64ecma"), Mime: resp.Header.Get("Content-Type")}, nil
}

func (b *bucket) checkSize(ctx context.Context, resourcePath string, resp *http.Response) (int64, error) {
	if rangeHdr := resp.Header.Get("Content-Range"); len(rangeHdr) != 0 {
		if size, err := parseSizeFromRange(rangeHdr); err == nil {
			return size, nil
		}
		si, err := b.Stat(ctx, resourcePath)
		if err != nil {
			return 0, err
		}
		return si.Size, nil
	}
	if size, err := strconv.ParseInt(resp.Header.Get("Content-Length"), 10, 64); err == nil {
		return size, nil
	}
	si, err := b.Stat(ctx, resourcePath)
	if err != nil {
		return -1, err
	}
	return si.Size, nil
}

// Open:
// https://www.alibabacloud.com/help/zh/oss/developer-reference/getobject
func (b *bucket) Open(ctx context.Context, resourcePath string, start, length int64) (RangeReader, error) {
	u := &url.URL{
		Scheme: b.scheme,
		Host:   b.bucketEndpoint,
		Path:   resourcePath,
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	// https://developer.mozilla.org/en-US/docs/Web/HTTP/Headers/Range
	switch {
	case start < 0:
		req.Header.Set("Range", fmt.Sprintf("bytes=%d", start))
	case start >= 0 && length > 0:
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, start+length-1))
	case start > 0:
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", start))
	default: // NO RANGE
	}
	resource := b.getResourceV2(resourcePath, "")
	b.signature(req, resource)
	resp, err := b.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode == http.StatusNotFound {
		_ = resp.Body.Close()
		return nil, os.ErrNotExist
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		defer resp.Body.Close()
		return nil, readOssError(resp)
	}
	size, err := b.checkSize(ctx, resourcePath, resp)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return NewRangeReader(resp.Body, size, resp.Header.Get("Content-Range")), nil
}

func (b *bucket) Put(ctx context.Context, resourcePath string, r io.Reader, mime string) error {
	u := &url.URL{
		Scheme: b.scheme,
		Host:   b.bucketEndpoint,
		Path:   resourcePath,
	}
	req, err := http.NewRequestWithContext(ctx, "PUT", u.String(), r)
	if err != nil {
		return err
	}
	if len(mime) != 0 {
		req.Header.Set("Content-Type", mime)
	}
	resource := b.getResourceV2(resourcePath, "")
	b.signature(req, resource)
	resp, err := b.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return os.ErrNotExist
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return readOssError(resp)
	}
	return nil
}

/*
import base64
import hmac
import hashlib
import urllib
h = hmac.new(accesskey,
             "GET\n\n\n1141889120\n%2Fexamplebucket%2Foss-api.pdf?\
             &x-oss-ac-forward-allow=true\
             &x-oss-ac-source-ip=127.0.0.1\
             &x-oss-ac-subnet-mask=32\
             &x-oss-signature-version=OSS2",
             hashlib.sha256)
Signature = base64.encodestring(h.digest()).strip()
*/

func (b *bucket) Share(ctx context.Context, resourcePath string, expiresAt int64) string {
	u := &url.URL{
		Scheme: b.sharedScheme,
		Host:   b.sharedBucketEndpoint,
		Path:   resourcePath,
	}
	if expiresAt <= 0 {
		expiresAt = time.Now().Add(time.Hour).Unix()
	}
	//
	headers := make(map[string]string)
	headers["x-oss-expires"] = strconv.FormatInt(expiresAt, 10)
	headers["x-oss-access-key-id"] = b.accessKeyID
	headers["x-oss-signature-version"] = "OSS2"

	hs := newHeaderSorter(headers)
	hs.Sort()
	var q strings.Builder
	for i := range hs.Keys {
		if i != 0 {
			_, _ = q.WriteString("&")
		}
		_, _ = q.WriteString(hs.Keys[i])
		_ = q.WriteByte('=')
		_, _ = q.WriteString(url.QueryEscape(hs.Vals[i]))
	}
	qs := q.String()
	canonicalizedResource := b.getResourceV2(resourcePath, qs)
	// V2:
	// Please note that the v2 signature document given in the OSS documentation is wrong. Please analyze the open source code to implement it.
	// 		signStr = req.Method + "\n" + contentMd5 + "\n" + contentType + "\n" + date + "\n" + canonicalizedOSSHeaders + strings.Join(additionalList, ";") + "\n" + canonicalizedResource
	signedText := fmt.Sprintf("GET\n\n\n%d\n\n%s", expiresAt, canonicalizedResource)
	h := hmac.New(sha256.New, []byte(b.accessKeySecret))
	_, _ = h.Write([]byte(signedText))
	signed := base64.StdEncoding.EncodeToString(h.Sum(nil))
	u.RawQuery = qs + "&x-oss-signature=" + url.QueryEscape(signed)
	return u.String()
}
