package patchview

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/exp/charmtone"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/zeebo/xxh3"
)

const (
	// defaultCacheSize is the default cache size.
	defaultCacheSize = 1000
	// maxSourceLenForCache is the maximum source length allowed for caching.
	maxSourceLenForCache = 10000
	// tabSpaces is the number of spaces to replace tabs with.
	tabSpaces = "    " // 4 spaces
)

// SyntaxHighlighter is a syntax highlighter.
type SyntaxHighlighter struct {
	style *chroma.Style

	cachedLexer    chroma.Lexer
	cachedFilename string

	// cache is nil only when lru.New fails, which can only happen for
	// non-positive sizes. Since defaultCacheSize is a positive constant,
	// this is effectively never nil; callers still nil-check for safety
	// so Highlight can transparently fall back to the uncached path.
	cache *lru.Cache[uint64, string]
}

// NewSyntaxHighlighter creates a syntax highlighter.
// filename: used for language detection
// isDark: whether the background is dark
func NewSyntaxHighlighter(filename string, isDark bool) *SyntaxHighlighter {
	cache, _ := lru.New[uint64, string](defaultCacheSize)
	h := &SyntaxHighlighter{
		style: getDefaultChromaStyle(isDark),
		cache: cache,
	}

	// Warm up lexer
	if filename != "" {
		h.cachedLexer = lexers.Match(filename)
		if h.cachedLexer != nil {
			h.cachedLexer = chroma.Coalesce(h.cachedLexer)
			h.cachedFilename = filename
		}
	}

	return h
}

// Highlight highlights code.
// source: original code
// bgColor: background color (hex format, e.g. "#303a30")
func (h *SyntaxHighlighter) Highlight(source, bgColor string) string {
	if h.style == nil {
		return source
	}

	// Preprocess: sanitize line (replace tabs, escape control chars)
	source = sanitizeLine(source)

	// Check cache. Skip caching for very long sources to bound memory.
	if h.cache != nil && len(source) <= maxSourceLenForCache {
		cacheKey := h.createCacheKey(source, h.cachedFilename, bgColor)
		if cached, ok := h.cache.Get(cacheKey); ok {
			return cached
		}
		result := h.doHighlight(source, bgColor)
		h.cache.Add(cacheKey, result)
		return result
	}

	return h.doHighlight(source, bgColor)
}

// sanitizeLine processes a line of code:
// - Replaces tabs with spaces
// - Replaces control characters with Unicode Control Picture characters
func sanitizeLine(s string) string {
	var result strings.Builder
	result.Grow(len(s) + len(s)/4) // extra space for tab expansion

	for _, r := range s {
		switch {
		case r == '\t':
			result.WriteString(tabSpaces)
		case r == 0x7F:
			result.WriteRune('\u2421') // DEL -> ␡
		case r >= 0x00 && r <= 0x1F:
			result.WriteRune('\u2400' + r) // Control chars -> Unicode Control Picture
		default:
			result.WriteRune(r)
		}
	}

	return result.String()
}

// doHighlight performs actual highlighting.
func (h *SyntaxHighlighter) doHighlight(source, bgColor string) string {
	lexer := h.cachedLexer
	if lexer == nil {
		return source
	}

	it, err := lexer.Tokenise(nil, source)
	if err != nil {
		return source
	}

	var b strings.Builder
	formatter := newDiffFormatter(bgColor)
	if err := formatter.Format(&b, h.style, it); err != nil {
		return source
	}

	return b.String()
}

// createCacheKey creates a cache key.
func (h *SyntaxHighlighter) createCacheKey(source, filename, bgColor string) uint64 {
	hh := xxh3.New()
	_, _ = hh.WriteString(filename)
	_, _ = hh.Write([]byte{0})
	_, _ = hh.WriteString(bgColor)
	_, _ = hh.Write([]byte{0})
	_, _ = hh.WriteString(source)
	return hh.Sum64()
}

// ClearCache clears the cache.
func (h *SyntaxHighlighter) ClearCache() {
	if h.cache != nil {
		h.cache.Purge()
	}
}

