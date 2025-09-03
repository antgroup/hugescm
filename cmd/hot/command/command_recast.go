// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package command

type Recast struct {
	Revision string `arg:"" optional:"" name:"revision-range" help:"Revision range"`
	Depth    int
	To       string
}

func (c *Recast) Run(g *Globals) error {

	return nil
}
