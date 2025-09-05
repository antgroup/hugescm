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
	refPrefix       = "refs/"
	refHeadPrefix   = refPrefix + "heads/"
	refTagPrefix    = refPrefix + "tags/"
	refRemotePrefix = refPrefix + "remotes/"
	refNotePrefix   = refPrefix + "notes/"
)

const (
	RefRevParseRulesCount = 6
)

// RefRevParseRules are a set of rules to parse references into short names.
// These are the same rules as used by git in shorten_unambiguous_ref.
// See: https://github.com/git/git/blob/9857273be005833c71e2d16ba48e193113e12276/refs.c#L610
var RefRevParseRules = []string{
	"%s",
	"refs/%s",
	"refs/tags/%s",
	"refs/heads/%s",
	"refs/remotes/%s",
	"refs/remotes/%s/HEAD",
}

// ReferenceType reference type's
type ReferenceType int8

const (
	InvalidReference  ReferenceType = 0
	HashReference     ReferenceType = 1
	SymbolicReference ReferenceType = 2
)

func (r ReferenceType) String() string {
	switch r {
	case InvalidReference:
		return "invalid-reference"
	case HashReference:
		return "hash-reference"
	case SymbolicReference:
		return "symbolic-reference"
	}

	return ""
}

// ReferenceName reference name's
type ReferenceName string

// NewBranchReferenceName returns a reference name describing a branch based on
// his short name.
func NewBranchReferenceName(name string) ReferenceName {
	return ReferenceName(refHeadPrefix + name)
}

// NewNoteReferenceName returns a reference name describing a note based on his
// short name.
func NewNoteReferenceName(name string) ReferenceName {
	return ReferenceName(refNotePrefix + name)
}

// NewRemoteReferenceName returns a reference name describing a remote branch
// based on his short name and the remote name.
func NewRemoteReferenceName(remote, name string) ReferenceName {
	return ReferenceName(refRemotePrefix + fmt.Sprintf("%s/%s", remote, name))
}

// NewRemoteHEADReferenceName returns a reference name describing a the HEAD
// branch of a remote.
func NewRemoteHEADReferenceName(remote string) ReferenceName {
	return ReferenceName(refRemotePrefix + fmt.Sprintf("%s/%s", remote, HEAD))
}

// NewTagReferenceName returns a reference name describing a tag based on short
// his name.
func NewTagReferenceName(name string) ReferenceName {
	return ReferenceName(refTagPrefix + name)
}

// IsBranch check if a reference is a branch
func (r ReferenceName) IsBranch() bool {
	return strings.HasPrefix(string(r), refHeadPrefix)
}

func (r ReferenceName) BranchName() string {
	return strings.TrimPrefix(string(r), refHeadPrefix)
}

// IsNote check if a reference is a note
func (r ReferenceName) IsNote() bool {
	return strings.HasPrefix(string(r), refNotePrefix)
}

// IsRemote check if a reference is a remote
func (r ReferenceName) IsRemote() bool {
	return strings.HasPrefix(string(r), refRemotePrefix)
}

// IsTag check if a reference is a tag
func (r ReferenceName) IsTag() bool {
	return strings.HasPrefix(string(r), refTagPrefix)
}

func (r ReferenceName) TagName() string {
	return strings.TrimPrefix(string(r), refTagPrefix)
}

func (r ReferenceName) String() string {
	return string(r)
}

// Short returns the short name of a ReferenceName
//
//	un strict, does not check whether the name is ambiguous
func (r ReferenceName) Short() string {
	s := string(r)
	res := s
	// skip first
	for _, format := range RefRevParseRules[1:] {
		_, err := fmt.Sscanf(s, format, &res)
		if err == nil {
			continue
		}
	}

	return res
}

const (
	HEAD   ReferenceName = "HEAD"
	Master ReferenceName = "refs/heads/master"
)

// Branch returns `true` and the branch name if the reference is a branch. E.g.
// if ReferenceName is "refs/heads/master", it will return "master". If it is
// not a branch, `false` is returned.
func (r ReferenceName) Branch() (string, bool) {
	if branch, ok := strings.CutPrefix(r.String(), refHeadPrefix); ok && len(branch) != 0 {
		return branch, true
	}
	return "", false
}

