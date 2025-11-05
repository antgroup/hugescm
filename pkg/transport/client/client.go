// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package client

import (
	"context"
	"fmt"

	"github.com/antgroup/hugescm/pkg/transport"
	"github.com/antgroup/hugescm/pkg/transport/http"
	"github.com/antgroup/hugescm/pkg/transport/ssh"
)

func NewTransport(ctx context.Context, endpoint *transport.Endpoint, operation transport.Operation, verbose bool) (transport.Transport, error) {
	switch endpoint.Scheme {
	case "http", "https":
		return http.NewTransport(ctx, endpoint, operation, verbose)
	case "ssh":
		return ssh.NewTransport(ctx, endpoint, operation, verbose)
	}
	return nil, fmt.Errorf("unsupported protocol '%s'", endpoint.Scheme)
}
