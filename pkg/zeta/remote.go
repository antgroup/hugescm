// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"encoding/json"
	"fmt"
	"os"
)

type RemoteInfo struct {
	Remote string `json:"remote"`
}

type ShowRemoteOptions struct {
	JSON bool
}

func (r *Repository) ShowRemote(opts *ShowRemoteOptions) error {
	remote := r.cleanedRemote()
	if opts.JSON {
		return json.NewEncoder(os.Stdout).Encode(RemoteInfo{Remote: remote})
	}
	_, _ = fmt.Fprintf(os.Stdout, "remote: %s\n", remote)
	return nil
}
