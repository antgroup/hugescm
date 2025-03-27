// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/pkg/transport"
)

func (c *client) BatchObjects(ctx context.Context, objects []plumbing.Hash) (transport.SessionReader, error) {
	reader := transport.NewObjectsReader(objects)
	defer reader.Close() // nolint
	batchURL := c.baseURL.JoinPath("objects", "batch").String()
	req, err := c.newRequest(ctx, "POST", batchURL, reader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", ZETA_MIME_BLOBS)
	req.Header.Set("Content-Type", ZETA_MIME_MULTI_OBJECTS)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		err = parseError(resp)
		_ = resp.Body.Close()
		return nil, err
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != ZETA_MIME_BLOBS {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unsupported content-type: %s", contentType)
	}
	return &sessionReader{
		Reader: resp.Body,
		Closer: resp.Body,
	}, nil
}

func (c *client) Share(ctx context.Context, wantObjects []*transport.WantObject) ([]*transport.Representation, error) {
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(&transport.BatchShareObjectsRequest{
		Objects: wantObjects,
	}); err != nil {
		return nil, err
	}
	batchURL := c.baseURL.JoinPath("objects", "share").String()
	req, err := c.newRequest(ctx, "POST", batchURL, bytes.NewReader(b.Bytes()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", ZETA_MIME_JSON_METADATA)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close() // nolint
	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		return nil, parseError(resp)
	}
	var response transport.BatchShareObjectsResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	return response.Objects, nil
}

var (
	rangeRegex = regexp.MustCompile(`bytes (\d+)\-.*`)
)

type sizeReader struct {
	io.Reader
	closer io.Closer
	offset int64
	size   int64
}

func (sr *sizeReader) Close() error {
	if sr.closer != nil {
		return sr.closer.Close()
	}
	return nil
}

func (sr *sizeReader) Offset() int64 {
	return sr.offset
}

func (sr *sizeReader) Size() int64 {
	return sr.size
}

func (sr *sizeReader) LastError() error {
	return nil
}

func (c *client) GetObject(ctx context.Context, oid plumbing.Hash, offset int64) (transport.SizeReader, error) {
	downloadURL := c.baseURL.JoinPath("objects", oid.String()).String()
	req, err := c.newRequest(ctx, "GET", downloadURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", ZETA_MIME_BLOB)
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
		return nil, err
	}
	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		defer resp.Body.Close() // nolint
		return nil, parseError(resp)
	}
	sr := &sizeReader{
		Reader: resp.Body,
		closer: resp.Body,
		size:   -1,
	}
	if size, err := strconv.ParseInt(resp.Header.Get(ZETA_COMPRESSED_SIZE), 10, 64); err == nil {
		sr.size = size
	}

	if resp.StatusCode != http.StatusPartialContent {
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
