package zeta

import (
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/plumbing/filemode"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/zeta/object"
)

func typePadding(e *object.TreeEntry, padding int) string {
	t := e.Type().String()
	if padding > len(t) {
		return t + strings.Repeat(" ", padding-len(t))
	}
	return t
}

func encodeEntry(w io.Writer, e *object.TreeEntry, t, sz string, v term.Level) error {
	if e.IsFragments() {
		if _, err := fmt.Fprintf(w, "%s %s %s %s %s\n", e.Mode.Origin(), v.Yellow(t), e.Hash, sz, v.Yellow(e.Name)); err != nil {
			return err
		}
		return nil
	}
	switch e.Mode {
	case filemode.Symlink:
		if _, err := fmt.Fprintf(w, "%s %s %s %s %s\n", e.Mode, v.Purple(t), e.Hash, sz, v.Purple(e.Name)); err != nil {
			return err
		}
	case filemode.Executable:
		if _, err := fmt.Fprintf(w, "%s %s %s %s %s\n", e.Mode, v.Red(t), e.Hash, sz, v.Red(e.Name)); err != nil {
			return err
		}
	case filemode.Dir:
		if _, err := fmt.Fprintf(w, "%s %s %s %s %s\n", e.Mode, v.Blue(t), e.Hash, sz, v.Blue(e.Name)); err != nil {
			return err
		}
	default:
		if _, err := fmt.Fprintf(w, "%s %s %s %s %s\n", e.Mode, t, e.Hash, sz, e.Name); err != nil {
			return err
		}
	}
	return nil
}

func indexPadding(f *object.Fragments) int {
	var v uint32
	for _, e := range f.Entries {
		v = max(v, e.Index)
	}
	indexMax := len(strconv.Itoa(int(v)))
	return indexMax
}

func fragmentIndexPadding(e *object.Fragment, padding int) string {
	ss := strconv.Itoa(int(e.Index))
	if len(ss) >= padding {
		return ss
	}
	return strings.Repeat(" ", padding-len(ss)) + ss
}

func encodeFragments(w io.Writer, f *object.Fragments, v term.Level) error {
	if _, err := fmt.Fprintf(w, "%s: %s %s: %s\n",
		v.Blue("origin"), v.Green(f.Origin.String()),
		v.Blue("size"), v.Green(strconv.FormatUint(f.Size, 10))); err != nil {
		return err
	}
	padding := indexPadding(f)
	for _, e := range f.Entries {
		if _, err := fmt.Fprintf(w, "%s %s\t%d\n", e.Hash, fragmentIndexPadding(e, padding), e.Size); err != nil {
			return err
		}
	}
	return nil
}

const (
	fragmentsName = "fragments"
)

func encodeTree(w io.Writer, t *object.Tree, v term.Level) error {
	p := 0
	if v != term.LevelNone && slices.IndexFunc(t.Entries, func(e *object.TreeEntry) bool { return e.IsFragments() }) != -1 {
		p = len(fragmentsName) // commit
	}
	padding := t.SizePadding()
	for _, e := range t.Entries {
		if err := encodeEntry(w, e, typePadding(e, p), sizePadding(e, padding), v); err != nil {
			return err
		}
	}
	return nil
}

func encodeTag(w io.Writer, t *object.Tag, v term.Level) error {
	headers := []string{
		fmt.Sprintf("%s %s", v.Blue("object"), v.Green(t.Object.String())),
		fmt.Sprintf("%s %s", v.Blue("type"), v.Green(t.ObjectType.String())),
		fmt.Sprintf("%s %s", v.Blue("tag"), v.Green(t.Name)),
		fmt.Sprintf("%s %s", v.Blue("tagger"), v.Green(t.Tagger.String())),
	}
	_, err := fmt.Fprintf(w, "%s\n\n%s", strings.Join(headers, "\n"), t.Content)
	return err
}

func encodeCommit(w io.Writer, c *object.Commit, v term.Level) (err error) {
	if _, err = fmt.Fprintf(w, "%s %s\n", v.Blue("tree"), v.Green(c.Tree.String())); err != nil {
		return err
	}

	for _, parent := range c.Parents {
		if _, err = fmt.Fprintf(w, "%s %s\n", v.Blue("parent"), v.Green(parent.String())); err != nil {
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
	case *object.Commit:
		return encodeCommit(w, o, v)
	case *object.Tag:
		return encodeTag(w, o, v)
	case *object.Tree:
		return encodeTree(w, o, v)
	case *object.Fragments:
		return encodeFragments(w, o, v)
	}
	_, err := fmt.Fprintln(w, a)
	return err
}
