package viewport

import (
	"testing"

	"github.com/antgroup/hugescm/modules/viewport/internal"
)

func TestPreFooterLineWithFooterEnabled(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
	})

	// Without pre-footer: 3 content lines + footer
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// With pre-footer: 2 content lines + pre-footer + footer
	vp.SetPreFooterLine("Pre-footer text")
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"Pre-footer text",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineWithFooterDisabled(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
	})
	vp.SetFooterEnabled(false)

	// Without pre-footer
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"",
		"",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// With pre-footer (still renders even though footer disabled)
	vp.SetPreFooterLine("Pre-footer text")
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"Pre-footer text",
		"",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestEmptyPreFooterLine(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
	})

	// Empty pre-footer means no extra line rendered
	vp.SetPreFooterLine("")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// All 4 content lines visible with footer (height 5 = 4 content + 1 footer)
	if vp.GetPreFooterLine() != "" {
		t.Errorf("expected empty pre-footer line, got %q", vp.GetPreFooterLine())
	}
}

func TestPreFooterLineTruncation(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Set pre-footer longer than viewport width
	vp.SetPreFooterLine("This is a very long pre-footer line that exceeds the width")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"This is a ve...",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineSmallHeight(t *testing.T) {
	w, h := 20, 3
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
	})

	// Height 3 with footer and pre-footer: 1 content + 1 pre-footer + 1 footer
	vp.SetPreFooterLine("Pre-footer")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"Pre-footer",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineWithHeader(t *testing.T) {
	w, h := 30, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"Header"})
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
	})

	// Height 6 with header, pre-footer, footer: 1 header + 3 content + 1 pre-footer + 1 footer
	vp.SetPreFooterLine("Pre-footer")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Header",
		"line 1",
		"line 2",
		"line 3",
		"Pre-footer",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineDynamicToggle(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
	})

	// Initially no pre-footer
	expectedNoPreFooter := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedNoPreFooter, vp.View())

	// Set pre-footer
	vp.SetPreFooterLine("Pre-footer")
	expectedWithPreFooter := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"Pre-footer",
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedWithPreFooter, vp.View())

	// Remove pre-footer
	vp.SetPreFooterLine("")
	internal.CmpStr(t, expectedNoPreFooter, vp.View())

	// Set pre-footer again
	vp.SetPreFooterLine("Different pre-footer")
	expectedDifferent := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"Different pre-footer",
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedDifferent, vp.View())
}

func TestPreFooterLineGetterSetter(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)

	// Initially empty
	if got := vp.GetPreFooterLine(); got != "" {
		t.Errorf("expected empty pre-footer initially, got %q", got)
	}

	// Set and get
	vp.SetPreFooterLine("Test pre-footer")
	if got := vp.GetPreFooterLine(); got != "Test pre-footer" {
		t.Errorf("expected 'Test pre-footer', got %q", got)
	}

	// Clear and get
	vp.SetPreFooterLine("")
	if got := vp.GetPreFooterLine(); got != "" {
		t.Errorf("expected empty pre-footer after clearing, got %q", got)
	}
}

func TestPreFooterLineReducesContentLines(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
	})

	// Without pre-footer: 4 content lines visible (height 5 - 1 footer = 4)
	expectedNoPreFooter := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"80% (4/5)",
	})
	internal.CmpStr(t, expectedNoPreFooter, vp.View())

	// With pre-footer: 3 content lines visible (height 5 - 1 pre-footer - 1 footer = 3)
	vp.SetPreFooterLine("Pre-footer")
	expectedWithPreFooter := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"Pre-footer",
		"60% (3/5)",
	})
	internal.CmpStr(t, expectedWithPreFooter, vp.View())
}

func TestPreFooterLineWithWrap(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h, WithWrapText[object](true))
	setContent(vp, []string{
		"short",
		"longer text that wraps",
	})

	// Pre-footer should appear just above footer, after wrapped content
	vp.SetPreFooterLine("Pre-foot")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"short",
		"longer tex",
		"t that wra",
		"ps",
		"Pre-foot",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineScrolling(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
	})
	vp.SetPreFooterLine("Pre-footer")

	// Initially shows first 3 content lines (height 5 - 1 pre-footer - 1 footer = 3)
	expectedInitial := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"Pre-footer",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedInitial, vp.View())

	// Scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedAfterScroll := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 2",
		"line 3",
		"line 4",
		"Pre-footer",
		"66% (4/6)",
	})
	internal.CmpStr(t, expectedAfterScroll, vp.View())
}

func TestPreFooterLineStyled(t *testing.T) {
	w, h := 30, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Set a styled pre-footer line
	styledPreFooter := internal.RedFg.Render("Red") + " and " + internal.BlueFg.Render("Blue")
	vp.SetPreFooterLine(styledPreFooter)

	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		internal.RedFg.Render("Red") + " and " + internal.BlueFg.Render("Blue"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineStyledTruncation(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Set a styled pre-footer line that exceeds width
	styledPreFooter := internal.RedFg.Render("This is a very long styled text")
	vp.SetPreFooterLine(styledPreFooter)

	// Should truncate with continuation indicator, preserving style
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		internal.RedFg.Render("This is a ve..."),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineDoesNotWrap(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h, WithWrapText[object](true))
	setContent(vp, []string{
		"short",
		"another",
	})

	// Set a long pre-footer line - it should NOT wrap, only truncate
	vp.SetPreFooterLine("This is a very long pre-footer that should not wrap")

	// Pre-footer should be truncated to single line, not wrapped
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"short",
		"another",
		"",
		"",
		"This is...",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineDoesNotWrapWithWrappedContent(t *testing.T) {
	w, h := 10, 7
	vp := newViewport(w, h, WithWrapText[object](true))
	setContent(vp, []string{
		"short",
		"this line wraps to multiple lines",
	})

	// Set a long pre-footer line - it should NOT wrap even when content wraps
	vp.SetPreFooterLine("Long pre-footer text here")

	// Content wraps, but pre-footer should be truncated to single line
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"short",
		"this line ",
		"wraps to m",
		"ultiple li",
		"nes",
		"Long pr...",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineStyledWithWrap(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h, WithWrapText[object](true))
	setContent(vp, []string{
		"short",
		"longer content here",
	})

	// Styled pre-footer should be truncated, not wrapped
	styledPreFooter := internal.RedFg.Render("Styled") + " " + internal.BlueFg.Render("pre-footer line")
	vp.SetPreFooterLine(styledPreFooter)

	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"short",
		"longer content ",
		"here",
		"",
		internal.RedFg.Render("Styled") + " " + internal.BlueFg.Render("pre-f..."),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineExactWidth(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Pre-footer exactly matches width - should not truncate
	vp.SetPreFooterLine("1234567890")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"1234567890",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineOneCharOverWidth(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Pre-footer one char over width - should truncate
	vp.SetPreFooterLine("12345678901")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"1234567...",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineUnicode(t *testing.T) {
	w, h := 20, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Pre-footer with unicode (emojis are 2 cells wide)
	vp.SetPreFooterLine("Status: ✓ Done")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"Status: ✓ Done",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPreFooterLineUnicodeTruncation(t *testing.T) {
	w, h := 12, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Pre-footer with unicode that needs truncation
	// Each 💖 is 2 cells wide, so with width 12 we can fit 4 emojis (8 cells) + ".." (2 cells) = 10
	// or 5 emojis (10 cells) + ".." (2 cells) = 12 exactly
	vp.SetPreFooterLine("💖💖💖💖💖💖💖💖")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"💖💖💖💖💖..",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}
