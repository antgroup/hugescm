package viewport

import (
	"strconv"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/antgroup/hugescm/modules/viewport/internal"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

func TestViewport_SelectionOn_WrapOff_Empty(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetSelectionEnabled(true)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{})
	internal.CmpStr(t, expectedView, vp.View())
	vp.SetHeader([]string{"header"})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{"header"})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_SmolDimensions(t *testing.T) {
	w, h := 0, 0
	vp := newViewport(w, h)
	vp.SetSelectionEnabled(true)
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
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"...",
		internal.BlueFg.Render("hi"),
		"...",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_Basic(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really long line"),
		internal.RedFg.Render("a") + " really really long line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really rea..."),
		internal.RedFg.Render("a") + " really rea...",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_GetConfigs(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"first",
		"second",
	})
	if selectionEnabled := vp.GetSelectionEnabled(); !selectionEnabled {
		t.Errorf("expected selection to be enabled, got %v", selectionEnabled)
	}
	if wrapText := vp.GetWrapText(); wrapText {
		t.Errorf("expected text wrapping to be disabled, got %v", wrapText)
	}
	if selectedItemIdx := vp.GetSelectedItemIdx(); selectedItemIdx != 0 {
		t.Errorf("expected selected item index to be 0, got %v", selectedItemIdx)
	}
	vp, _ = vp.Update(downKeyMsg)
	if selectedItemIdx := vp.GetSelectedItemIdx(); selectedItemIdx != 1 {
		t.Errorf("expected selected item index to be 1, got %v", selectedItemIdx)
	}
	if selectedItem := vp.GetSelectedItem(); selectedItem != nil && selectedItem.GetItem().Content() != "second" {
		t.Errorf("got unexpected selected item: %v", selectedItem)
	}
}