// Reference represents a Git reference.
type Reference struct {
	// Name is the name of the reference
	Name ReferenceName
	// Target is the target of the reference. For direct references it
	// contains the object ID, for symbolic references it contains the
	// target branch name.
	Target string
	// ObjectType is the type of the object referenced.
	ObjectType ObjectType
	// ShortName: ONLY git parsed (else maybe empty)
	ShortName string
	// IsSymbolic tells whether the reference is direct or symbolic
	IsSymbolic bool
}

// NewReference creates a direct reference to an object.
func NewReference(name ReferenceName, target string) Reference {
	return Reference{
		Name:       name,
		Target:     target,
		IsSymbolic: false,
	}
}

// NewSymbolicReference creates a symbolic reference to another reference.
func NewSymbolicReference(name ReferenceName, target ReferenceName) Reference {
	return Reference{
		Name:       name,
		Target:     string(target),
		IsSymbolic: true,
	}
}

type ErrAlreadyLocked struct {
	refname string
	message string
}

func (e *ErrAlreadyLocked) Error() string {
	if len(e.message) != 0 {
		return e.message
	}
	return fmt.Sprintf("reference is already locked: %q", e.refname)
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

// fatal: update_ref failed for ref 'refs/heads/release/1.0.0_20250728': 'refs/heads/release' exists; cannot create 'refs/heads/release/1.0.0_20250728
func UpdateRef(ctx context.Context, repoPath string, reference string, oldRev, newRev string, forceUpdate bool) error {
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
			return &ErrAlreadyLocked{refname: reference}
		}
		if strings.Contains(message, " exists; cannot create ") {
			return &ErrAlreadyLocked{message: message}
		}
		if strings.Contains(message, "Another git process seems to be running in this repository") {
			return &ErrAlreadyLocked{refname: reference, message: message}
		}
		return fmt.Errorf("update-ref %s error: %w stderr: %v", reference, err, message)
	}
	return nil
}

type ErrReferenceBadName struct {
	Name string
}

func (err ErrReferenceBadName) Error() string {
	return fmt.Sprintf("bad revision name: '%s'", err.Name)
}

func IsErrReferenceBadName(err error) bool {
	_, ok := err.(*ErrReferenceBadName)
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

func ParseOneReference(referenceLine string) (*Reference, error) {
	fields := strings.SplitN(referenceLine, "\x00", 4)
	if len(fields) != 4 {
		return nil, fmt.Errorf("invalid output from git for-each-ref command: %v", referenceLine)
	}
	typ, err := ParseObjectType(fields[3])
	if err != nil {
		return nil, err
	}
	return &Reference{Name: ReferenceName(fields[0]), ShortName: fields[1], Target: fields[2], ObjectType: typ}, nil
}

type ReferenceEx struct {
	Name       ReferenceName // name
	ShortName  string        // short name
	Target     string        // target commit,tag or symbolic
	IsSymbolic bool          // is symbolic
	Commit     *Commit       // commit
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
	defer reader.Close()
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		b, err := ParseOneReference(scanner.Text())
		if err != nil {
			break
		}
		if i, ok := matches[b.Name.String()]; ok {
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
	cc, err := ParseRev(ctx, repoPath, br.Target)
	if IsErrNotExist(err) {
		return nil, NewBranchNotFound(refname)
	}
	if err != nil {
		return nil, err
	}
	return &ReferenceEx{Name: br.Name, ShortName: br.ShortName, Target: br.Target, IsSymbolic: br.IsSymbolic, Commit: cc}, nil
}

func HasSpecificReference(ctx context.Context, repoPath string, referencePrefix string) (bool, error) {
	showRefArgs := []string{"for-each-ref"}
	if len(referencePrefix) != 0 {
		showRefArgs = append(showRefArgs, referencePrefix)
	}
	showRefArgs = append(showRefArgs, "--format=%(refname)", "--count=1")
	cmd := command.New(ctx, repoPath, "git", showRefArgs...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	defer stdout.Close()
	scanner := bufio.NewScanner(stdout)
	if err := cmd.Start(); err != nil {
		return false, err
	}
	defer cmd.Exit() // nolint
	var result bool
	for scanner.Scan() {
		result = true
	}
	return result, nil
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
		r, err := ParseOneReference(scanner.Text())
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
