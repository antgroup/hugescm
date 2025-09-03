package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/git"
	"github.com/antgroup/hugescm/modules/hexview"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/modules/trace"
	"github.com/antgroup/hugescm/modules/zeta/object"
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
	if w, ok := a.(object.Printer); ok {
		fd, _, err := c.NewFD()
		if err != nil {
			return err
		}
		defer fd.Close() // nolint
		_ = w.Pretty(fd)
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
	if len(c.Output) == 0 && term.StdoutLevel != term.LevelNone && charset == diferenco.BINARY {
		p := NewPrinter(context.Background())
		defer p.Close() // nolint
		if c.Limit > MAX_SHOW_BINARY_BLOB {
			reader = io.MultiReader(io.LimitReader(reader, MAX_SHOW_BINARY_BLOB), strings.NewReader(binaryTruncated))
			c.Limit = int64(MAX_SHOW_BINARY_BLOB + len(binaryTruncated))
		}
		if err := hexview.Format(reader, p, c.Limit, p.ColorMode()); err != nil && !errors.Is(err, syscall.EPIPE) {
			return err
		}
		return nil
	}
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
