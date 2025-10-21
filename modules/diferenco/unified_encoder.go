package diferenco

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco/color"
)

const (
	ZERO_OID = "0000000000000000000000000000000000000000000000000000000000000000" // zeta zero OID
)

var (
	operationChar = map[Operation]byte{
		Insert: '+',
		Delete: '-',
		Equal:  ' ',
	}

	operationColorKey = map[Operation]color.ColorKey{
		Insert: color.New,
		Delete: color.Old,
		Equal:  color.Context,
	}
)

// UnifiedEncoder encodes an unified diff into the provided Writer. It does not
// support similarity index for renames or sorting hash representations.
type UnifiedEncoder struct {
	io.Writer

	// srcPrefix and dstPrefix are prepended to file paths when encoding a diff.
	srcPrefix string
	dstPrefix string
	noRename  bool
	// colorConfig is the color configuration. The default is no color.
	color color.ColorConfig
}

// NewUnifiedEncoder returns a new UnifiedEncoder that writes to w.
func NewUnifiedEncoder(w io.Writer) *UnifiedEncoder {
	return &UnifiedEncoder{
		Writer:    w,
		srcPrefix: "a/",
		dstPrefix: "b/",
	}
}

// SetColor sets e's color configuration and returns e.
func (e *UnifiedEncoder) SetColor(colorConfig color.ColorConfig) *UnifiedEncoder {
	e.color = colorConfig
	return e
}

// SetSrcPrefix sets e's srcPrefix and returns e.
func (e *UnifiedEncoder) SetSrcPrefix(prefix string) *UnifiedEncoder {
	e.srcPrefix = prefix
	return e
}

// SetDstPrefix sets e's dstPrefix and returns e.
func (e *UnifiedEncoder) SetDstPrefix(prefix string) *UnifiedEncoder {
	e.dstPrefix = prefix
	return e
}

func (e *UnifiedEncoder) SetNoRename() *UnifiedEncoder {
	e.noRename = true
	return e
}

func (e *UnifiedEncoder) Encode(patches []*Unified) error {
	for _, u := range patches {
		if err := e.writeUnified(u); err != nil {
			return err
		}
	}
	return nil
}

func (e *UnifiedEncoder) appendPathLines(lines []string, fromPath, toPath string, isBinary bool, isFragments bool) []string {
	if isFragments {
		return append(lines,
			fmt.Sprintf("Fragments files %s and %s differ", fromPath, toPath),
		)
	}
	if isBinary {
		return append(lines,
			fmt.Sprintf("Binary files %s and %s differ", fromPath, toPath),
		)
	}
	return append(lines,
		"--- "+fromPath,
		"+++ "+toPath,
	)
}

func (e *UnifiedEncoder) writeFilePatchHeader(u *Unified, b *strings.Builder) {
	from, to := u.From, u.To
	if from == nil && to == nil {
		return
	}
	var lines []string
	switch {
	case from != nil && to != nil:
		hashEquals := from.Hash == to.Hash
		lines = append(lines,
			fmt.Sprintf("diff --zeta %s%s %s%s",
				e.srcPrefix, from.Name, e.dstPrefix, to.Name),
		)
		if from.Mode != to.Mode {
			lines = append(lines,
				fmt.Sprintf("old mode %o", from.Mode),
				fmt.Sprintf("new mode %o", to.Mode),
			)
		}
		if !e.noRename {
			if from.Name != to.Name {
				lines = append(lines,
					"rename from "+from.Name,
					"rename to "+to.Name,
				)
			}
		}
		if from.Mode != to.Mode && !hashEquals {
			lines = append(lines,
				fmt.Sprintf("index %s..%s", from.Hash, to.Hash),
			)
		} else if !hashEquals {
			lines = append(lines,
				fmt.Sprintf("index %s..%s %o", from.Hash, to.Hash, from.Mode),
			)
		}
		if !hashEquals {
			lines = e.appendPathLines(lines, e.srcPrefix+from.Name, e.dstPrefix+to.Name, u.IsBinary, u.IsFragments)
		}
	case from == nil:
		lines = append(lines,
			fmt.Sprintf("diff --zeta %s %s", e.srcPrefix+to.Name, e.dstPrefix+to.Name),
			fmt.Sprintf("new file mode %o", to.Mode),
			fmt.Sprintf("index %s..%s", ZERO_OID, to.Hash),
		)
		lines = e.appendPathLines(lines, "/dev/null", e.dstPrefix+to.Name, u.IsBinary, u.IsFragments)
	case to == nil:
		lines = append(lines,
			fmt.Sprintf("diff --zeta %s %s", e.srcPrefix+from.Name, e.dstPrefix+from.Name),
			fmt.Sprintf("deleted file mode %o", from.Mode),
			fmt.Sprintf("index %s..%s", from.Hash, ZERO_OID),
		)
		lines = e.appendPathLines(lines, e.srcPrefix+from.Name, "/dev/null", u.IsBinary, u.IsFragments)
	}
	b.WriteString(e.color[color.Meta])
	b.WriteString(lines[0])
	for _, line := range lines[1:] {
		b.WriteByte('\n')
		b.WriteString(line)
	}
	b.WriteString(e.color.Reset(color.Meta))
	b.WriteByte('\n')
}

