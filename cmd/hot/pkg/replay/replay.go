// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0
package replay

import (
	"bufio"
	"context"
	"encoding/hex"
	"strings"
	"sync"

	"github.com/antgroup/hugescm/modules/command"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
)

type Replayer struct {
	ctx      context.Context
	repoPath string
	// mu guards entries and commits (see below)
	mu *sync.Mutex
	// entries is a mapping of old tree entries to new (rewritten) ones.
	// Since TreeEntry contains a []byte (and is therefore not a key-able
	// type), a unique TreeEntry -> string function is used for map keys.
	entries map[string]*gitobj.TreeEntry
	// commits is a mapping of old commit SHAs to new ones, where the ASCII
	// hex encoding of the SHA1 values are used as map keys.
	commits map[string][]byte
	// odb is the *ObjectDatabase from which blobs, commits, and trees are
	// loaded from.
	odb         *git.ODB
	stepEnd     int
	stepCurrent int
	verbose     bool
}

func NewReplayer(ctx context.Context, repoPath string, stepEnd int, verbose bool) (*Replayer, error) {
	odb, err := git.NewODB(repoPath, git.HashFormatOK(repoPath))
	if err != nil {
		return nil, err
	}

	return &Replayer{
		ctx:         ctx,
		repoPath:    repoPath,
		mu:          new(sync.Mutex),
		entries:     make(map[string]*gitobj.TreeEntry),
		commits:     map[string][]byte{},
		odb:         odb,
		stepEnd:     stepEnd,
		stepCurrent: 1,
		verbose:     verbose,
	}, nil
}

func (r *Replayer) Close() error {
	if r.odb != nil {
		return r.odb.Close()
	}
	return nil
}

func (r *Replayer) referencesToRewrite() ([]*git.Reference, error) {
	refs, err := git.ParseReferences(r.ctx, r.repoPath, git.OrderNone)
	if err != nil {
		return nil, err
	}
	references := make([]*git.Reference, 0, len(refs))
	for _, ref := range refs {
		if strings.HasPrefix(ref.Name, "refs/remotes/") {
			continue
		}

		references = append(references, ref)
	}

	return references, nil
}

// Return all branch/tags commit reverse order
func (r *Replayer) commitsToRewrite() ([][]byte, error) {
	// --topo-order is required to ensure topological order.
	reader, err := git.NewReader(r.ctx, &command.RunOpts{RepoPath: r.repoPath}, "rev-list", "--reverse", "--topo-order", "--all")
	if err != nil {
		return nil, err
	}
	defer reader.Close()
	sr := bufio.NewScanner(reader)
	var commits [][]byte
	for sr.Scan() {
		oid, err := hex.DecodeString(strings.TrimSpace(sr.Text()))
		if err != nil {
			continue
		}
		commits = append(commits, oid)
	}
	return commits, nil
}
