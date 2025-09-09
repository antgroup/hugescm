package refs

import (
	"context"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/antgroup/hugescm/cmd/hot/pkg/hud"
	"github.com/antgroup/hugescm/cmd/hot/pkg/tr"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/git/gitobj"
)

// CommitGPGSignature represents a git commit signature part.
type CommitGPGSignature struct {
	Signature string
	Payload   string // TODO check if can be reconstruct from the rest of commit information to not have duplicate data
}

type Reference struct {
	Name      string
	ShortName string
	Hash      string
	Peeling   string
	Tree      string
	Parents   []string
	Author    *git.Signature
	Committer *git.Signature
	Message   string
	Leading   int // leading > mainline
	Lagging   int // lagging < mainline
	Broken    bool
}

func (r *Reference) Merged() bool {
	return r.IsBranch() && r.Leading == 0
}

func (r *Reference) IsBranch() bool {
	return strings.HasPrefix(r.Name, "refs/heads/")
}

func (r *Reference) IsTag() bool {
	return strings.HasPrefix(r.Name, "refs/tags/")
}

type Matcher interface {
	Match(string) bool
}

type References struct {
	BasePoint string
	Current   string
	Items     []*Reference
}

func (r *References) resolveRefCommit(odb *git.ODB, ref *git.Reference) ([]byte, *gitobj.Commit, error) {
	sha, err := hex.DecodeString(ref.Target)
	if err != nil {
		return nil, nil, fmt.Errorf("could not decode: %q", ref.Target)
	}
	for range 20 {
		obj, err := odb.Object(sha)
		if err != nil {
			return nil, nil, fmt.Errorf("open git object error: %w", err)
		}
		if obj.Type() == gitobj.CommitObjectType {
			return sha, obj.(*gitobj.Commit), nil
		}
		if obj.Type() != gitobj.TagObjectType {
			return nil, nil, fmt.Errorf("oid: %s unsupport object type: %s", hex.EncodeToString(sha), obj.Type())
		}
		tag := obj.(*gitobj.Tag)
		sha = tag.Object
	}
	return nil, nil, fmt.Errorf("ref '%s' recursion depth is not supported", ref.Name)
}

func (r *References) resolve(ctx context.Context, repoPath string, odb *git.ODB, ref *git.Reference) error {
	sha, cc, err := r.resolveRefCommit(odb, ref)
	if err != nil {
		r.Items = append(r.Items, &Reference{Name: ref.Name.String(), Hash: ref.Target, Broken: true})
		return err
	}
	reference := &Reference{
		Name:      ref.Name.String(),
		ShortName: ref.ShortName,
		Hash:      ref.Target,
		Tree:      hex.EncodeToString(cc.TreeID),
		Message:   cc.Message,
		Author:    git.SignatureFromLine(cc.Author),
		Committer: git.SignatureFromLine(cc.Committer),
	}
	for _, p := range cc.ParentIDs {
		reference.Parents = append(reference.Parents, hex.EncodeToString(p))
	}
	if peeling := hex.EncodeToString(sha); peeling != ref.Target {
		reference.Peeling = peeling
	}
	if reference.Hash != r.BasePoint && ref.Name.IsBranch() {
		reference.Leading, reference.Lagging, _ = git.RevDivergingCount(ctx, repoPath, reference.Hash, r.BasePoint)
	}
	r.Items = append(r.Items, reference)
	return nil
}

func ScanReferences(ctx context.Context, repoPath string, m Matcher, order git.Order) (*References, error) {
	odb, err := git.NewODB(repoPath, git.HashFormatOK(repoPath))
	if err != nil {
		return nil, err
	}
	defer odb.Close() // nolint
	refs, err := git.ParseReferences(ctx, repoPath, order)
	if err != nil {
		return nil, err
	}
	b := hud.NewBar(tr.W("scan references"), len(refs), 1, 1, false)
	hash, refname, _ := git.ParseReference(ctx, repoPath, "HEAD")
	r := &References{
		BasePoint: hash,
		Current:   refname,
		Items:     make([]*Reference, 0, 200),
	}
	for _, ref := range refs {
		b.Add(1)
		if !m.Match(ref.Name.String()) {
			continue
		}
		if err := r.resolve(ctx, repoPath, odb, ref); err != nil {
			fmt.Fprintf(os.Stderr, "Parse ref: %s error: %v\n", ref.Name, err)
		}
	}
	b.Done()
	return r, nil
}

func RemoveBrokenRef(repoPath string, refName string) error {
	refPath := filepath.Join(repoPath, refName)
	return os.Remove(refPath)
}
