package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/formatters"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/antgroup/hugescm/cmd/hot/pkg/hud"
	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/hexview"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/modules/tui"
)

const (
	MAX_SHOW_BINARY_BLOB = 10<<20 - 8
)

type Cat struct {
	Object   string `arg:"" name:"object" help:"The name of the object to show"`
	CWD      string `short:"C" name:"cwd" help:"Specify repository location" default:"." type:"path"`
	Type     bool   `name:"type" short:"t" help:"Show object type"`
	Size     bool   `name:"size" short:"s" help:"Show object size"`
	Textconv bool   `name:"textconv" help:"Converting text to Unicode"`
	JSON     bool   `name:"json" short:"j" help:"Returns data as JSON; limited to commits, trees, and tags"`
	Limit    int64  `name:"limit" short:"L" help:"Omits blobs larger than n bytes or units. n may be zero. supported units: KB,MB,GB,K,M,G" default:"-1" type:"size"`
	Output   string `name:"output" help:"Output to a specific file instead of stdout" placeholder:"<file>"`
}

func (c *Cat) Run(g *Globals) error {
	repoPath := git.RevParseRepoPath(context.Background(), c.CWD)
	trace.DbgPrint("repository location: %v", repoPath)
	d, err := git.NewDecoder(context.Background(), repoPath)
	if err != nil {
		die("new git decoder error: %v", err)
		return err
	}
	defer d.Close() // nolint
	o, err := d.Object(c.Object)
	if err != nil {
		die("open '%s' error: %v\n", c.Object, err)
		return err
	}
	if oo, ok := o.(*git.Object); ok {
		return c.formatObject(oo)
	}
	return c.showObject(o)
}

func (c *Cat) Println(a ...any) error {
	fd, _, err := c.NewFD()
	if err != nil {
		return err
	}
	defer fd.Close() // nolint
	_, err = fmt.Fprintln(fd, a...)
	return err
}

func (c *Cat) NewFD() (io.WriteCloser, term.Level, error) {
	if len(c.Output) == 0 {
		return &NopWriteCloser{Writer: os.Stdout}, term.StdoutLevel, nil
	}
	fd, err := os.Create(c.Output)
	return fd, term.LevelNone, err
}

const (
	binaryTruncated = "*** Binary truncated ***"
)

type sizer interface {
	Size() int64
}

func (c *Cat) showObject(a any) error {
	if c.Size {
		if s, ok := a.(sizer); ok {
			return c.Println(s.Size())
		}
		return nil
	}
	if c.Type {
		switch a.(type) {
		case *git.Commit:
			return c.Println("commit")
		case *git.Tag:
			return c.Println("tag")
		case *git.Tree:
			return c.Println("tree")
		}
		return nil
	}
	if c.JSON {
		fd, _, err := c.NewFD()
		if err != nil {
			return err
		}
		defer fd.Close() // nolint
		return json.NewEncoder(fd).Encode(a)
	}
	fd, termLevel, err := c.NewFD()
	if err != nil {
		return err
	}
	defer fd.Close() // nolint
	return hud.Display(fd, a, termLevel)
}

func (c *Cat) isMarkdown() bool {
	if _, filename, ok := strings.Cut(c.Object, ":"); ok {
		return strings.EqualFold(filename, "README") || strings.EqualFold(filepath.Ext(filename), ".md")
	}
	return false
}

func (c *Cat) getLexer() chroma.Lexer {
	_, filename, ok := strings.Cut(c.Object, ":")
	if !ok {
		return nil
	}
	lexer := lexers.Match(filename)
	return lexer
}

var termWidth = func() (width int, err error) {
	width, _, err = term.GetSize(int(os.Stdout.Fd()))
	if err == nil {
		return width, nil
	}

	return 0, err
}

