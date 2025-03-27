package git

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
)

const (
	RefsPrefix   = "refs/"
	BranchPrefix = "refs/heads/"
	TagPrefix    = "refs/tags/"
)

func ReferenceBranchName(b string) string {
	if strings.HasPrefix(b, BranchPrefix) {
		return b
	}
	return BranchPrefix + b
}

func BranchRev(r string) string {
	if ValidateHexLax(r) {
		return r
	}
	if strings.HasPrefix(r, RefsPrefix) {
		return r
	}
	return BranchPrefix + r
}

func ReferenceTagName(tag string) string {
	if strings.HasPrefix(tag, TagPrefix) {
		return tag
	}
	return TagPrefix + tag
}

type ErrAlreadyLocked struct {
	Ref string
}

func (e *ErrAlreadyLocked) Error() string {
	return fmt.Sprintf("reference is already locked: %q", e.Ref)
}

var (
	refLockedRegex       = regexp.MustCompile("cannot lock ref '(.+?)'")
	ErrReferenceNotFound = errors.New("reference not found")
)

func IsErrAlreadyLocked(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(*ErrAlreadyLocked)
	return ok
}

func ReferenceTarget(ctx context.Context, repoPath, reference string) (string, error) {
	// fatal: ambiguous argument 'refs/heads/dev': unknown revision or path not in the working tree
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{RepoPath: repoPath, Stderr: stderr},
		"git", "rev-parse", reference)
	oid, err := cmd.OneLine()
	if err != nil {
		if strings.Contains(stderr.String(), "fatal:") {
			return "", ErrReferenceNotFound
		}
		return "", err
	}
	return oid, nil
}

func ReferenceUpdate(ctx context.Context, repoPath string, reference string, oldRev, newRev string, forceUpdate bool) error {
	updateRefArgs := []string{"update-ref", "--", reference, newRev}
	if !forceUpdate {
		// git update-ref refs/heads/master <newvalue> <oldvalue> check oldRev matched
		updateRefArgs = append(updateRefArgs, oldRev)
	}
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx,
		&command.RunOpts{
			RepoPath: repoPath,
			Stderr:   stderr,
		}, "git", updateRefArgs...)
	if err := cmd.Run(); err != nil {
		message := stderr.String()
		if refLockedRegex.MatchString(message) {
			return &ErrAlreadyLocked{Ref: reference}
		}
		return fmt.Errorf("update-ref %s error: %w stderr: %v", reference, err, message)
	}
	return nil
}

type ErrBadReferenceName struct {
	Name string
}

func (err ErrBadReferenceName) Error() string {
	return fmt.Sprintf("bad revision name: '%s'", err.Name)
}

func IsErrBadReferenceName(err error) bool {
	_, ok := err.(*ErrBadReferenceName)
	return ok
}

// https://github.com/git/git/blob/ae73b2c8f1da39c39335ee76a0f95857712c22a7/refs.c#L41-L290

var (
	// refnameDisposition table
	//
	// Here golang's logic is different from C's, golang's strings are not NULL-terminated, so byte(0) is a forbidden character.
	refnameDisposition = [256]byte{
		4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
		4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4, 4,
		4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 5, 0, 0, 0, 2, 1,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 0, 0, 0, 0, 4,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 4, 0, 4, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 3, 0, 0, 4, 4,
	}
)

/*
 * How to handle various characters in refnames:
 * 0: An acceptable character for refs
 * 1: End-of-component
 * 2: ., look for a preceding . to reject .. in refs
 * 3: {, look for a preceding @ to reject @{ in refs
 * 4: A bad character: ASCII control characters, and
 *    ":", "?", "[", "\", "^", "~", SP, or TAB
 * 5: *, reject unless REFNAME_REFSPEC_PATTERN is set
 */
