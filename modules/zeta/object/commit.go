// Copyright 2018 Sourced Technologies, S.L.
// SPDX-License-Identifier: Apache-2.0

package object

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/antgroup/hugescm/modules/merkletrie/noder"
	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/streamio"
)

var (
	COMMIT_MAGIC = [4]byte{'Z', 'C', 0x00, 0x01}
)

// DateFormat is the format being used in the original git implementation
const DateFormat = "Mon Jan 02 15:04:05 2006 -0700"

const (
	BlobInlineMaxBytes = 4096
)

type Signature struct {
	Name  string    `json:"name"`
	Email string    `json:"email"`
	When  time.Time `json:"when"`
}

var timeZoneLength = 5

func (s *Signature) decodeTimeAndTimeZone(b []byte) {
	space := bytes.IndexByte(b, ' ')
	if space == -1 {
		space = len(b)
	}

	ts, err := strconv.ParseInt(string(b[:space]), 10, 64)
	if err != nil {
		return
	}

	s.When = time.Unix(ts, 0).In(time.UTC)
	var tzStart = space + 1
	if tzStart >= len(b) || tzStart+timeZoneLength > len(b) {
		return
	}

	timezone := string(b[tzStart : tzStart+timeZoneLength])
	tzhours, err1 := strconv.ParseInt(timezone[0:3], 10, 64)
	tzmins, err2 := strconv.ParseInt(timezone[3:], 10, 64)
	if err1 != nil || err2 != nil {
		return
	}
	if tzhours < 0 {
		tzmins *= -1
	}

	tz := time.FixedZone("", int(tzhours*60*60+tzmins*60))

	s.When = s.When.In(tz)
}

// Decode decodes a byte slice into a signature
func (s *Signature) Decode(b []byte) {
	open := bytes.LastIndexByte(b, '<')
	close := bytes.LastIndexByte(b, '>')
	if open == -1 || close == -1 {
		return
	}

	if close < open {
		return
	}

	s.Name = string(bytes.Trim(b[:open], " "))
	s.Email = string(b[open+1 : close])

	hasTime := close+2 < len(b)
	if hasTime {
		s.decodeTimeAndTimeZone(b[close+2:])
	}
}

const (
	formatTimeZoneOnly = "-0700"
)

// String implements the fmt.Stringer interface and formats a Signature as
// expected in the Git commit internal object format. For instance:
//
//	Taylor Blau <ttaylorr@github.com> 1494258422 -0600
func (s *Signature) String() string {
	at := s.When.Unix()
	zone := s.When.Format(formatTimeZoneOnly)

	return fmt.Sprintf("%s <%s> %d %s", s.Name, s.Email, at, zone)
}

// ExtraHeader encapsulates a key-value pairing of header key to header value.
// It is stored as a struct{string, string} in memory as opposed to a
// map[string]string to maintain ordering in a byte-for-byte encode/decode round
// trip.
type ExtraHeader struct {
	// K is the header key, or the first run of bytes up until a ' ' (\x20)
	// character.
	K string
	// V is the header value, or the remaining run of bytes in the line,
	// stripping off the above "K" field as a prefix.
	V string
}

type Commit struct {
	Hash plumbing.Hash `json:"hash"` // commit oid
	// Author is the Author this commit, or the original writer of the
	// contents.
	//
	// NOTE: this field is stored as a string to ensure any extra "cruft"
	// bytes are preserved through migration.
	Author Signature `json:"author"`
	// Committer is the individual or entity that added this commit to the
	// history.
	//
	// NOTE: this field is stored as a string to ensure any extra "cruft"
	// bytes are preserved through migration.
	Committer Signature `json:"committer"`
	// ParentIDs are the IDs of all parents for which this commit is a
	// linear child.
	Parents []plumbing.Hash `json:"parents"`
	// Tree is the root Tree associated with this commit.
	Tree plumbing.Hash `json:"tree"`
	// ExtraHeaders stores headers not listed above, for instance
	// "encoding", "gpgsig", or "mergetag" (among others).
	ExtraHeaders []*ExtraHeader `json:"-"`
	// Message is the commit message, including any signing information
	// associated with this commit.
	Message string `json:"message"`
	b       Backend
}

func (c *Commit) Encode(w io.Writer) error {
	_, err := w.Write(COMMIT_MAGIC[:])
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintf(w, "tree %s\n", c.Tree.String()); err != nil {
		return err
	}

	for _, parent := range c.Parents {
		if _, err = fmt.Fprintf(w, "parent %s\n", parent.String()); err != nil {
			return err
		}
	}

	if _, err = fmt.Fprintf(w, "author %s\ncommitter %s\n", c.Author.String(), c.Committer.String()); err != nil {
		return err
	}

	for _, hdr := range c.ExtraHeaders {
		if _, err = fmt.Fprintf(w, "%s %s\n", hdr.K, strings.ReplaceAll(hdr.V, "\n", "\n ")); err != nil {
			return err
		}

	}
	// c.Message is built from messageParts in the Decode() function.
	//
	// Since each entry in messageParts _does not_ contain its trailing LF,
	// append an empty string to capture the final newline.

	if _, err = fmt.Fprintf(w, "\n%s", c.Message); err != nil {
		return err
	}

	return nil
}

