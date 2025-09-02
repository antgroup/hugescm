package git

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/antgroup/hugescm/modules/command"
)

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
	// Hash of the commit object.
	Hash string
	// Tree is the hash of the root tree of the commit.
	Tree string
	// Parents are the hashes of the parent commits of the commit.
	Parents []string
	// Author is the original author of the commit.
	Author Signature
	// Committer is the one performing the commit, might be different from
	// Author.
	Committer Signature
	// ExtraHeaders stores headers not listed above, for instance
	// "encoding", "gpgsig", or "mergetag" (among others).
	ExtraHeaders []*ExtraHeader
	// Message is the commit message, contains arbitrary text.
	Message string
}

func (c *Commit) Signature() string {
	for _, e := range c.ExtraHeaders {
		if e.K == "gpgsig" {
			return e.V
		}
	}
	return ""
}

// CommitGPGSignature represents a git commit signature part.
type CommitGPGSignature struct {
	Signature string
	Payload   string // TODO check if can be reconstruct from the rest of commit information to not have duplicate data
}

func (c *Commit) ExtractCommitGPGSignature() *CommitGPGSignature {
	var signature string
	for _, e := range c.ExtraHeaders {
		if e.K == "gpgsig" {
			signature = e.V
		}
	}
	if len(signature) == 0 {
		return nil
	}

	var w strings.Builder
	var err error

	if _, err = fmt.Fprintf(&w, "tree %s\n", c.Tree); err != nil {
		return nil
	}

	for _, parent := range c.Parents {
		if _, err = fmt.Fprintf(&w, "parent %s\n", parent); err != nil {
			return nil
		}
	}

	if _, err = fmt.Fprint(&w, "author "); err != nil {
		return nil
	}

	if err = c.Author.Encode(&w); err != nil {
		return nil
	}

	if _, err = fmt.Fprint(&w, "\ncommitter "); err != nil {
		return nil
	}

	if err = c.Committer.Encode(&w); err != nil {
		return nil
	}

	if _, err = fmt.Fprintf(&w, "\n\n%s", c.Message); err != nil {
		return nil
	}

	return &CommitGPGSignature{
		Signature: signature,
		Payload:   w.String()}
}

func (c *Commit) Decode(hash string, reader io.Reader) error {
	c.Hash = hash
	r, ok := reader.(*bufio.Reader)
	if !ok {
		r = bufio.NewReader(reader)
	}
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
				c.Tree = fields[1]
			case "parent":
				if len(fields) != 2 {
					return fmt.Errorf("error parsing parent: %s", text)
				}
				c.Parents = append(c.Parents, fields[1])
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

func (c *Commit) Subject() string {
	if i := strings.IndexAny(c.Message, "\r\n"); i != -1 {
		return c.Message[0:i]
	}
	return c.Message
}

func RevUniqueList(ctx context.Context, repoPath string, ours, theirs string) ([]string, error) {
	stderr := command.NewStderr()
	cmd := command.NewFromOptions(ctx, &command.RunOpts{
		RepoPath: repoPath,
		Stderr:   stderr,
	}, "git",
		"rev-list",
		"--cherry-pick",
		"--right-only",
		"--no-merges",
		"--topo-order",
		"--reverse",
		fmt.Sprintf("%s...%s", ours, theirs),
	)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	defer stdout.Close()
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	var todoList []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		todoList = append(todoList, strings.TrimSpace(scanner.Text()))
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("rev-list error: %w stderr: %v", err, stderr.String())
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scanning rev-list output: %w", err)
	}
	return todoList, nil
}