func checkReferenceNameComponent(refname []byte) int {
	last := byte(0)
	var i int
	for ; i < len(refname); i++ {
		ch := refname[i] & 255
		disp := refnameDisposition[ch]
		switch disp {
		case 1:
			goto OUT // Do not use range, which causes extra processing for goto statements.
		case 2:
			if last == '.' {
				return -1
			}
		case 3:
			if last == '@' {
				return -1
			}
		case 4:
			return -1
		case 5:
			// we not use pattern mode
			return -1
		}
		last = ch
	}
OUT:
	if i == 0 {
		return 0
	}
	if refname[0] == '.' {
		return -1
	}
	if bytes.HasSuffix(refname, []byte(".lock")) {
		return -1
	}
	return i
}

/*
 * Try to read one refname component from the front of refname.
 * Return the length of the component found, or -1 if the component is
 * not legal.  It is legal if it is something reasonable to have under
 * ".git/refs/"; We do not like it if:
 *
 * - it begins with ".", or
 * - it has double dots "..", or
 * - it has ASCII control characters, or
 * - it has ":", "?", "[", "\", "^", "~", SP, or TAB anywhere, or
 * - it has "*" anywhere unless REFNAME_REFSPEC_PATTERN is set, or
 * - it ends with a "/", or
 * - it ends with ".lock", or
 * - it contains a "@{" portion
 *
 * When sanitized is not NULL, instead of rejecting the input refname
 * as an error, try to come up with a usable replacement for the input
 * refname in it.
 */
func ValidateReferenceName(refname []byte) bool {
	if bytes.Equal(refname, []byte("@")) {
		return false
	}
	var componentLen int
	for {
		/* We are at the start of a path component. */
		if componentLen = checkReferenceNameComponent(refname); componentLen <= 0 {
			return false
		}
		if len(refname) == componentLen {
			break
		}
		refname = refname[componentLen+1:]
	}
	return refname[componentLen-1] != '.'
}

// ValidateBranchName: creating branches starting with - is not supported
func ValidateBranchName(branch []byte) bool {
	if len(branch) == 0 || branch[0] == '-' {
		return false
	}
	return ValidateReferenceName(branch)
}

// ValidateTagName: creating tags starting with - is not supported
func ValidateTagName(tag []byte) bool {
	if len(tag) == 0 || tag[0] == '-' {
		return false
	}
	return ValidateReferenceName(tag)
}

const (
	ReferenceLineFormat = "%(refname)%00%(refname:short)%00%(objectname)%00%(objecttype)"
)

type Reference struct {
	// Name is the full reference name of the reference.
	Name string
	// Hash is the Hash of the referred-to object.
	Hash string
	// ObjectType is the type of the object referenced.
	ObjectType ObjectType
	// ShortName is the short reference name of the reference
	ShortName string
}

func ParseReferenceLine(referenceLine string) (*Reference, error) {
	fields := strings.SplitN(referenceLine, "\x00", 4)
	if len(fields) != 4 {
		return nil, fmt.Errorf("invalid output from git for-each-ref command: %v", referenceLine)
	}
	typ, err := ParseObjectType(fields[3])
	if err != nil {
		return nil, err
	}
	return &Reference{Name: fields[0], ShortName: fields[1], Hash: fields[2], ObjectType: typ}, nil
}

type ReferenceEx struct {
	// Name is the full reference name of the reference.
	Name string
	// Hash is the Hash of the referred-to object.
	Hash string
	// ObjectType is the type of the object referenced.
	ObjectType ObjectType
	// ShortName is the short reference name of the reference
	ShortName string
	// ShortName is the short reference name of the reference
	Commit *Commit
}

