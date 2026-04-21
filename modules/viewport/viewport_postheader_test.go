package viewport

import (
	"testing"

	"github.com/antgroup/hugescm/modules/viewport/internal"
)

func TestPostHeaderLineWithFooterEnabled(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
	})

	// Without post-header: 3 content lines + footer
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// With post-header: post-header + 2 content lines + footer (height 5 - 1 post-header - 1 footer = 3 content)
	vp.SetPostHeaderLine("Post-header text")
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Post-header text",
		"line 1",
		"line 2",
		"line 3",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineWithFooterDisabled(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
	})
	vp.SetFooterEnabled(false)

	// Without post-header
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"",
		"",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// With post-header (still renders even though footer disabled)
	vp.SetPostHeaderLine("Post-header text")
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Post-header text",
		"line 1",
		"line 2",
		"line 3",
		"",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestEmptyPostHeaderLine(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
	})

	// Empty post-header means no extra line rendered
	vp.SetPostHeaderLine("")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineTruncation(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Set post-header longer than viewport width
	vp.SetPostHeaderLine("This is a very long post-header line that exceeds the width")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"This is a ve...",
		"line 1",
		"line 2",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineSmallHeight(t *testing.T) {
	w, h := 20, 3
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
	})

	// Height 3 with footer and post-header: 1 post-header + 1 content + 1 footer
	vp.SetPostHeaderLine("Post-header")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Post-header",
		"line 1",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineWithHeader(t *testing.T) {
	w, h := 30, 7
	vp := newViewport(w, h)
	vp.SetHeader([]string{"Header"})
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
	})

	// Height 7 with header, post-header, footer: 1 header + 1 post-header + 3 content + 1 padding + 1 footer
	vp.SetPostHeaderLine("Post-header")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Header",
		"Post-header",
		"line 1",
		"line 2",
		"line 3",
		"",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineDynamicToggle(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
	})

	// Initially no post-header
	expectedNoPostHeader := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedNoPostHeader, vp.View())

	// Set post-header
	vp.SetPostHeaderLine("Post-header")
	expectedWithPostHeader := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Post-header",
		"line 1",
		"line 2",
		"line 3",
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedWithPostHeader, vp.View())

	// Remove post-header
	vp.SetPostHeaderLine("")
	internal.CmpStr(t, expectedNoPostHeader, vp.View())

	// Set post-header again with different text
	vp.SetPostHeaderLine("Different post-header")
	expectedDifferent := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Different post-header",
		"line 1",
		"line 2",
		"line 3",
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedDifferent, vp.View())
}

func TestPostHeaderLineReducesContentLines(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
	})

	// Without post-header: 4 content lines visible (height 5 - 1 footer = 4)
	expectedNoPostHeader := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"80% (4/5)",
	})
	internal.CmpStr(t, expectedNoPostHeader, vp.View())

	// With post-header: 3 content lines visible (height 5 - 1 post-header - 1 footer = 3)
	vp.SetPostHeaderLine("Post-header")
	expectedWithPostHeader := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Post-header",
		"line 1",
		"line 2",
		"line 3",
		"60% (3/5)",
	})
	internal.CmpStr(t, expectedWithPostHeader, vp.View())
}

func TestPostHeaderLineWithWrap(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h, WithWrapText[object](true))
	setContent(vp, []string{
		"short",
		"longer text that wraps",
	})

	// Post-header should appear before content
	vp.SetPostHeaderLine("Post-head")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Post-head",
		"short",
		"longer tex",
		"t that wra",
		"ps",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineScrolling(t *testing.T) {
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
	vp.SetPostHeaderLine("Post-header")

	// Initially shows first 3 content lines (height 5 - 1 post-header - 1 footer = 3)
	expectedInitial := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Post-header",
		"line 1",
		"line 2",
		"line 3",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedInitial, vp.View())

	// Scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedAfterScroll := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Post-header",
		"line 2",
		"line 3",
		"line 4",
		"66% (4/6)",
	})
	internal.CmpStr(t, expectedAfterScroll, vp.View())
}

func TestPostHeaderLineStyled(t *testing.T) {
	w, h := 30, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Set a styled post-header line
	styledPostHeader := internal.RedFg.Render("Red") + " and " + internal.BlueFg.Render("Blue")
	vp.SetPostHeaderLine(styledPostHeader)

	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		internal.RedFg.Render("Red") + " and " + internal.BlueFg.Render("Blue"),
		"line 1",
		"line 2",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineDoesNotWrap(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h, WithWrapText[object](true))
	setContent(vp, []string{
		"short",
		"another",
	})

	// Set a long post-header line - it should NOT wrap, only truncate
	vp.SetPostHeaderLine("This is a very long post-header that should not wrap")

	// Post-header should be truncated to single line, not wrapped
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"This is...",
		"short",
		"another",
		"",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineWithPreFooterLine(t *testing.T) {
	w, h := 30, 6
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
	})

	// Both post-header and pre-footer: height 6 - 1 post-header - 1 pre-footer - 1 footer = 3 content
	vp.SetPostHeaderLine("Post-header")
	vp.SetPreFooterLine("Pre-footer")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Post-header",
		"line 1",
		"line 2",
		"line 3",
		"Pre-footer",
		"60% (3/5)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineWithHeaderAndPreFooter(t *testing.T) {
	w, h := 30, 7
	vp := newViewport(w, h)
	vp.SetHeader([]string{"Header"})
	setContent(vp, []string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
	})

	// All extras: 1 header + 1 post-header + 2 content + 1 pre-footer + 1 footer = 7
	// (height 7 - 1 header - 1 post-header - 1 pre-footer - 1 footer = 3 content)
	vp.SetPostHeaderLine("Post-header")
	vp.SetPreFooterLine("Pre-footer")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"Header",
		"Post-header",
		"line 1",
		"line 2",
		"line 3",
		"Pre-footer",
		"60% (3/5)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineExactWidth(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Post-header exactly matches width - should not truncate
	vp.SetPostHeaderLine("1234567890")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"1234567890",
		"line 1",
		"line 2",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestPostHeaderLineOneCharOverWidth(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"line 1",
		"line 2",
	})

	// Post-header one char over width - should truncate
	vp.SetPostHeaderLine("12345678901")
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"1234567...",
		"line 1",
		"line 2",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}
