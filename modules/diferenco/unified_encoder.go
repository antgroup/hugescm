package diferenco

import (
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco/color"
)

const (
	ZERO_OID_MAX = "0000000000000000000000000000000000000000000000000000000000000000" // ZERO OID MAX
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
	vcs       string
	noRename  bool
	// colorConfig is the color configuration. The default is no color.
	color color.ColorConfig
}

// EncoderOption sets an option on UnifiedEncoder.
type EncoderOption func(*UnifiedEncoder)

// WithVCS sets the VCS name for the encoder.
func WithVCS(vcs string) EncoderOption {
	return func(e *UnifiedEncoder) {
		if vcs != "" {
			e.vcs = vcs
		}
	}
}

// WithColor sets the color configuration.
func WithColor(cc color.ColorConfig) EncoderOption {
	return func(e *UnifiedEncoder) {
		e.color = cc
	}
}

// WithSrcPrefix sets the source prefix for file paths.
func WithSrcPrefix(prefix string) EncoderOption {
	return func(e *UnifiedEncoder) {
		e.srcPrefix = prefix
	}
}

// WithDstPrefix sets the destination prefix for file paths.
func WithDstPrefix(prefix string) EncoderOption {
	return func(e *UnifiedEncoder) {
		e.dstPrefix = prefix
	}
}

// WithNoRename disables rename detection in the output.
func WithNoRename() EncoderOption {
	return func(e *UnifiedEncoder) {
		e.noRename = true
	}
}

// NewUnifiedEncoder returns a new UnifiedEncoder that writes to w.
func NewUnifiedEncoder(w io.Writer, opts ...EncoderOption) *UnifiedEncoder {
	e := &UnifiedEncoder{
		Writer:    w,
		srcPrefix: "a/",
		dstPrefix: "b/",
		vcs:       "zeta",
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

func (e *UnifiedEncoder) Encode(patches []*Patch) error {
	for _, p := range patches {
		if err := e.writePatch(p); err != nil {
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

func (e *UnifiedEncoder) writeFilePatchHeader(p *Patch, b *strings.Builder) {
	from, to := p.From, p.To
	if from == nil && to == nil {
		return
	}
	var lines []string
	switch {
	case from != nil && to != nil:
		hashEquals := from.Hash == to.Hash
		lines = append(lines,
			fmt.Sprintf("diff --%s %s%s %s%s",
				e.vcs, e.srcPrefix, from.Name, e.dstPrefix, to.Name),
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
			lines = e.appendPathLines(lines, e.srcPrefix+from.Name, e.dstPrefix+to.Name, p.IsBinary, p.IsFragments)
		}
	case from == nil:
		lines = append(lines,
			fmt.Sprintf("diff --%s %s %s", e.vcs, e.srcPrefix+to.Name, e.dstPrefix+to.Name),
			fmt.Sprintf("new file mode %o", to.Mode),
			fmt.Sprintf("index %s..%s", ZERO_OID_MAX[0:min(len(to.Hash), len(ZERO_OID_MAX))], to.Hash),
		)
		lines = e.appendPathLines(lines, "/dev/null", e.dstPrefix+to.Name, p.IsBinary, p.IsFragments)
	case to == nil:
		lines = append(lines,
			fmt.Sprintf("diff --%s %s %s", e.vcs, e.srcPrefix+from.Name, e.dstPrefix+from.Name),
			fmt.Sprintf("deleted file mode %o", from.Mode),
			fmt.Sprintf("index %s..%s", from.Hash, ZERO_OID_MAX[0:min(len(from.Hash), len(ZERO_OID_MAX))]),
		)
		lines = e.appendPathLines(lines, e.srcPrefix+from.Name, "/dev/null", p.IsBinary, p.IsFragments)
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
		_, _ = b.WriteString(" -0,0")
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
	if before, ok := strings.CutSuffix(o.Content, "\n"); ok {
		_, _ = b.WriteString(before)
		_, _ = b.WriteString(e.color.Reset(colorKey))
		_ = b.WriteByte('\n')
		return
	}
	_, _ = b.WriteString(o.Content)
	_, _ = b.WriteString(e.color.Reset(colorKey))
	_, _ = b.WriteString("\n\\ No newline at end of file\n")
}

func (e *UnifiedEncoder) writePatch(p *Patch) error {
	b := &strings.Builder{}
	if len(p.Message) != 0 {
		_, _ = b.WriteString(p.Message)
		if !strings.HasSuffix(p.Message, "\n") {
			_ = b.WriteByte('\n')
		}
	}
	e.writeFilePatchHeader(p, b)
	if len(p.Hunks) == 0 {
		if _, err := io.WriteString(e.Writer, b.String()); err != nil {
			return err
		}
		return nil
	}
	for _, hunk := range p.Hunks {
		e.writePatchHunk(b, hunk)
	}
	if _, err := io.WriteString(e.Writer, b.String()); err != nil {
		return err
	}
	return nil
}
