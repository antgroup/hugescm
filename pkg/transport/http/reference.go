// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/pkg/transport"
)

func (c *client) FetchReference(ctx context.Context, refname plumbing.ReferenceName) (*transport.Reference, error) {
	if len(refname) == 0 {
		refname = plumbing.HEAD
	}
	req, err := c.newRequest(ctx, "GET", c.baseURL.JoinPath("reference", string(refname)).String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", ZETA_MIME_JSON_METADATA)
	resp, err := c.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		break
	case http.StatusNotFound:
		return nil, transport.ErrReferenceNotExist
	default:
		return nil, parseError(resp)
	}
	var ref transport.Reference
	if err := json.NewDecoder(resp.Body).Decode(&ref); err != nil {
		return nil, fmt.Errorf("decode reference response error: %w", err)
	}
	return &ref, nil
}
