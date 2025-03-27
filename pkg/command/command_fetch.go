// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"

	"github.com/antgroup/hugescm/pkg/zeta"
)

// In the design of HugeSCM, we have abandoned the philosophy of git where the retrieval of repository data should be minimalistic, that is, to fetch only what is needed. Therefore,
// when implementing the fetch feature, it's important to adhere to the principle that zeta fetch will not support fetching all data at once,
// but will only support fetching specific reference metadata and particular objects.
type Fetch struct {
	Name      string `arg:"" optional:"" name:"name" help:"Reference or commit to be downloaded"`
	Unshallow bool   `name:"unshallow" help:"Get complete history"`
	Tag       bool   `name:"tag" short:"t" help:"Download tags instead of branches only when refname is incomplete"` //
	Limit     int64  `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
	Force     bool   `name:"force" short:"f" help:"Override reference update check"`
}

const (
	fetchSummaryFormat = `%szeta fetch [reference] [--unshallow] [--tag] [--skip-larges]`
)

func (c *Fetch) Summary() string {
	return fmt.Sprintf(fetchSummaryFormat, W("Usage: "))
}

func (c *Fetch) Run(g *Globals) error {
	r, err := zeta.Open(context.Background(), &zeta.OpenOptions{
		Worktree: g.CWD,
		Values:   g.Values,
		Verbose:  g.Verbose,
	})
	if err != nil {
		return err
	}
	defer r.Close() // nolint
	_, err = r.DoFetch(context.Background(), &zeta.DoFetchOptions{
		Name:        c.Name,
		Unshallow:   c.Unshallow,
		Limit:       c.Limit,
		Tag:         c.Tag,
		FetchAlways: true,
	})
	return err
}
