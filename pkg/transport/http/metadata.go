// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/klauspost/compress/zstd"
)

func sparseDirsGenReader(sparseDirs []string) io.Reader {
	var b strings.Builder
	var total int
	for _, s := range sparseDirs {
		total += len(s) + 1
	}
	b.Grow(total)
	for _, s := range sparseDirs {
		_, _ = b.WriteString(s)
		_ = b.WriteByte('\n')
	}
	return strings.NewReader(b.String())
}

type decompressReader struct {
	io.Reader
	closer []io.Closer
}

func (r decompressReader) Close() error {
	for _, c := range r.closer {
		_ = c.Close()
	}
	return nil
}

func newDecompressReader(rc io.ReadCloser, h http.Header) (io.ReadCloser, error) {
	switch contentType := h.Get("Content-Type"); contentType {
	case ZETA_MIME_METADATA:
		return &decompressReader{
			Reader: rc,
			closer: []io.Closer{rc},
		}, nil
	case ZETA_MIME_COMPRESS_METADATA:
		zr, err := zstd.NewReader(rc)
		if err != nil {
			return nil, err
		}
		return &decompressReader{
			Reader: zr,
			closer: []io.Closer{zr.IOReadCloser(), rc},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported content-type: '%s'", contentType)
	}
}

func (c *client) FetchMetadata(ctx context.Context, target plumbing.Hash, opts *transport.MetadataOptions) (transport.SessionReader, error) {
	var body io.Reader
	method := http.MethodGet
	if len(opts.SparseDirs) != 0 {
		method = http.MethodPost
		body = sparseDirsGenReader(opts.SparseDirs)
	}
	metadataURL := c.baseURL.JoinPath("metadata", target.String())
	q := make(url.Values)
	if !opts.Have.IsZero() {
		q.Set("have", opts.Have.String())
	}
	if !opts.DeepenFrom.IsZero() {
		q.Set("deepen-from", opts.DeepenFrom.String())
	}
	q.Set("deepen", strconv.Itoa(opts.Deepen))
	if opts.Depth >= 0 {
		q.Set("depth", strconv.Itoa(opts.Depth))
	}
	if len(q) > 0 {
		metadataURL.RawQuery = q.Encode()
	}
	req, err := c.newRequest(ctx, method, metadataURL.String(), body)
	if err != nil {
		return nil, err
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", ZETA_MIME_MULTI_OBJECTS)
	}
	req.Header.Set("Accept", ZETA_MIME_COMPRESS_METADATA)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode > 299 || resp.StatusCode < 200 {
		defer resp.Body.Close()
		return nil, parseError(resp)
	}
	rc, err := newDecompressReader(resp.Body, resp.Header)
	if err != nil {
		_ = resp.Body.Close()
		return nil, err
	}
	return &sessionReader{
		Reader: rc,
		Closer: rc,
	}, nil
}