// ReferencePrefixMatch: follow git's priority for finding refs
//
// https://git-scm.com/docs/git-rev-parse#Documentation/git-rev-parse.txt-emltrefnamegtemegemmasterememheadsmasterememrefsheadsmasterem
//
// https://github.com/git/git/blob/master/Documentation/revisions.txt
func ReferencePrefixMatch(ctx context.Context, repoPath string, refname string) (*ReferenceEx, error) {
	refs := make([]*Reference, 6)
	matches := map[string]int{
		refname:                             0, //1
		"refs/" + refname:                   1, //2
		"refs/tags/" + refname:              2, //3
		"refs/heads/" + refname:             3, //4
		"refs/remotes/" + refname:           4, //5
		"refs/remotes/" + refname + "/HEAD": 5, //6
	}
	stderr := command.NewStderr()
	psArgs := []string{"for-each-ref", "--format", ReferenceLineFormat}
	if !strings.HasPrefix(refname, "-") {
		psArgs = append(psArgs, refname) //1
	}
	psArgs = append(psArgs,
		"refs/"+refname,                 //2
		"refs/tags/"+refname,            //3
		"refs/heads/"+refname,           //4
		"refs/remotes/"+refname,         //5
		"refs/remotes/"+refname+"/HEAD", //6
	)
	reader, err := NewReader(ctx, &command.RunOpts{RepoPath: repoPath, Stderr: stderr}, psArgs...)
	if err != nil {
		return nil, err
	}
	defer reader.Close() // nolint
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		b, err := ParseReferenceLine(scanner.Text())
		if err != nil {
			break
		}
		if i, ok := matches[b.Name]; ok {
			refs[i] = b
		}
	}
	br := func() *Reference {
		for _, b := range refs {
			if b != nil {
				return b
			}
		}
		return nil
	}()
	if br == nil {
		return nil, NewBranchNotFound(refname)
	}

	d, err := NewDecoder(ctx, repoPath)
	if err != nil {
		return nil, err
	}
	defer d.Close() // nolint

	cc, err := d.ResolveCommit(br.Hash)
	if IsErrNotExist(err) {
		return nil, NewBranchNotFound(refname)
	}
	if err != nil {
		return nil, err
	}
	return &ReferenceEx{Name: br.Name, ShortName: br.ShortName, Hash: br.Hash, Commit: cc}, nil
}

type Order int

const (
	OrderNone Order = iota
	OrderNewest
	OrderOldest
)

func ParseReferences(ctx context.Context, repoPath string, order Order) ([]*Reference, error) {
	cmdArgs := []string{"for-each-ref"}
	switch order {
	case OrderNewest:
		cmdArgs = append(cmdArgs, "--sort=-committerdate")
	case OrderOldest:
		cmdArgs = append(cmdArgs, "--sort=committerdate")
	}
	cmdArgs = append(cmdArgs, "--format", ReferenceLineFormat)
	reader, err := NewReader(ctx, &command.RunOpts{RepoPath: repoPath}, cmdArgs...)
	if err != nil {
		return nil, err
	}
	defer reader.Close() // nolint
	refs := make([]*Reference, 0, 100)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		r, err := ParseReferenceLine(scanner.Text())
		if err != nil {
			break
		}
		refs = append(refs, r)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return refs, nil
}

// RevParseCurrent: resolve the reference pointed to by HEAD
//
// not git repo:
//
// fatal: not a git repository (or any of the parent directories): .git
//
// empty repo:
//
// fatal: ambiguous argument 'HEAD': unknown revision or path not in the working tree.
// Use '--' to separate paths from revisions, like this:
// 'git <command> [<revision>...] -- [<file>...]'
//
// ref not exists: HEAD
//
// refs/heads/master
func RevParseCurrent(ctx context.Context, environ []string, repoPath string) (string, error) {
	//  git rev-parse --symbolic-full-name HEAD
	cmd := command.NewFromOptions(ctx, &command.RunOpts{RepoPath: repoPath, Environ: environ},
		"git", "rev-parse", "--symbolic-full-name", "HEAD")
	line, err := cmd.OneLine()
	if err != nil {
		return ReferenceNameDefault, err
	}
	return line, nil
}

// RevParseCurrentEx parse HEAD return hash and refname
//
//	git rev-parse HEAD --symbolic-full-name HEAD
//
// result:
//
//	85e15f6f6272033eb83e5a56f650a7a5f9c84cf6
//	refs/heads/master
func RevParseCurrentEx(ctx context.Context, environ []string, repoPath string) (string, string, error) {
	cmd := command.NewFromOptions(ctx, &command.RunOpts{RepoPath: repoPath, Environ: environ},
		"git", "rev-parse", "HEAD", "--symbolic-full-name", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return "", ReferenceNameDefault, err
	}
	hash, refname, _ := strings.Cut(string(output), "\n")
	return hash, strings.TrimSpace(refname), nil
}
