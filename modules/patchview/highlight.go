package patchview

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"charm.land/lipgloss/v2"
	"github.com/alecthomas/chroma/v2"
	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/alecthomas/chroma/v2/styles"
	"github.com/charmbracelet/x/exp/charmtone"
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

// lruCache is an LRU cache implementation.
type lruCache struct {
	mu       sync.Mutex
	items    map[uint64]*lruItem
	head     *lruItem
	tail     *lruItem
	capacity int
}

type lruItem struct {
	key   uint64
	value string
	prev  *lruItem
	next  *lruItem
}

func newLRUCache(capacity int) *lruCache {
	return &lruCache{
		items:    make(map[uint64]*lruItem),
		capacity: capacity,
	}
}

func (c *lruCache) get(key uint64) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if item, ok := c.items[key]; ok {
		c.moveToFrontLocked(item)
		return item.value, true
	}
	return "", false
}

func (c *lruCache) set(key uint64, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if item, ok := c.items[key]; ok {
		item.value = value
		c.moveToFrontLocked(item)
		return
	}
	item := &lruItem{key: key, value: value}
	c.items[key] = item
	if c.head == nil {
		c.head = item
		c.tail = item
	} else {
		item.next = c.head
		c.head.prev = item
		c.head = item
	}
	if c.capacity > 0 && len(c.items) > c.capacity {
		c.evictTailLocked()
	}
}

func (c *lruCache) moveToFrontLocked(item *lruItem) {
	if item == c.head {
		return
	}
	if item.prev != nil {
		item.prev.next = item.next
	}
	if item.next != nil {
		item.next.prev = item.prev
	}
	if item == c.tail {
		c.tail = item.prev
	}
	item.prev = nil
	item.next = c.head
	c.head.prev = item
	c.head = item
}

func (c *lruCache) evictTailLocked() {
	if c.tail == nil {
		return
	}
	delete(c.items, c.tail.key)
	if c.tail.prev != nil {
		c.tail.prev.next = nil
		c.tail = c.tail.prev
	} else {
		c.head = nil
		c.tail = nil
	}
}

func (c *lruCache) clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[uint64]*lruItem)
	c.head = nil
	c.tail = nil
}

// SyntaxHighlighter is a syntax highlighter.
type SyntaxHighlighter struct {
	style *chroma.Style

	cachedLexer    chroma.Lexer
	cachedFilename string

	cache        *lruCache
	cacheEnabled bool
}

// NewSyntaxHighlighter creates a syntax highlighter.
// filename: used for language detection
// isDark: whether the background is dark
func NewSyntaxHighlighter(filename string, isDark bool) *SyntaxHighlighter {
	h := &SyntaxHighlighter{
		style:        getDefaultChromaStyle(isDark),
		cache:        newLRUCache(defaultCacheSize),
		cacheEnabled: true,
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

	// Check cache
	if h.cacheEnabled && len(source) <= maxSourceLenForCache {
		cacheKey := h.createCacheKey(source, h.cachedFilename, bgColor)
		if cached, ok := h.cache.get(cacheKey); ok {
			return cached
		}
		result := h.doHighlight(source, bgColor)
		h.cache.set(cacheKey, result)
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
	h.cache.clear()
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
			fmt.Fprint(w, value)
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

		fmt.Fprint(w, s.Render(value))
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
			chroma.Text:                charmtone.Smoke.Hex() + " bg:" + charmtone.Charcoal.Hex(),
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
			chroma.Background:          "bg:" + charmtone.Charcoal.Hex(),
		})
	}
	// Light theme: catppuccin-latte
	return styles.Get("catppuccin-latte")
}