func TestViewport_SelectionOn_WrapOff_ShowFooter(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"first line",
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really really long line"),
		internal.RedFg.Render("a") + " really really long line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really rea..."),
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(6)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really rea..."),
		internal.RedFg.Render("a") + " really rea...",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(7)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		internal.RedFg.Render("second") + " line",
		internal.RedFg.Render("a really rea..."),
		internal.RedFg.Render("a") + " really rea...",
		"",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_FooterStyle(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h, WithStyles[object](Styles{
		FooterStyle:       internal.RedFg,
		SelectedItemStyle: selectionStyle,
	}))
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"1",
		"2",
		"3",
		"4",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("1"),
		"2",
		"3",
		internal.RedFg.Render("25% (1/4)"),
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_FooterDisabled(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"first line",
		"second line",
		"third line",
		"fourth line",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		"second line",
		"third line",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetFooterEnabled(false)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first line"),
		"second line",
		"third line",
		"fourth line",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_SpaceAround(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"    first line     ",
		"          first line          ",
		"               first line               ",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("    first li..."),
		"          fi...",
		"            ...",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_MultiHeader(t *testing.T) {
	w, h := 15, 2
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header1", "header2"})
	vp.SetSelectionEnabled(true)
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
		internal.BlueFg.Render("line1"),
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		internal.BlueFg.Render("line2"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(5)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line1",
		internal.BlueFg.Render("line2"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(6)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header1",
		"header2",
		"line1",
		internal.BlueFg.Render("line2"),
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_OverflowLine(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"long header overflows"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"123456789012345",
		"1234567890123456",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"long header ...",
		internal.BlueFg.Render("123456789012345"),
		"123456789012...",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_OverflowHeight(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
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
		internal.BlueFg.Render("123456789012345"),
		"123456789012...",
		"123456789012...",
		"123456789012...",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_Scrolling(t *testing.T) {
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
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
		internal.BlueFg.Render("first"),
		"second",
		"third",
		"fourth",
		"16% (1/6)",
	})
	validate(expectedView)

	// scrolling up past top is no-op
	vp, _ = vp.Update(upKeyMsg)
	validate(expectedView)

	// scrolling down by one
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		internal.BlueFg.Render("second"),
		"third",
		"fourth",
		"33% (2/6)",
	})
	validate(expectedView)

	// scrolling to bottom
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		"fourth",
		"fifth",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	validate(expectedView)

	// scrolling down past bottom when at bottom is no-op
	vp, _ = vp.Update(downKeyMsg)
	validate(expectedView)
}

func TestViewport_SelectionOn_WrapOff_EnsureItemInView(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
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
		internal.BlueFg.Render("first"),
		"second",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll so last item in view
	vp.EnsureItemInView(5, 0, 0, 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fifth",
		"sixth",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll so second item in view
	vp.EnsureItemInView(1, 0, 0, 0, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"third",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("second"),
		"third",
		"33% (2/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// ensure idempotence
	vp.EnsureItemInView(1, 0, 0, 0, 0)
	internal.CmpStr(t, expectedView, vp.View())

	// invalid values truncated
	vp.EnsureItemInView(1, -1, 1e9, 0, 0)
	internal.CmpStr(t, expectedView, vp.View())

	// full width ok
	vp.EnsureItemInView(1, 0, len("second"), 0, 0)
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_EnsureItemInViewVerticalPad(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	numItems := 100
	nums := make([]string, 0, numItems)
	for i := range numItems {
		nums = append(nums, strconv.Itoa(i+1))
	}
	setContent(vp, nums)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("1"),
		"2",
		"3",
		"4",
		"1% (1/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "5" with verticalPad=1
	// should leave 1 line of context below
	vp.SetSelectedItemIdx(4)
	vp.EnsureItemInView(4, 0, 0, 1, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"3",
		"4",
		selectionStyle.Render("5"),
		"6",
		"5% (5/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll up to "3" with verticalPad=1
	// should leave 1 line of context above
	vp.SetSelectedItemIdx(2)
	vp.EnsureItemInView(2, 0, 0, 1, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"2",
		selectionStyle.Render("3"),
		"4",
		"5",
		"3% (3/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "8" with verticalPad=2
	// should leave 2 lines of context above
	vp.SetSelectedItemIdx(99) // reset to bottom
	vp.EnsureItemInView(99, 0, 0, 0, 0)
	vp.SetSelectedItemIdx(7)
	vp.EnsureItemInView(7, 0, 0, 2, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"6",
		"7",
		selectionStyle.Render("8"),
		"9",
		"8% (8/100)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "99", not enough content below for verticalPad=3
	// pad below as much as possible
	vp.SetSelectedItemIdx(0) // reset to top
	vp.EnsureItemInView(0, 0, 0, 0, 0)
	vp.SetSelectedItemIdx(98)
	vp.EnsureItemInView(98, 0, 0, 3, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"97",
		"98",
		selectionStyle.Render("99"),
		"100",
		"99% (99...",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to "50", request more padding than is available given viewport height -> center item
	vp.SetSelectedItemIdx(0) // reset to top
	vp.EnsureItemInView(0, 0, 0, 0, 0)
	vp.SetSelectedItemIdx(49)
	vp.EnsureItemInView(49, 0, 0, 3, 0)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"49",
		selectionStyle.Render("50"),
		"51",
		"52",
		"50% (50...",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_EnsureItemInViewHorizontalPad(t *testing.T) {
	w, h := 10, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"some line that is really long",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("some li..."),
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: pan right to space after "line" with horizontalPad=2
	// should leave 2 columns of padding to the right
	vp.SetSelectedItemIdx(0) // reset to top
	vp.EnsureItemInView(0, 0, 0, 0, 0)
	vp.EnsureItemInView(0, len("some line"), len("some line "), 0, 2)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("...line..."), // 'so|me line_th|at is really long'
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: pan to the visible "me" of "some" with horizontalPad=1
	// should leave 1 column of context to the left
	vp.EnsureItemInView(0, len("so"), len("some"), 0, 1)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("... lin..."), // 's|o__ line t|hat is really long'
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// horizontalPad: pan right to the " r" of "is really" with huge horizontalPad
	// should center the target portion horizontally
	vp.EnsureItemInView(0, len("some line that is"), len("some line that is r"), 0, 100)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("...s re..."), // 'some line tha|t is__eall|y long'
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_SetXOffset(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
	})
	initialExpectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("the fir..."),
		"the sec...",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, initialExpectedView, vp.View())

	vp.SetXOffset(-1)
	internal.CmpStr(t, initialExpectedView, vp.View())

	vp.SetXOffset(0)
	internal.CmpStr(t, initialExpectedView, vp.View())

	vp.SetXOffset(4)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("...st line"),
		"...ond ...",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetXOffset(1000)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("...t line"),
		"...nd line",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_BulkScrolling(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
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
		internal.BlueFg.Render("first"),
		"second",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page down
	vp, _ = vp.Update(fullPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("third"),
		"fourth",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page down
	vp, _ = vp.Update(halfPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("fourth"),
		"fifth",
		"66% (4/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page down
	vp, _ = vp.Update(fullPgDownKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fifth",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page up
	vp, _ = vp.Update(fullPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		internal.BlueFg.Render("fourth"),
		"66% (4/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page up
	vp, _ = vp.Update(halfPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		internal.BlueFg.Render("third"),
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// half page up
	vp, _ = vp.Update(halfPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		internal.BlueFg.Render("second"),
		"33% (2/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// full page up
	vp, _ = vp.Update(fullPgUpKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"second",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// go to bottom
	vp, _ = vp.Update(goToBottomKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fifth",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// go to top
	vp, _ = vp.Update(goToTopKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"second",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_Panning(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header long"})
	vp.SetSelectionEnabled(true)
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
		internal.BlueFg.Render("first l..."),
		"second ...",
		"third l...",
		"fourth",
		"16% (1/6)",
	})
	validate(expectedView)

	// pan right
	vp.SetXOffset(5)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		internal.BlueFg.Render("...ne t..."),
		"...ine ...",
		"...ne t...",
		".",
		"16% (1/6)",
	})
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...ne t...",
		internal.BlueFg.Render("...ine ..."),
		"...ne t...",
		".",
		"33% (2/6)",
	})
	validate(expectedView)

	// pan all the way right
	vp.SetXOffset(41)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...",
		internal.BlueFg.Render("...e first"),
		"...",
		"...",
		"33% (2/6)",
	})
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...",
		"...e first",
		internal.BlueFg.Render("..."),
		"...",
		"50% (3/6)",
	})
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...",
		"...e first",
		"...",
		internal.BlueFg.Render("..."),
		"66% (4/6)",
	})
	validate(expectedView)

	// scroll down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...e first",
		"...",
		"...",
		internal.BlueFg.Render("..."),
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
		internal.BlueFg.Render("..."),
		"100% (6/6)",
	})
	validate(expectedView)

	// scroll up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...ly long",
		"...",
		internal.BlueFg.Render("...ly long"),
		"...",
		"83% (5/6)",
	})
	validate(expectedView)

	// scroll up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		"...ly long",
		internal.BlueFg.Render("..."),
		"...ly long",
		"...",
		"66% (4/6)",
	})
	validate(expectedView)

	// scroll up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		internal.BlueFg.Render("...ly long"),
		"...",
		"...ly long",
		"...",
		"50% (3/6)",
	})
	validate(expectedView)

	// scroll up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		internal.BlueFg.Render("...n mu..."),
		"...ly long",
		"...",
		"...ly long",
		"33% (2/6)",
	})
	validate(expectedView)

	// scroll up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		internal.BlueFg.Render("...ly long"),
		"...n mu...",
		"...ly long",
		"...",
		"16% (1/6)",
	})
	validate(expectedView)

	// set shorter Item
	setContent(vp, []string{
		"the first one",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header ...",
		internal.BlueFg.Render("...rst one"),
		"",
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_MaintainSelection(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	vp.SetSelectionComparator(objectsEqual)
	setContent(vp, []string{
		"sixth",
		"seventh",
		"eighth",
		"ninth",
		"tenth",
		"eleventh",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("sixth"),
		"seventh",
		"eighth",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// selection down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"sixth",
		internal.BlueFg.Render("seventh"),
		"eighth",
		"33% (2/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item above
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
		"seventh",
		"eighth",
		"ninth",
		"tenth",
		"eleventh",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"sixth",
		internal.BlueFg.Render("seventh"),
		"eighth",
		"63% (7/11)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item below
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
		"seventh",
		"eighth",
		"ninth",
		"tenth",
		"eleventh",
		"twelfth",
		"thirteenth",
		"fourteenth",
		"fifteenth",
		"sixteenth",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"sixth",
		internal.BlueFg.Render("seventh"),
		"eighth",
		"43% (7/16)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_StickyTop(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	// stickyness should override maintain selection
	vp.SetSelectionComparator(objectsEqual)
	vp.SetTopSticky(true)
	setContent(vp, []string{
		"first",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"second",
		"first",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("second"),
		"first",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// de-activate by moving selection down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		internal.BlueFg.Render("first"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"second",
		"first",
		"third",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		internal.BlueFg.Render("first"),
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_StickyBottom(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	// stickyness should override maintain selection
	vp.SetSelectionComparator(objectsEqual)
	vp.SetBottomSticky(true)
	setContent(vp, []string{
		"first",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"second",
		"first",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		internal.BlueFg.Render("first"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// de-activate by moving selection up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("second"),
		"first",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"second",
		"first",
		"third",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("second"),
		"first",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_StickyBottomOverflowHeight(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	// stickyness should override maintain selection
	vp.SetSelectionComparator(objectsEqual)
	vp.SetBottomSticky(true)

	// test covers case where first set Item to empty, then overflow height
	setContent(vp, []string{})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
	})
	internal.CmpStr(t, expectedView, vp.View())

	setContent(vp, []string{
		"second",
		"first",
		"third",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		internal.BlueFg.Render("third"),
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_StickyTopBottom(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	// stickyness should override maintain selection
	vp.SetSelectionComparator(objectsEqual)
	vp.SetTopSticky(true)
	vp.SetBottomSticky(true)
	setContent(vp, []string{
		"first",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item, top sticky wins out arbitrarily when both set
	setContent(vp, []string{
		"second",
		"first",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("second"),
		"first",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// selection to bottom
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		internal.BlueFg.Render("first"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"second",
		"first",
		"third",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		internal.BlueFg.Render("third"),
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// de-activate by moving selection up
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"third",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item
	setContent(vp, []string{
		"second",
		"first",
		"third",
		"fourth",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"third",
		"50% (2/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_StickyTop(t *testing.T) {
	w, h := 15, 4
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(false)
	vp.SetTopSticky(true)
	setContent(vp, []string{
		"first",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item
	setContent(vp, []string{
		"second",
		"first",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"first",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll down to de-activate sticky
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"first",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item - should not return to top
	setContent(vp, []string{
		"third",
		"second",
		"first",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		"second",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_StickyBottom(t *testing.T) {
	w, h := 15, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(false)
	vp.SetBottomSticky(true)
	setContent(vp, []string{
		"first",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item at bottom
	setContent(vp, []string{
		"first",
		"second",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll up to de-activate sticky
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item - should not jump to bottom
	setContent(vp, []string{
		"first",
		"second",
		"third",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_StickyBottomOverflowHeight(t *testing.T) {
	w, h := 15, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(false)
	vp.SetBottomSticky(true)
	setContent(vp, []string{})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add more items than fit in viewport
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fifth",
		"100% (5/5)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOff_WrapOff_StickyTopBottom(t *testing.T) {
	w, h := 15, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(false)
	vp.SetTopSticky(true)
	vp.SetBottomSticky(true)
	setContent(vp, []string{
		"first",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item, top sticky wins when both set
	setContent(vp, []string{
		"first",
		"second",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll to bottom
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item - bottom sticky should activate
	setContent(vp, []string{
		"first",
		"second",
		"third",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"third",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// scroll up to middle
	vp, _ = vp.Update(upKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add item - neither sticky should activate (not at top or bottom)
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"second",
		"50% (2/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_RemoveLogsWhenSelectionBottom(t *testing.T) {
	w, h := 10, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)

	// add Item
	setContent(vp, []string{
		"second",
		"first",
		"third",
		"fourth",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("second"),
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// selection to bottom
	vp.SetSelectedItemIdx(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("fourth"),
		"100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// remove Item
	setContent(vp, []string{
		"second",
		"first",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_ChangeHeight(t *testing.T) {
	w, h := 10, 3
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
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
		internal.BlueFg.Render("first"),
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// increase height
	vp.SetHeight(8)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection to third line
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		internal.BlueFg.Render("third"),
		"fourth",
		"fifth",
		"sixth",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// reduce height
	vp.SetHeight(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("third"),
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// increase height
	vp.SetHeight(8)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		internal.BlueFg.Render("third"),
		"fourth",
		"fifth",
		"sixth",
		"50% (3/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection to last line
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// reduce height
	vp.SetHeight(3)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
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
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_ChangeContent(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
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
		internal.BlueFg.Render("first"),
		"second",
		"third",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection to bottom
	vp.SetSelectedItemIdx(5)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fourth",
		"fifth",
		internal.BlueFg.Render("sixth"),
		"100% (6/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// remove Item
	setContent(vp, []string{
		"second",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("second"),
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// remove all Item
	setContent(vp, []string{})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// add Item (maintain selection off)
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
	})
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("first"),
		"second",
		"third",
		"16% (1/6)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_AnsiOnSelection(t *testing.T) {
	w, h := 20, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"line with " + internal.RedFg.Render("red") + " text",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("line with red text"), // selection style overrides text styling
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_AnsiOnSelection_NoOverride(t *testing.T) {
	w, h := 20, 5
	vp := newViewport(w, h, WithSelectionStyleOverridesItemStyle[object](false))
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"line with " + internal.RedFg.Render("red") + " text",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("line with ") + internal.RedFg.Render("red") + selectionStyle.Render(" text"), // item style preserved
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_SelectionEmpty(t *testing.T) {
	w, h := 20, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render(" "),
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_ExtraSlash(t *testing.T) {
	w, h := 25, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"|2024|" + internal.RedFg.Render("fl..lq") + "/" + internal.RedFg.Render("flask-3") + "|",
	})
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("|2024|fl..lq/flask-3|"), // selection style overrides text styling
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_SetHighlights(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"the first line",
		"the second line",
		"the third line",
		"the fourth line",
	})
	highlights := []Highlight{
		{
			ItemIndex: 0,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 4,
					End:   9,
				},
				Style: internal.GreenFg,
			},
		},
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
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.BlueFg.Render("the ") + internal.GreenFg.Render("first") + internal.BlueFg.Render(" line"),
		"the " + internal.RedFg.Render("second") + " line",
		"the third line",
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_SetHighlightsStyledContent(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		internal.RedFg.Render("the first line"),
		internal.GreenFg.Render("the second line"),
		internal.BlueFg.Render("the third line"),
		internal.RedFg.Render("the fourth line"),
	})
	highlights := []Highlight{
		{
			ItemIndex: 0,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 4,
					End:   9,
				},
				Style: internal.GreenFg,
			},
		},
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
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		selectionStyle.Render("the ") + internal.GreenFg.Render("first") + selectionStyle.Render(" line"),
		internal.GreenFg.Render("the ") + internal.RedFg.Render("second") + internal.GreenFg.Render(" line"),
		internal.BlueFg.Render("the third line"),
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_SetHighlightsStyledContent_NoOverride(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h, WithSelectionStyleOverridesItemStyle[object](false))
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		internal.RedFg.Render("the first line"),
		internal.GreenFg.Render("the second line"),
		internal.BlueFg.Render("the third line"),
		internal.RedFg.Render("the fourth line"),
	})
	highlights := []Highlight{
		{
			ItemIndex: 0,
			ItemHighlight: item.Highlight{
				ByteRangeUnstyledContent: item.ByteRange{
					Start: 4,
					End:   9,
				},
				Style: internal.GreenFg,
			},
		},
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
	}
	vp.SetHighlights(highlights)
	expectedView := internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		internal.RedFg.Render("the ") + internal.GreenFg.Render("first") + internal.RedFg.Render(" line"), // item style preserved, highlight applied
		internal.GreenFg.Render("the ") + internal.RedFg.Render("second") + internal.GreenFg.Render(" line"),
		internal.BlueFg.Render("the third line"),
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_SetHighlightsAnsiUnicode(t *testing.T) {
	w, h := 15, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"A💖中é"})
	vp.SetSelectionEnabled(true)
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
		internal.BlueFg.Render("A") + internal.RedFg.Render("💖中") + internal.BlueFg.Render("é line"),
		"another line",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionPrefix_Basic(t *testing.T) {
	w, h := 20, 6
	prefix := "> "

	vp := newViewport(w, h, WithStyles[object](Styles{
		SelectionPrefix:   prefix,
		FooterStyle:       lipgloss.NewStyle(),
		SelectedItemStyle: selectionStyle,
	}))
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{"first", "second", "third"})

	// selection on first item: prefix on first, padding on others
	expectedView := internal.Pad(w, h, []string{
		prefix + selectionStyle.Render("first"),
		"  " + "second",
		"  " + "third",
		"",
		"",
		"33% (1/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// move selection down
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(w, h, []string{
		"  " + "first",
		prefix + selectionStyle.Render("second"),
		"  " + "third",
		"",
		"",
		"66% (2/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// verify content width is reduced (long line truncated at contentWidth = 18)
	setContent(vp, []string{"short", "this is a longer line that should truncate"})
	vp.SetSelectedItemIdx(0)
	expectedView = internal.Pad(w, h, []string{
		prefix + selectionStyle.Render("short"),
		"  " + "this is a longe...",
		"",
		"",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionPrefix_NoColor(t *testing.T) {
	// simulates NO_COLOR: all styles are empty, only the prefix shows selection
	w, h := 20, 5
	prefix := "> "
	emptyStyle := lipgloss.NewStyle()

	vp := newViewport(w, h, WithStyles[object](Styles{
		SelectionPrefix:   prefix,
		FooterStyle:       emptyStyle,
		SelectedItemStyle: emptyStyle,
	}))
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{"alpha", "beta"})

	expectedView := internal.Pad(w, h, []string{
		// selected line has prefix but no style (emptyStyle is a no-op)
		prefix + "alpha",
		"  " + "beta",
		"",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_SetSameDimensionsPreservesScrollPosition(t *testing.T) {
	w, h := 10, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
		"seventh",
		"eighth",
	})

	// move selection to fifth item, causing a scroll
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView := internal.Pad(w, h, []string{
		"header",
		"third",
		"fourth",
		internal.BlueFg.Render("fifth"),
		"62% (5/8)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// setting the same width and height should not change the scroll position
	vp.SetWidth(w)
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetHeight(h)
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionOn_WrapOff_ChangeHeightPreservesSelectionPosition(t *testing.T) {
	w, h := 10, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{
		"first",
		"second",
		"third",
		"fourth",
		"fifth",
		"sixth",
		"seventh",
		"eighth",
	})

	// move selection to fifth item
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	vp, _ = vp.Update(downKeyMsg)
	expectedView := internal.Pad(w, h, []string{
		"header",
		"second",
		"third",
		"fourth",
		internal.BlueFg.Render("fifth"),
		"62% (5/8)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// increase height - selection should remain visible and not jump to the top
	vp.SetHeight(10)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"first",
		"second",
		"third",
		"fourth",
		internal.BlueFg.Render("fifth"),
		"sixth",
		"seventh",
		"eighth",
		"62% (5/8)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// reduce height - selection should still be visible
	vp.SetHeight(4)
	expectedView = internal.Pad(vp.GetWidth(), vp.GetHeight(), []string{
		"header",
		"fourth",
		internal.BlueFg.Render("fifth"),
		"62% (5/8)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_SelectionPrefix_EmptyPrefix(t *testing.T) {
	// when SelectionPrefix is empty, no prefix or padding is added
	w, h := 20, 5
	vp := newViewport(w, h) // default test helper has empty prefix
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{"first", "second"})

	expectedView := internal.Pad(w, h, []string{
		selectionStyle.Render("first"),
		"second",
		"",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}
