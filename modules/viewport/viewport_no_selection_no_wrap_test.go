package viewport

import (
	"strconv"
	"testing"

	"github.com/antgroup/hugescm/modules/viewport/internal"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

func TestViewport_SelectionOff_WrapOff_Empty(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{})
	internal.CmpStr(t, expectedView, vp.View())
	vp.SetHeader([]string{"header"})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"header"})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_SmolDimensions(t *testing.T) {
	w, h := 0, 0
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{"hi"})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{""})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetWidth(1)
	vp.SetHeight(1)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"."})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetWidth(2)
	vp.SetHeight(2)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"..", ""})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetWidth(3)
	vp.SetHeight(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"...", "hi", "..."})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_Basic(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really long line"),
		internal.RedFg.Render("a") + " really really long line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really rea..."),
		internal.RedFg.Render("a") + " really rea...",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_GetConfigs(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"first",
		"second",
	})
	if selectionEnabled := vp.GetSelectionEnabled(); selectionEnabled {
		t.Errorf("expected selection to be disabled, got %v", selectionEnabled)
	}
	if wrapText := vp.GetWrapText(); wrapText {
		t.Errorf("expected text wrapping to be disabled, got %v", wrapText)
	}
	if selectedItemIdx := vp.GetSelectedItemIdx(); selectedItemIdx != 0 {
		t.Errorf("expected selected item index to be 0, got %v", selectedItemIdx)
	}
	if selectedItem := vp.GetSelectedItem(); selectedItem != nil {
		t.Errorf("expected selected item to be nil, got %v", selectedItem)
	}
}

func TestViewport_SelectionOff_WrapOff_ShowFooter(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really long line"),
		internal.RedFg.Render("a") + " really really long line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really rea..."),
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(6)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really rea..."),
		internal.RedFg.Render("a") + " really rea...",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(7)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really rea..."),
		internal.RedFg.Render("a") + " really rea...",
		"",
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_FooterStyle(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h, WithStyles[object](Styles{
		FooterStyle:       internal.RedFg,
		SelectedItemStyle: selectionStyle,
	}))
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"1",
		"2",
		"3",
		"4",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"1",
		"2",
		"3",
		internal.RedFg.Render("75% (3/4)"),
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_FooterDisabled(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"first line",
		"second line",
		"third line",
		"fourth line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		"second line",
		"third line",
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetFooterEnabled(false)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first line",
		"second line",
		"third line",
		"fourth line",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_SpaceAround(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"    first line     ",
		"          first line          ",
		"               first line               ",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"    first li...",
		"          fi...",
		"            ...",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_MultiHeader(t *testing.T) {
	w, h := 15, 2
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header1", "header2"})
	setContent(vp, []string{
		"line1",
		"line2",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(4)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line1",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line2",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(5)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line1",
		"line2",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(6)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line1",
		"line2",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_OverflowLine(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"long header overflows"})
	setContent(vp, []string{
		"123456789012345",
		"1234567890123456",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"long header ...",
		"123456789012345",
		"123456789012...",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_OverflowHeight(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"123456789012345",
		"1234567890123456",
		"1234567890123456",
		"1234567890123456",
		"1234567890123456",
		"1234567890123456",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"123456789012345",
		"123456789012...",
		"123456789012...",
		"123456789012...",
		"66% (4/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_Scrolling(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	doSetContent := func() {
		setContent(vp, []string{
			"first",
			"second",
			"third",
			"fourth",
			"fifth",
			"sixth",
		})
	}
	validate := func(expectedView string) {
		// set Item multiple times to confirm no side effects of doing it
		internal.CmpStr(t, expectedView, vp.View())
		doSetContent()
		internal.CmpStr(t, expectedView, vp.View())
	}
	doSetContent()
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"third",
		"fourth",
		"66% (4/6)",
	})
	validate(expectedView)

	// scrolling up past top is no-op
	vp, _ = vp.Update(upKeyMsg)
	validate(expectedView)

	// scrolling down by one
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"third",
		"fourth",
		"fifth",
		"83% (5/6)",
	})
	validate(expectedView)

	// scrolling down by one again
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		"fourth",
		"fifth",
		"sixth",
		"100% (6/6)",
	})
	validate(expectedView)

	// scrolling down past bottom when at bottom is no-op
	vp, _ = vp.Update(downKeyMsg)
	validate(expectedView)
}

