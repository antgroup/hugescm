package item

import (
	"charm.land/lipgloss/v2"
)

// ByteRange represents a range of bytes
type ByteRange struct {
	Start, End int
}

// WidthRange represents a range of character widths in terminal cells
type WidthRange struct {
	Start, End int
}

// Match represents a range of bytes and their according start and end width in an item
type Match struct {
	ByteRange  ByteRange
	WidthRange WidthRange
}

// Highlight represents a range and style to highlight
type Highlight struct {
	Style                    lipgloss.Style
	ByteRangeUnstyledContent ByteRange
}