func (e *UnifiedEncoder) writePatchHunk(b *strings.Builder, hunk *Hunk) {
	fromCount, toCount := 0, 0
	for _, l := range hunk.Lines {
		switch l.Kind {
		case Delete:
			fromCount++
		case Insert:
			toCount++
		default:
			fromCount++
			toCount++
		}
	}
	_, _ = b.WriteString(e.color[color.Frag])
	_, _ = b.WriteString("@@")
	if fromCount > 1 {
		_, _ = b.WriteString(" -")
		_, _ = b.WriteString(strconv.Itoa(hunk.FromLine))
		_ = b.WriteByte(',')
		_, _ = b.WriteString(strconv.Itoa(fromCount))
	} else if hunk.FromLine == 1 && fromCount == 0 {
		// Match odd GNU diff -u behavior adding to empty file.
		_, _ = b.WriteString(" +0,0")
	} else {
		_, _ = b.WriteString(" -")
		_, _ = b.WriteString(strconv.Itoa(hunk.FromLine))
	}
	if toCount > 1 {
		_, _ = b.WriteString(" +")
		_, _ = b.WriteString(strconv.Itoa(hunk.ToLine))
		_ = b.WriteByte(',')
		_, _ = b.WriteString(strconv.Itoa(toCount))
	} else if hunk.ToLine == 1 && toCount == 0 {
		// Match odd GNU diff -u behavior adding to empty file.
		_, _ = b.WriteString(" +0,0")
	} else {
		_, _ = b.WriteString(" +")
		_, _ = b.WriteString(strconv.Itoa(hunk.ToLine))
	}
	_, _ = b.WriteString(" @@")
	_, _ = b.WriteString(e.color.Reset(color.Frag))
	_ = b.WriteByte('\n')
	for _, line := range hunk.Lines {
		e.writeLine(b, &line)
	}
}

func (e *UnifiedEncoder) writeLine(b *strings.Builder, o *Line) {
	colorKey := operationColorKey[o.Kind]
	_, _ = b.WriteString(e.color[colorKey])
	_ = b.WriteByte(operationChar[o.Kind])
	if strings.HasSuffix(o.Content, "\n") {
		_, _ = b.WriteString(strings.TrimSuffix(o.Content, "\n"))
		_, _ = b.WriteString(e.color.Reset(colorKey))
		_ = b.WriteByte('\n')
		return
	}
	_, _ = b.WriteString(o.Content)
	_, _ = b.WriteString(e.color.Reset(colorKey))
	_, _ = b.WriteString("\n\\ No newline at end of file\n")
}

func (e *UnifiedEncoder) writeUnified(u *Unified) error {
	b := &strings.Builder{}
	if len(u.Message) != 0 {
		_, _ = b.WriteString(u.Message)
		if !strings.HasSuffix(u.Message, "\n") {
			_ = b.WriteByte('\n')
		}
	}
	e.writeFilePatchHeader(u, b)
	if len(u.Hunks) == 0 {
		if _, err := io.WriteString(e.Writer, b.String()); err != nil {
			return err
		}
		return nil
	}
	for _, hunk := range u.Hunks {
		e.writePatchHunk(b, hunk)
	}
	if _, err := io.WriteString(e.Writer, b.String()); err != nil {
		return err
	}
	return nil
}