func TestViewport_SelectionOff_WrapOff_EnsureItemInView(t *testing.T) {
	w, h := 10, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth line that is really long",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"33% (2/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(5, 0, 0, 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fifth",
		"sixth l...",
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(5, len("sixth line"), len("sixth line "), 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"...h",
		"...h li...", // 's|ixth line '
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(5, len("sixth line that is really lon"), len("sixth line that is really long"), 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"...",
		"...ly long",
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(1, 0, 0, 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"third",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.EnsureItemInView(4, 0, 0, 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fourth",
		"fifth",
		"83% (5/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// ensure idempotence
	vp.EnsureItemInView(4, 0, 0, 0, 0)
	internal.CmpStr(t, expectedView, vp.View())

	// invalid values truncated
	vp.EnsureItemInView(4, -1, 1e9, 0, 0)
	internal.CmpStr(t, expectedView, vp.View())

	// full width ok
	vp.EnsureItemInView(4, 0, len("fifth"), 0, 0)
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_EnsureItemInViewVerticalPad(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	numItems := 100
	nums := make([]string, 0, numItems)
	for i := range numItems {
		nums = append(nums, strconv.Itoa(i+1))
	}
	setContent(vp, nums)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"1",
		"2",
		"3",
		"4",
		"4% (4/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "5" with verticalPad=1
	// should leave 1 line of context below
	vp.EnsureItemInView(4, 0, 0, 1, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"3",
		"4",
		"5",
		"6",
		"6% (6/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll up to "3" with verticalPad=1
	// should leave 1 line of context above
	vp.EnsureItemInView(2, 0, 0, 1, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"2",
		"3",
		"4",
		"5",
		"5% (5/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll up to visible "8" with verticalPad=2
	// should leave 2 lines of context above
	vp.EnsureItemInView(9, 0, 0, 0, 0) // reset to bottom
	vp.EnsureItemInView(7, 0, 0, 2, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"6",
		"7",
		"8",
		"9",
		"9% (9/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "99", not enough content below for verticalPad=3
	// pad below as much as possible
	vp.EnsureItemInView(0, 0, 0, 0, 0) // reset to top
	vp.EnsureItemInView(98, 0, 0, 3, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"97",
		"98",
		"99",
		"100",
		"100% (1...",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "50", request more padding than is available given viewport height -> center item
	vp.EnsureItemInView(0, 0, 0, 0, 0) // reset to top
	vp.EnsureItemInView(49, 0, 0, 3, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"49",
		"50",
		"51",
		"52",
		"52% (52...",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_EnsureItemInViewHorizontalPad(t *testing.T) {
	w, h := 10, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"some line that is really long",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"some li...",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: pan right to space after "line" with horizontalPad=2
	// should leave 2 columns of padding to the right
	vp.EnsureItemInView(0, 0, 0, 0, 0) // reset to top
	vp.EnsureItemInView(0, len("some line"), len("some line "), 0, 2)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"...line...", // 'so|me line_th|at is really long'
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: pan to the visible "me" of "some" with horizontalPad=1
	// should leave 1 column of context to the left
	vp.EnsureItemInView(0, len("so"), len("some"), 0, 1)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"... lin...", // 's|o__ line t|hat is really long'
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: pan right to the " r" of "is really" with huge horizontalPad
	// should center the target portion horizontally
	vp.EnsureItemInView(0, len("some line that is"), len("some line that is r"), 0, 100)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"...s re...", // 'some line tha|t is__eall|y long'
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_SetXOffset(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"the first line",
		"the second line",
	})
	initialExpectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the fir...",
		"the sec...",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, initialExpectedView, vp.View())

	vp.SetXOffset(-1)
	internal.CmpStr(t, initialExpectedView, vp.View())

	vp.SetXOffset(0)
	internal.CmpStr(t, initialExpectedView, vp.View())

	vp.SetXOffset(4)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"...st line",
		"...ond ...",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetXOffset(1000)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"...t line ",
		"...nd line",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_BulkScrolling(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"33% (2/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page down
	vp, _ = vp.Update(fullPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		"fourth",
		"66% (4/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page down
	vp, _ = vp.Update(halfPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fourth",
		"fifth",
		"83% (5/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page down
	vp, _ = vp.Update(fullPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fifth",
		"sixth",
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page up
	vp, _ = vp.Update(fullPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		"fourth",
		"66% (4/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page up
	vp, _ = vp.Update(halfPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"third",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page up
	vp, _ = vp.Update(fullPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"33% (2/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// go to bottom
	vp, _ = vp.Update(goToBottomKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fifth",
		"sixth",
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// go to top
	vp, _ = vp.Update(goToTopKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"33% (2/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_Panning(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header long"})
	doSetContent := func() {
		setContent(vp, []string{
			"first line that is fairly long",
			"second line that is even much longer than the first",
			"third line that is fairly long",
			"fourth",
			"fifth line that is fairly long",
			"sixth",
		})
	}
	validate := func(expectedView string) {
		// set Item multiple times to confirm no side effects of doing it
		internal.CmpStr(t, expectedView, vp.View())
		doSetContent()
		internal.CmpStr(t, expectedView, vp.View())
	}
	doSetContent()
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"first l...",
		"second ...",
		"third l...",
		"fourth",
		"66% (4/6)",
	})
	validate(expectedView)

	// pan right
	vp.SetXOffset(5)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...ne t...",
		"...ine ...",
		"...ne t...",
		".",
		"66% (4/6)",
	})
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...ine ...",
		"...ne t...",
		".",
		"...ne t...",
		"83% (5/6)",
	})
	validate(expectedView)

	// pan all the way right
	vp.SetXOffset(41)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...e first",
		"...",
		"...",
		"...",
		"83% (5/6)",
	})
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...ly long",
		"...",
		"...ly long",
		"...",
		"100% (6/6)",
	})
	validate(expectedView)

	// set shorter Item
	setContent(vp, []string{
		"the first one",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...rst one",
		"",
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_ChangeHeight(t *testing.T) {
	w, h := 10, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// increase height
	vp.SetHeight(6)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"third",
		"fourth",
		"66% (4/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll to bottom
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		"fourth",
		"fifth",
		"sixth",
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// reduce height
	vp.SetHeight(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// increase height
	vp.SetHeight(8)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_ChangeContent(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"third",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fourth",
		"fifth",
		"sixth",
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// remove Item
	setContent(vp, []string{})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// re-add Item
	setContent(vp, []string{
		"first",
		"second",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_SetSelectionEnabled_SetsTopVisibleItem(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
	})
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp.SetSelectionEnabled(true)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		internal.BlueFg.Render("third"),
		"fourth",
		"fifth",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_SetHighlights(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line",
	})
	highlights := []Highlight{
		{
			ItemIndex: 1,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 4,
					End:   10,
				},
				Style: internal.RedFg,
			},
		},
		{
			ItemIndex: 2,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 4,
					End:   9,
				},
				Style: internal.GreenFg,
			},
		},
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"the first line",
		"the " + internal.RedFg.Render("second") + " line",
		"the " + internal.GreenFg.Render("third") + " line",
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_SetHighlightsStyledContent(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	setContent(vp, []string{
		internal.RedFg.Render("the first line"),
		internal.GreenFg.Render("the second line"),
		internal.BlueFg.Render("the third line"),
		internal.RedFg.Render("the fourth line"),
	})
	highlights := []Highlight{
		{
			ItemIndex: 1,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 4,
					End:   10,
				},
				Style: internal.BlueFg,
			},
		},
		{
			ItemIndex: 2,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 4,
					End:   9,
				},
				Style: internal.RedFg,
			},
		},
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.RedFg.Render("the first line"),
		internal.GreenFg.Render("the ") + internal.BlueFg.Render("second") + internal.GreenFg.Render(" line"),
		internal.BlueFg.Render("the ") + internal.RedFg.Render("third") + internal.BlueFg.Render(" line"),
		"75% (3/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_SetHighlightsAnsiUnicode(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	// A (1w, 1b), 💖 (2w, 4b), 中 (2w, 3b), é (1w, 3b) = 6w, 11b
	vp.SetHeader([]string{"A💖中é"})
	setContent(vp, []string{
		"A💖中é line",
		"another line",
	})
	highlights := []Highlight{
		{
			ItemIndex: 0,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 1,
					End:   8,
				},
				Style: internal.RedFg,
			},
		},
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"A💖中é",
		"A" + internal.RedFg.Render("💖中") + "é line",
		"another line",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}
