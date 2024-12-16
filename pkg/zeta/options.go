// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package zeta

import (
	"errors"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/strengthen"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

var (
	ErrBranchHashExclusive  = errors.New("branch and Hash are mutually exclusive")
	ErrCreateRequiresBranch = errors.New("branch is mandatory when Create is used")
)

// CheckoutOptions describes how a checkout operation should be performed.
type CheckoutOptions struct {
	// Hash is the hash of the commit to be checked out. If used, HEAD will be
	// in detached mode. If Create is not used, Branch and Hash are mutually
	// exclusive.
	Hash plumbing.Hash
	// Branch to be checked out, if Branch and Hash are empty is set to `master`.
	Branch plumbing.ReferenceName
	// Create a new branch named Branch and start it at Hash.
	Create bool
	Force  bool
	Merge  bool
	First  bool
	One    bool
	Quiet  bool
}

// Validate validates the fields and sets the default values.
func (o *CheckoutOptions) Validate() error {
	if !o.Create && !o.Hash.IsZero() && o.Branch != "" {
		return ErrBranchHashExclusive
	}

	if o.Create && o.Branch == "" {
		return ErrCreateRequiresBranch
	}

	if o.Branch == "" {
		o.Branch = plumbing.Mainline
	}

	return nil
}

// ResetMode defines the mode of a reset operation.
type ResetMode int8

const (
	// MixedReset resets the index but not the working tree (i.e., the changed
	// files are preserved but not marked for commit) and reports what has not
	// been updated. This is the default action.
	MixedReset ResetMode = iota
	// HardReset resets the index and working tree. Any changes to tracked files
	// in the working tree are discarded.
	HardReset
	// MergeReset resets the index and updates the files in the working tree
	// that are different between Commit and HEAD, but keeps those which are
	// different between the index and working tree (i.e. which have changes
	// which have not been added).
	//
	// If a file that is different between Commit and the index has unstaged
	// changes, reset is aborted.
	MergeReset
	// SoftReset does not touch the index file or the working tree at all (but
	// resets the head to <commit>, just like all modes do). This leaves all
	// your changed files "Changes to be committed", as git status would put it.
	SoftReset
)

// working index HEAD target         working index HEAD
// ----------------------------------------------------
//  A       B     C    D     --soft   A       B     D
// 			  --mixed  A       D     D
// 			  --hard   D       D     D
// 			  --merge (disallowed)
// 			  --keep  (disallowed)
// working index HEAD target         working index HEAD
// ----------------------------------------------------
//  A       B     C    C     --soft   A       B     C
// 			  --mixed  A       C     C
// 			  --hard   C       C     C
// 			  --merge (disallowed)
// 			  --keep   A       C     C
// working index HEAD target         working index HEAD
// ----------------------------------------------------
//  B       B     C    D     --soft   B       B     D
// 			  --mixed  B       D     D
// 			  --hard   D       D     D
// 			  --merge  D       D     D
// 			  --keep  (disallowed)
// working index HEAD target         working index HEAD
// ----------------------------------------------------
//  B       B     C    C     --soft   B       B     C
// 			  --mixed  B       C     C
// 			  --hard   C       C     C
// 			  --merge  C       C     C
// 			  --keep   B       C     C
// working index HEAD target         working index HEAD
// ----------------------------------------------------
//  B       C     C    D     --soft   B       C     D
// 			  --mixed  B       D     D
// 			  --hard   D       D     D
// 			  --merge (disallowed)
// 			  --keep  (disallowed)
// working index HEAD target         working index HEAD
// ----------------------------------------------------
//  B       C     C    C     --soft   B       C     C
// 			  --mixed  B       C     C
// 			  --hard   C       C     C
// 			  --merge  B       C     C
// 			  --keep   B       C     C

// ResetOptions describes how a reset operation should be performed.
type ResetOptions struct {
	// Commit, if commit is present set the current branch head (HEAD) to it.
	Commit plumbing.Hash
	// Mode, form resets the current branch head to Commit and possibly updates
	// the index (resetting it to the tree of Commit) and the working tree
	// depending on Mode. If empty MixedReset is used.
	Mode ResetMode

	// Fetch missing objects
	Fetch bool
	// One by one checkout files
	One bool

	Quiet bool
}

// Validate validates the fields and sets the default values.
func (o *ResetOptions) Validate(r *Repository) error {
	if o.Commit == plumbing.ZeroHash {
		ref, err := r.Current()
		if err != nil {
			return err
		}

		o.Commit = ref.Hash()
	}

	return nil
}

// AddOptions describes how an `add` operation should be performed
type AddOptions struct {
	// All equivalent to `git add -A`, update the index not only where the
	// working tree has a file matching `Path` but also where the index already
	// has an entry. This adds, modifies, and removes index entries to match the
	// working tree.  If no `Path` nor `Glob` is given when `All` option is
	// used, all files in the entire working tree are updated.
	All bool
	// Path is the exact filepath to the file or directory to be added.
	Path string
	// Glob adds all paths, matching pattern, to the index. If pattern matches a
	// directory path, all directory contents are added to the index recursively.
	Glob string
	// SkipStatus adds the path with no status check. This option is relevant only
	// when the `Path` option is specified and does not apply when the `All` option is used.
	// Notice that when passing an ignored path it will be added anyway.
	// When true it can speed up adding files to the worktree in very large repositories.
	SkipStatus bool
	DryRun     bool
}

// Validate validates the fields and sets the default values.
func (o *AddOptions) Validate(r *Repository) error {
	if o.Path != "" && o.Glob != "" {
		return errors.New("fields Path and Glob are mutual exclusive")
	}

	return nil
}

// CommitOptions describes how a commit operation should be performed.
type CommitOptions struct {
	// All automatically stage files that have been modified and deleted, but
	// new files you have not told Git about are not affected.
	All bool
	// AllowEmptyCommits enable empty commits to be created. An empty commit
	// is when no changes to the tree were made, but a new commit message is
	// provided. The default behavior is false, which results in ErrEmptyCommit.
	AllowEmptyCommits bool
	// Author is the author's signature of the commit. If Author is empty the
	// Name and Email is read from the config, and time.Now it's used as When.
	Author object.Signature
	// Committer is the committer's signature of the commit. If Committer is
	// nil the Author signature is used.
	Committer object.Signature
	// Parents are the parents commits for the new commit, by default when
	// len(Parents) is zero, the hash of HEAD reference is used.
	Parents []plumbing.Hash
	// SignKey denotes a key to sign the commit with. A nil value here means the
	// commit will not be signed. The private key must be present and already
	// decrypted.
	SignKey *openpgp.Entity
	// Amend will create a new commit object and replace the commit that HEAD currently
	// points to. Cannot be used with All nor Parents.
	Amend             bool
	AllowEmptyMessage bool
	Message           []string
	File              string
}

func genMessage(message []string) string {
	if len(message) == 0 {
		return ""
	}
	lines := make([]string, 0, 10)
	lines = append(lines, strings.Split(message[0], "\n")...)
	if len(message) > 1 {
		lines = append(lines, message[1:]...)
	}
	var pos int
	for i, n := range lines {
		if len(n) != 0 {
			pos = i
			break
		}
	}
	lines = lines[pos:]
	if len(lines) == 0 {
		return ""
	}
	lines[0] = strings.TrimSpace(lines[0])
	if lines[len(lines)-1] != "" {
		lines = append(lines, "")
	}
	return strings.Join(lines, "\n")
}

var (
	ErrMissingAuthor = errors.New("author field is required")
)

type LogOrder int8

const (
	LogOrderDefault LogOrder = iota
	LogOrderDFS
	LogOrderDFSPost
	LogOrderBSF
	LogOrderCommitterTime
)

// LogOptions describes how a log action should be performed.
type LogOptions struct {
	// When the From option is set the log will only contain commits
	// reachable from it. If this option is not set, HEAD will be used as
	// the default From.
	From plumbing.Hash

	// The default traversal algorithm is Depth-first search
	// set Order=LogOrderCommitterTime for ordering by committer time (more compatible with `git log`)
	// set Order=LogOrderBSF for Breadth-first search
	Order LogOrder

	// Show only those commits in which the specified file was inserted/updated.
	// It is equivalent to running `zeta log -- <file-name>`.
	// this field is kept for compatibility, it can be replaced with PathFilter
	FileName *string

	// Filter commits based on the path of files that are updated
	// takes file path as argument and should return true if the file is desired
	// It can be used to implement `zeta log -- <path>`
	// either <path> is a file path, or directory path, or a regexp of file/directory path
	PathFilter func(string) bool

	// Pretend as if all the refs in refs/, along with HEAD, are listed on the command line as <commit>.
	// It is equivalent to running `zeta log --all`.
	// If set on true, the From option will be ignored.
	All bool

	// Show commits more recent than a specific date.
	// It is equivalent to running `zeta log --since <date>` or `zeta log --after <date>`.
	Since *time.Time

	// Show commits older than a specific date.
	// It is equivalent to running `zeta log --until <date>` or `zeta log --before <date>`.
	Until *time.Time
}

// Validate validates the fields and sets the default values.
func (o *CommitOptions) Validate(r *Repository) error {
	if o.All && o.Amend {
		return errors.New("all and amend cannot be used together")
	}

	if o.Amend && len(o.Parents) > 0 {
		return errors.New("parents cannot be used with amend")
	}
	if err := o.loadConfigAuthorAndCommitter(r); err != nil {
		return err
	}

	if len(o.Parents) == 0 {
		current, err := r.Current()
		if err != nil && err != plumbing.ErrReferenceNotFound {
			return err
		}

		if current != nil {
			o.Parents = []plumbing.Hash{current.Hash()}
		}
	}

	return nil
}

var (
	// dateFormats is a list of all the date formats that Git accepts,
	// except for the built-in one, which is handled below.
	dateFormats = []string{
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"2006-01-02T15:04:05-0700",
		"2006-01-02 15:04:05-0700",
		"2006.01.02T15:04:05-0700",
		"2006.01.02 15:04:05-0700",
		"01/02/2006T15:04:05-0700",
		"01/02/2006 15:04:05-0700",
		"02.01.2006T15:04:05-0700",
		"02.01.2006 15:04:05-0700",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05Z",
		"2006.01.02T15:04:05Z",
		"2006.01.02 15:04:05Z",
		"01/02/2006T15:04:05Z",
		"01/02/2006 15:04:05Z",
		"02.01.2006T15:04:05Z",
		"02.01.2006 15:04:05Z",
	}

	// defaultDatePattern is the regexp for Git's native date format.
	defaultDatePattern = regexp.MustCompile(`\A(\d+) ([+-])(\d{2})(\d{2})\z`)
)

func fallbackParseTime(date string) time.Time {
	if len(date) != 0 {
		// time.Parse doesn't parse seconds from the Epoch, like we use in the
		// Git native format, so we have to do it ourselves.
		strs := defaultDatePattern.FindStringSubmatch(date)
		if strs != nil {
			unixSecs, _ := strconv.ParseInt(strs[1], 10, 64)
			hours, _ := strconv.Atoi(strs[3])
			offset, _ := strconv.Atoi(strs[4])
			offset = (offset + hours*60) * 60
			if strs[2] == "-" {
				offset = -offset
			}

			return time.Unix(unixSecs, 0).In(time.FixedZone("", offset))
		}

		for _, format := range dateFormats {
			if t, err := time.Parse(format, date); err == nil {
				return t
			}
		}
	}
	// The user provided an invalid value, so default to the current time.
	return time.Now()
}

func (o *CommitOptions) loadConfigAuthorAndCommitter(r *Repository) error {
	o.Author.Name = r.authorName()
	o.Author.Email = r.authorEmail()
	o.Committer.Name = r.committerName()
	o.Committer.Email = r.committerEmail()
	if date, ok := os.LookupEnv(ENV_ZETA_AUTHOR_DATE); ok {
		o.Author.When = fallbackParseTime(date)
	}
	if date, ok := os.LookupEnv(ENV_ZETA_AUTHOR_DATE); ok {
		o.Author.When = fallbackParseTime(date)
	}
	if o.Author.When.IsZero() {
		o.Author.When = time.Now()
	}
	if o.Committer.When.IsZero() {
		o.Committer.When = o.Author.When
	}
	if len(o.Author.Name) == 0 || len(o.Author.Email) == 0 {
		return ErrMissingAuthor
	}
	return nil
}

// CleanOptions describes how a clean should be performed.
type CleanOptions struct {
	// dry run
	DryRun bool
	// Dir recurses into nested directories.
	Dir bool
	// All removes all changes, even those excluded by gitignore.
	All bool
}

// GrepOptions describes how a grep should be performed.
type GrepOptions struct {
	// Patterns are compiled Regexp objects to be matched.
	Patterns []*regexp.Regexp
	// InvertMatch selects non-matching lines.
	InvertMatch bool
	// CommitHash is the hash of the commit from which worktree should be derived.
	CommitHash plumbing.Hash
	// ReferenceName is the branch or tag name from which worktree should be derived.
	ReferenceName plumbing.ReferenceName
	// PathSpecs are compiled Regexp objects of pathspec to use in the matching.
	PathSpecs []*regexp.Regexp
	// Size Limit
	Limit int64
}

var (
	ErrHashOrReference = errors.New("ambiguous options, only one of CommitHash or ReferenceName can be passed")
)

// Validate validates the fields and sets the default values.
//
// TODO: deprecate in favor of Validate(r *Repository) in v6.
func (o *GrepOptions) Validate(w *Worktree) error {
	return o.validate(w.Repository)
}

func (o *GrepOptions) validate(r *Repository) error {
	if !o.CommitHash.IsZero() && o.ReferenceName != "" {
		return ErrHashOrReference
	}

	// If none of CommitHash and ReferenceName are provided, set commit hash of
	// the repository's head.
	if o.CommitHash.IsZero() && o.ReferenceName == "" {
		ref, err := r.Current()
		if err != nil {
			return err
		}
		o.CommitHash = ref.Hash()
	}
	if o.Limit == 0 {
		o.Limit = 128 * strengthen.MiByte // limit 128M
	}

	return nil
}