// Enabled returns whether the highlighter is enabled.
func (h *SyntaxHighlighter) Enabled() bool {
	return h.style != nil
}

// diffFormatter is a Chroma formatter that forces background color.
type diffFormatter struct {
	bgColor string
}

func newDiffFormatter(bgColor string) *diffFormatter {
	return &diffFormatter{
		bgColor: bgColor,
	}
}

func (f *diffFormatter) Format(w io.Writer, style *chroma.Style, it chroma.Iterator) error {
	for token := it(); token != chroma.EOF; token = it() {
		value := strings.TrimRight(token.Value, "\n")
		if value == "" {
			continue
		}

		entry := style.Get(token.Type)
		if entry.IsZero() {
			_, _ = fmt.Fprint(w, value)
			continue
		}

		s := lipgloss.NewStyle().Background(lipgloss.Color(f.bgColor))
		if entry.Bold == chroma.Yes {
			s = s.Bold(true)
		}
		if entry.Underline == chroma.Yes {
			s = s.Underline(true)
		}
		if entry.Italic == chroma.Yes {
			s = s.Italic(true)
		}
		if entry.Colour.IsSet() {
			s = s.Foreground(lipgloss.Color(entry.Colour.String()))
		}

		_, _ = fmt.Fprint(w, s.Render(value))
	}
	return nil
}

// getDefaultChromaStyle returns a theme suitable for terminal background.
// Dark theme uses charmtone palette.
// Light theme uses catppuccin-latte.
func getDefaultChromaStyle(isDark bool) *chroma.Style {
	if isDark {
		// Dark theme: charmtone palette
		return chroma.MustNewStyle("zeta-charmtone-dark", chroma.StyleEntries{
			chroma.Text:                charmtone.Smoke.Hex() + " bg:" + charmtone.Char.Hex(),
			chroma.Error:               charmtone.Butter.Hex() + " bg:" + charmtone.Sriracha.Hex(),
			chroma.Comment:             charmtone.Oyster.Hex(),
			chroma.CommentPreproc:      charmtone.Bengal.Hex(),
			chroma.Keyword:             charmtone.Malibu.Hex(),
			chroma.KeywordReserved:     charmtone.Pony.Hex(),
			chroma.KeywordNamespace:    charmtone.Pony.Hex(),
			chroma.KeywordType:         charmtone.Guppy.Hex(),
			chroma.Operator:            charmtone.Salmon.Hex(),
			chroma.Punctuation:         charmtone.Zest.Hex(),
			chroma.Name:                charmtone.Smoke.Hex(),
			chroma.NameBuiltin:         charmtone.Cheeky.Hex(),
			chroma.NameTag:             charmtone.Mauve.Hex(),
			chroma.NameAttribute:       charmtone.Hazy.Hex(),
			chroma.NameClass:           "underline bold " + charmtone.Salt.Hex(),
			chroma.NameConstant:        charmtone.Salt.Hex(),
			chroma.NameDecorator:       charmtone.Citron.Hex(),
			chroma.NameException:       charmtone.Coral.Hex(),
			chroma.NameFunction:        charmtone.Guac.Hex(),
			chroma.NameOther:           charmtone.Smoke.Hex(),
			chroma.Literal:             charmtone.Smoke.Hex(),
			chroma.LiteralNumber:       charmtone.Julep.Hex(),
			chroma.LiteralDate:         charmtone.Salt.Hex(),
			chroma.LiteralString:       charmtone.Cumin.Hex(),
			chroma.LiteralStringEscape: charmtone.Bok.Hex(),
			chroma.GenericDeleted:      charmtone.Coral.Hex(),
			chroma.GenericEmph:         "italic",
			chroma.GenericInserted:     charmtone.Guac.Hex(),
			chroma.GenericStrong:       "bold",
			chroma.GenericSubheading:   charmtone.Squid.Hex(),
			chroma.Background:          "bg:" + charmtone.Char.Hex(),
		})
	}
	// Light theme: catppuccin-latte
	return styles.Get("catppuccin-latte")
}