func (c *Cat) markdownOut(w io.Writer, input io.Reader) error {
	width, _ := termWidth()
	if width == 0 || width > 120 {
		width = 80
	}
	// Detect background color to pick appropriate style
	style := "light"
	if lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
		style = "dark"
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(style),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return err
	}
	defer func() { _ = r.Close() }()
	// Write input to renderer
	if _, err = io.Copy(r, input); err != nil {
		return err
	}
	// Close to trigger rendering
	if err = r.Close(); err != nil {
		return err
	}
	// Write the rendered output to the destination
	if _, err = io.Copy(w, r); err != nil {
		return err
	}

	// Ensure proper termination for pager compatibility
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}

	return nil
}

func (c *Cat) syntaxHighlightOut(w io.Writer, input io.Reader, termLevel term.Level, lexer chroma.Lexer) error {
	// Read the input into a buffer
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, input); err != nil {
		return err
	}
	content := buf.String()

	// Coalesce the lexer
	lexer = chroma.Coalesce(lexer)

	// Detect background color to pick appropriate style
	styleName := "github"
	if lipgloss.HasDarkBackground(os.Stdin, os.Stdout) {
		styleName = "dracula"
	}

	// Get the style
	style := styles.Get(styleName)
	if style == nil {
		style = styles.Fallback
	}

	// Tokenize the content
	iterator, err := lexer.Tokenise(nil, content)
	if err != nil {
		return err
	}

	// Choose formatter based on terminal color support level
	var formatter chroma.Formatter
	switch termLevel {
	case term.Level16M:
		formatter = formatters.TTY16m
	case term.Level256:
		formatter = formatters.TTY256
	case term.LevelNone:
		formatter = formatters.NoOp
	default:
		formatter = formatters.TTY
	}

	if err := formatter.Format(w, style, iterator); err != nil {
		return err
	}

	// Ensure proper termination for pager compatibility
	if f, ok := w.(interface{ Flush() error }); ok {
		_ = f.Flush()
	}

	return nil
}

func (c *Cat) formatObject(o *git.Object) error {
	if c.Size {
		return c.Println(o.Size)
	}
	if c.Type {
		return c.Println("blob")
	}
	reader, charset, err := diferenco.NewUnifiedReaderEx(o, c.Textconv)
	if err != nil {
		return err
	}
	if c.Limit < 0 {
		c.Limit = o.Size
	}

	// Check if we should use pager (small files, no output file, color support)
	usePager := len(c.Output) == 0 && term.StdoutLevel != term.LevelNone && o.Size <= MAX_SHOW_BINARY_BLOB

	// Binary content: always use hexview, with or without pager
	if charset == diferenco.BINARY {
		if c.Limit > MAX_SHOW_BINARY_BLOB {
			reader = io.MultiReader(io.LimitReader(reader, MAX_SHOW_BINARY_BLOB), strings.NewReader(binaryTruncated))
			c.Limit = int64(MAX_SHOW_BINARY_BLOB + len(binaryTruncated))
		}

		if usePager {
			p := tui.NewPager(term.StdoutLevel)
			defer p.Close() // nolint
			return hexview.Format(reader, p, c.Limit, p.ColorMode())
		}

		fd, _, err := c.NewFD()
		if err != nil {
			return err
		}
		defer fd.Close() // nolint
		return hexview.Format(reader, fd, c.Limit, term.StdoutLevel)
	}

	// Markdown and source code: only with pager
	if usePager {
		// Markdown handling
		if c.isMarkdown() {
			p := tui.NewPager(term.StdoutLevel)
			defer p.Close() // nolint
			return c.markdownOut(p, io.LimitReader(reader, c.Limit))
		}

		// Source code handling
		if lexer := c.getLexer(); lexer != nil {
			p := tui.NewPager(term.StdoutLevel)
			defer p.Close() // nolint
			return c.syntaxHighlightOut(p, io.LimitReader(reader, c.Limit), p.ColorMode(), lexer)
		}
	}

	// Default: output directly (large files or output to file)
	fd, _, err := c.NewFD()
	if err != nil {
		return err
	}
	defer fd.Close() // nolint

	if _, err = io.Copy(fd, io.LimitReader(reader, c.Limit)); err != nil {
		return err
	}
	return nil
}