func (c *Commit) Decode(reader Reader) error {
	if reader.Type() != CommitObject {
		return ErrUnsupportedObject
	}
	c.Hash = reader.Hash()
	r := streamio.GetBufioReader(reader)
	defer streamio.PutBufioReader(r)

	var message strings.Builder
	var finishedHeaders bool
	for {
		line, readErr := r.ReadString('\n')
		if readErr != nil && readErr != io.EOF {
			return readErr
		}
		text := strings.TrimSuffix(line, "\n")
		if len(text) == 0 && !finishedHeaders {
			finishedHeaders = true
			continue
		}
		if fields := strings.Split(text, " "); !finishedHeaders {
			if len(fields) == 0 {
				// Executing in this block means that we got a
				// whitespace-only line, while parsing a header.
				//
				// Append it to the last-parsed header, and
				// continue.
				c.ExtraHeaders[len(c.ExtraHeaders)-1].V +=
					fmt.Sprintf("\n%s", text[1:])
				continue
			}
			if len(fields) < 2 {
				continue
			}
			switch fields[0] {
			case "tree":
				if len(fields) != 2 {
					return fmt.Errorf("error parsing tree: %s", text)
				}
				c.Tree = plumbing.NewHash(fields[1])
			case "parent":
				if len(fields) != 2 {
					return fmt.Errorf("error parsing parent: %s", text)
				}
				c.Parents = append(c.Parents, plumbing.NewHash(fields[1]))
			case "author":
				c.Author.Decode([]byte(text[7:]))
			case "committer":
				c.Committer.Decode([]byte(text[10:]))
			default:
				if strings.HasPrefix(text, " ") && len(c.ExtraHeaders) != 0 {
					idx := len(c.ExtraHeaders) - 1
					hdr := c.ExtraHeaders[idx]

					// Append the line of text (removing the
					// leading space) to the last header
					// that we parsed, adding a newline
					// between the two.
					hdr.V = strings.Join(append(
						[]string{hdr.V}, text[1:],
					), "\n")
				} else {
					c.ExtraHeaders = append(c.ExtraHeaders, &ExtraHeader{
						K: fields[0],
						V: strings.Join(fields[1:], " "),
					})
				}
			}
		} else {
			_, _ = message.WriteString(line)
		}
		if readErr == io.EOF {
			break
		}
	}
	c.Message = message.String()
	return nil
}

// Less defines a compare function to determine which commit is 'earlier' by:
// - First use Committer.When
// - If Committer.When are equal then use Author.When
// - If Author.When also equal then compare the string value of the hash
func (c *Commit) Less(rhs *Commit) bool {
	return c.Committer.When.Before(rhs.Committer.When) ||
		(c.Committer.When.Equal(rhs.Committer.When) &&
			(c.Author.When.Before(rhs.Author.When) ||
				(c.Author.When.Equal(rhs.Author.When) && bytes.Compare(c.Hash[:], rhs.Hash[:]) < 0)))
}

func indent(t string) string {
	var output []string
	for line := range strings.SplitSeq(t, "\n") {
		if len(line) != 0 {
			line = "    " + line
		}

		output = append(output, line)
	}

	return strings.Join(output, "\n")
}

func (c *Commit) String() string {
	return fmt.Sprintf(
		"%s %s\nAuthor: %s\nDate:   %s\n\n%s\n",
		CommitObject, c.Hash, c.Author.String(),
		c.Author.When.Format(DateFormat), indent(c.Message),
	)
}

func (c *Commit) Subject() string {
	if i := strings.IndexAny(c.Message, "\r\n"); i != -1 {
		return c.Message[0:i]
	}
	return c.Message
}

// Root returns the Tree from the commit.
func (c *Commit) Root(ctx context.Context) (*Tree, error) {
	return resolveTree(ctx, c.b, c.Tree)
}

// File returns the file with the specified "path" in the commit and a
// nil error if the file exists. If the file does not exist, it returns
// a nil file and the ErrFileNotFound error.
func (c *Commit) File(ctx context.Context, path string) (*File, error) {
	tree, err := c.Root(ctx)
	if err != nil {
		return nil, err
	}
	return tree.File(ctx, path)
}

// StatsContext returns the stats of a commit. Error will be return if context
// expires. Provided context must be non-nil.
func (c *Commit) StatsContext(ctx context.Context, m noder.Matcher, opts *PatchOptions) (FileStats, error) {
	from, err := c.Root(ctx)
	if err != nil {
		return nil, err
	}

	to := &Tree{}
	if len(c.Parents) != 0 {
		firstParent, err := c.b.Commit(ctx, c.Parents[0])
		if err != nil {
			return nil, err
		}

		to, err = firstParent.Root(ctx)
		if err != nil {
			return nil, err
		}
	}
	return to.StatsContext(ctx, from, m, opts)
}

// CommitIter is a generic closable interface for iterating over commits.
type CommitIter interface {
	Next(context.Context) (*Commit, error)
	ForEach(context.Context, func(*Commit) error) error
	Close()
}

// Parents return a CommitIter to the parent Commits.
func (c *Commit) MakeParents() CommitIter {
	return NewCommitIter(c.b, c.Parents)
}

// NumParents returns the number of parents in a commit.
func (c *Commit) NumParents() int {
	return len(c.Parents)
}

// GetCommit gets a commit from an object storer and decodes it.
func GetCommit(ctx context.Context, b Backend, oid plumbing.Hash) (*Commit, error) {
	return b.Commit(ctx, oid)
}
