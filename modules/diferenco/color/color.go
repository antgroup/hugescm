package color

// TODO read colors from a github.com/go-git/go-git/plumbing/format/config.Config struct
// TODO implement color parsing, see https://github.com/git/git/blob/v2.26.2/color.c

// Colors. See https://github.com/git/git/blob/v2.26.2/color.h#L24-L53.
const (
	Normal       = ""
	Reset        = "\033[m"
	Bold         = "\033[1m"
	Red          = "\033[31m"
	Green        = "\033[32m"
	Yellow       = "\033[33m"
	Blue         = "\033[34m"
	Magenta      = "\033[35m"
	Cyan         = "\033[36m"
	BoldRed      = "\033[1;31m"
	BoldGreen    = "\033[1;32m"
	BoldYellow   = "\033[1;33m"
	BoldBlue     = "\033[1;34m"
	BoldMagenta  = "\033[1;35m"
	BoldCyan     = "\033[1;36m"
	FaintRed     = "\033[2;31m"
	FaintGreen   = "\033[2;32m"
	FaintYellow  = "\033[2;33m"
	FaintBlue    = "\033[2;34m"
	FaintMagenta = "\033[2;35m"
	FaintCyan    = "\033[2;36m"
	BgRed        = "\033[41m"
	BgGreen      = "\033[42m"
	BgYellow     = "\033[43m"
	BgBlue       = "\033[44m"
	BgMagenta    = "\033[45m"
	BgCyan       = "\033[46m"
	Faint        = "\033[2m"
	FaintItalic  = "\033[2;3m"
	Reverse      = "\033[7m"
)

// A ColorKey is a key into a ColorConfig map and also equal to the key in the
// diff.color subsection of the config. See
// https://github.com/git/git/blob/v2.26.2/diff.c#L83-L106.
type ColorKey string

// ColorKeys.
const (
	Context                   ColorKey = "context"
	Meta                      ColorKey = "meta"
	Frag                      ColorKey = "frag"
	Old                       ColorKey = "old"
	New                       ColorKey = "new"
	Commit                    ColorKey = "commit"
	Whitespace                ColorKey = "whitespace"
	Func                      ColorKey = "func"
	OldMoved                  ColorKey = "oldMoved"
	OldMovedAlternative       ColorKey = "oldMovedAlternative"
	OldMovedDimmed            ColorKey = "oldMovedDimmed"
	OldMovedAlternativeDimmed ColorKey = "oldMovedAlternativeDimmed"
	NewMoved                  ColorKey = "newMoved"
	NewMovedAlternative       ColorKey = "newMovedAlternative"
	NewMovedDimmed            ColorKey = "newMovedDimmed"
	NewMovedAlternativeDimmed ColorKey = "newMovedAlternativeDimmed"
	ContextDimmed             ColorKey = "contextDimmed"
	OldDimmed                 ColorKey = "oldDimmed"
	NewDimmed                 ColorKey = "newDimmed"
	ContextBold               ColorKey = "contextBold"
	OldBold                   ColorKey = "oldBold"
	NewBold                   ColorKey = "newBold"
)

// A ColorConfig is a color configuration. A nil or empty ColorConfig
// corresponds to no color.
type ColorConfig map[ColorKey]string

// A ColorConfigOption sets an option on a ColorConfig.
type ColorConfigOption func(ColorConfig)

// WithColor sets the color for key.
func WithColor(key ColorKey, color string) ColorConfigOption {
	return func(cc ColorConfig) {
		cc[key] = color
	}
}

// defaultColorConfig is the default color configuration. See
// https://github.com/git/git/blob/v2.26.2/diff.c#L57-L81.
var defaultColorConfig = ColorConfig{
	Context:                   Normal,
	Meta:                      Bold,
	Frag:                      Cyan,
	Old:                       Red,
	New:                       Green,
	Commit:                    Yellow,
	Whitespace:                BgRed,
	Func:                      Normal,
	OldMoved:                  BoldMagenta,
	OldMovedAlternative:       BoldBlue,
	OldMovedDimmed:            Faint,
	OldMovedAlternativeDimmed: FaintItalic,
	NewMoved:                  BoldCyan,
	NewMovedAlternative:       BoldYellow,
	NewMovedDimmed:            Faint,
	NewMovedAlternativeDimmed: FaintItalic,
	ContextDimmed:             Faint,
	OldDimmed:                 FaintRed,
	NewDimmed:                 FaintGreen,
	ContextBold:               Bold,
	OldBold:                   BoldRed,
	NewBold:                   BoldGreen,
}

// NewColorConfig returns a new ColorConfig.
func NewColorConfig(options ...ColorConfigOption) ColorConfig {
	cc := make(ColorConfig)
	for key, value := range defaultColorConfig {
		cc[key] = value
	}
	for _, option := range options {
		option(cc)
	}
	return cc
}

// Reset returns the ANSI escape sequence to reset the color with key set from
// cc. If no color was set then no reset is needed so it returns the empty
// string.
func (cc ColorConfig) Reset(key ColorKey) string {
	if cc[key] == "" {
		return ""
	}
	return Reset
}
