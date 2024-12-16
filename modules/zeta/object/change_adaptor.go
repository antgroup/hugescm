// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"errors"
	"fmt"

	"github.com/antgroup/hugescm/modules/merkletrie"
	"github.com/antgroup/hugescm/modules/merkletrie/noder"
)

// The following functions transform changes types form the merkletrie
// package to changes types from this package.

func newChange(c merkletrie.Change) (*Change, error) {
	ret := &Change{}

	var err error
	if ret.From, err = newChangeEntry(c.From); err != nil {
		return nil, fmt.Errorf("from field: %s", err)
	}

	if ret.To, err = newChangeEntry(c.To); err != nil {
		return nil, fmt.Errorf("to field: %s", err)
	}

	return ret, nil
}

func newChangeEntry(p noder.Path) (ChangeEntry, error) {
	if p == nil {
		return empty, nil
	}

	asTreeNoder, ok := p.Last().(*TreeNoder)
	if !ok {
		return ChangeEntry{}, errors.New("cannot transform non-TreeNoders")
	}
	return ChangeEntry{
		Name: p.String(),
		Tree: asTreeNoder.parent,
		TreeEntry: TreeEntry{
			Name: asTreeNoder.name,
			Size: asTreeNoder.size,
			Mode: asTreeNoder.TrueMode(),
			Hash: asTreeNoder.HashRaw(),
		},
	}, nil
}

func newChanges(src merkletrie.Changes) (Changes, error) {
	ret := make(Changes, len(src))
	var err error
	for i, e := range src {
		ret[i], err = newChange(e)
		if err != nil {
			return nil, fmt.Errorf("change #%d: %s", i, err)
		}
	}

	return ret, nil
}
