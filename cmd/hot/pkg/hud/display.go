package hud

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/term"
)

func typePadding(e *git.TreeEntry, padding int) string {
	t := e.Type()
	if padding > len(t) {
		return t + strings.Repeat(" ", padding-len(t))
	}
	return t
}

func encodeEntry(w io.Writer, e *git.TreeEntry, t string, v term.Level) error {
	switch e.Filemode {
	case git.Symlink:
		if _, err := fmt.Fprintf(w, "%s %s %s %s\n", e.Filemode, v.Purple(t), e.Hash, v.Purple(e.Name)); err != nil {
			return err
		}
	case git.Executable:
		if _, err := fmt.Fprintf(w, "%s %s %s %s\n", e.Filemode, v.Red(t), e.Hash, v.Red(e.Name)); err != nil {
			return err
		}
	case git.Regular:
		if _, err := fmt.Fprintf(w, "%s %s %s %s\n", e.Filemode, t, e.Hash, e.Name); err != nil {
			return err
		}
	case git.Dir:
		if _, err := fmt.Fprintf(w, "%s %s %s %s\n", e.Filemode, v.Blue(t), e.Hash, v.Blue(e.Name)); err != nil {
			return err
		}
	case git.Submodule:
		if _, err := fmt.Fprintf(w, "%s %s %s %s\n", e.Filemode, v.Yellow(t), e.Hash, v.Yellow(e.Name)); err != nil {
			return err
		}
	default:
		if _, err := fmt.Fprintf(w, "%s %s %s %s\n", e.Filemode, t, e.Hash, e.Name); err != nil {
			return err
		}
	}
	return nil
}

const (
	commitTypeName = "commit"
)

func encodeTree(w io.Writer, t *git.Tree, v term.Level) error {
	p := 0
	if v != term.LevelNone && slices.IndexFunc(t.Entries, func(e *git.TreeEntry) bool { return e.Filemode == git.Submodule }) != -1 {
		p = len(commitTypeName) // commit
	}
	for _, e := range t.Entries {
		if err := encodeEntry(w, e, typePadding(e, p), v); err != nil {
			return err
		}
	}
	return nil
}

func encodeTag(w io.Writer, t *git.Tag, v term.Level) error {
	headers := []string{
		fmt.Sprintf("%s %s", v.Blue("object"), v.Green(t.Object)),
		fmt.Sprintf("%s %s", v.Blue("type"), v.Green(t.Type)),
		fmt.Sprintf("%s %s", v.Blue("tag"), v.Green(t.Name)),
		fmt.Sprintf("%s %s", v.Blue("tagger"), v.Green(t.Tagger.String())),
	}
	_, err := fmt.Fprintf(w, "%s\n\n%s", strings.Join(headers, "\n"), t.Content)
	return err
}

func encodeCommit(w io.Writer, c *git.Commit, v term.Level) (err error) {
	if _, err = fmt.Fprintf(w, "%s %s\n", v.Blue("tree"), v.Green(c.Tree)); err != nil {
		return err
	}

	for _, parent := range c.Parents {
		if _, err = fmt.Fprintf(w, "%s %s\n", v.Blue("parent"), v.Green(parent)); err != nil {
			return err
		}
	}

	if _, err = fmt.Fprintf(w, "%s %s\n%s %s\n", v.Blue("author"), v.Green(c.Author.String()), v.Blue("committer"), v.Green(c.Committer.String())); err != nil {
		return err
	}

	for _, hdr := range c.ExtraHeaders {
		if _, err = fmt.Fprintf(w, "%s %s\n", v.Blue(hdr.K), strings.ReplaceAll(hdr.V, "\n", "\n ")); err != nil {
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

func Display(w io.Writer, a any, v term.Level) error {
	switch o := a.(type) {
	case *git.Commit:
		return encodeCommit(w, o, v)
	case *git.Tag:
		return encodeTag(w, o, v)
	case *git.Tree:
		return encodeTree(w, o, v)
	}
	_, err := fmt.Fprintln(w, a)
	return err
}
