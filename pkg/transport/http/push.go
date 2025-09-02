// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/pkg/transport"
)

func (c *client) Push(ctx context.Context, r io.Reader, cmd *transport.Command) (rc transport.SessionReader, err error) {
	pushURL := c.baseURL.JoinPath("reference", string(cmd.Refname))
	var req *http.Request
	if req, err = c.newRequest(ctx, "POST", pushURL.String(), r); err != nil {
		return nil, fmt.Errorf("new request error: %v", err)
	}
	req.Header.Set(ZETA_COMMAND_OLDREV, cmd.OldRev)
	req.Header.Set(ZETA_COMMAND_NEWREV, cmd.NewRev)
	req.Header.Set(ZETA_OBJECTS_STATS, fmt.Sprintf("m-%d;b-%d", cmd.Metadata, cmd.Objects))
	req.Header.Set("Accept", ZETA_MIME_REPORT_RESULT)
	if len(cmd.PushOptions) != 0 {
		req.Header.Set(ZETA_PUSH_OPTION_COUNT, strconv.Itoa(len(cmd.PushOptions)))
		for i, o := range cmd.PushOptions {
			req.Header.Set(fmt.Sprintf("%s%d", ZETA_PUSH_OPTION_PREFIX, i), o)
		}
	}
	var resp *http.Response
	if resp, err = c.Do(req); err != nil {
		return nil, fmt.Errorf("do request error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close() // nolint
		return nil, parseError(resp)
	}
	if contentType := resp.Header.Get("Content-Type"); contentType != ZETA_MIME_REPORT_RESULT {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("unsupported content-type: %s", contentType)
	}
	return &sessionReader{
		Reader: resp.Body,
		Closer: resp.Body,
	}, nil
}

func (c *client) BatchCheck(ctx context.Context, refname plumbing.ReferenceName, haveObjects []*transport.HaveObject) ([]*transport.HaveObject, error) {
	trace.DbgPrint("check %d large objects", len(haveObjects))
	var b bytes.Buffer
	if err := json.NewEncoder(&b).Encode(&transport.BatchRequest{
		Objects: haveObjects,
	}); err != nil {
		return nil, err
	}
	batchURL := c.baseURL.JoinPath("reference", string(refname), "objects/batch").String()
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
	var response transport.BatchResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}
	return response.Objects, nil
}

func (c *client) PutObject(ctx context.Context, refname plumbing.ReferenceName, oid plumbing.Hash, r io.Reader, size int64) error {
	req, err := c.newRequest(ctx, "PUT", c.baseURL.JoinPath("reference", string(refname), "objects", oid.String()).String(), r)
	if err != nil {
		return fmt.Errorf("new request error: %v", err)
	}
	sizeS := strconv.FormatInt(size, 10)
	req.Header.Set("Accept", ZETA_MIME_JSON_METADATA)
	req.Header.Set("Content-Length", sizeS)
	req.Header.Set(ZETA_COMPRESSED_SIZE, sizeS)
	resp, err := c.Do(req)
	if err != nil {
		return fmt.Errorf("do request error: %v", err)
	}
	defer resp.Body.Close() // nolint
	if resp.StatusCode != http.StatusOK {
		return parseError(resp)
	}
	return nil
}
