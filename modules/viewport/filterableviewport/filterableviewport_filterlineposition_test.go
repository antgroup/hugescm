package filterableviewport

import (
	"testing"

	"github.com/antgroup/hugescm/modules/viewport"
	"github.com/antgroup/hugescm/modules/viewport/internal"
)

func TestFilterLinePositionTop(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithEmptyText[object]("No Filter"),
			WithFilterLinePosition[object](FilterLineTop),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
	}))

	// Filter line should appear at top (just below header, which is empty)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"No Filter",
		"line 1",
		"line 2",
		"line 3",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestFilterLinePositionTopWithActiveFilter(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithEmptyText[object]("No Filter"),
			WithFilterLinePosition[object](FilterLineTop),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
	}))

	// Apply a filter
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('l'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"[exact] Filter: l  (1/3 matches on 3 items)",
		focusedStyle.Render("l") + "ine 1",
		unfocusedStyle.Render("l") + "ine 2",
		unfocusedStyle.Render("l") + "ine 3",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestFilterLinePositionTopWithHeader(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		6,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithEmptyText[object]("No Filter"),
			WithFilterLinePosition[object](FilterLineTop),
		},
	)
	fv.SetHeader([]string{"My Header"})
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
	}))

	// Header, then filter line, then content, then footer
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"My Header",
		"No Filter",
		"line 1",
		"line 2",
		"line 3",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestFilterLinePositionTopDuringEditing(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithFilterLinePosition[object](FilterLineTop),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
	}))

	// Enter filter editing mode
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('t'))
	fv, _ = fv.Update(internal.MakeKeyMsg('e'))

	// Filter line with cursor should appear at top
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"[exact] Filter: te" + cursorStyle.Render(" ") + " (no matches)",
		"line 1",
		"line 2",
		"line 3",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestFilterLinePositionBottomIsDefault(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithEmptyText[object]("No Filter"),
			// No WithFilterLinePosition - should default to bottom
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
	}))

	// Filter line should appear at bottom (default behavior)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"line 1",
		"line 2",
		"line 3",
		"No Filter",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestFilterLinePositionTopScrolling(t *testing.T) {
	fv := makeFilterableViewport(
		50,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithEmptyText[object]("No Filter"),
			WithFilterLinePosition[object](FilterLineTop),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
	}))

	// Filter line at top, 3 content lines visible (height 5 - 1 filter - 1 footer = 3)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"No Filter",
		"line 1",
		"line 2",
		"line 3",
		footerStyle.Render("50% (3/6)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Scroll down
	fv, _ = fv.Update(downKeyMsg)
	expectedAfterScroll := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"No Filter",
		"line 2",
		"line 3",
		"line 4",
		footerStyle.Render("66% (4/6)"),
	})
	internal.CmpStr(t, expectedAfterScroll, fv.View())
}

func TestFilterLinePositionTopWithWrap(t *testing.T) {
	fv := makeFilterableViewport(
		15,
		7,
		[]viewport.Option[object]{
			viewport.WithWrapText[object](true),
		},
		[]Option[object]{
			WithEmptyText[object]("None"),
			WithFilterLinePosition[object](FilterLineTop),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"short",
		"longer text that wraps",
	}))

	// Filter line at top, then content (with wrapping)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"None",
		"short",
		"longer text tha",
		"t wraps",
		"",
		"",
		footerStyle.Render("100% (2/2)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}

func TestFilterLinePositionTopMatchNavigation(t *testing.T) {
	fv := makeFilterableViewport(
		60,
		5,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithFilterLinePosition[object](FilterLineTop),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"apple",
		"banana",
		"apricot",
	}))

	// Apply filter
	fv, _ = fv.Update(filterKeyMsg)
	fv, _ = fv.Update(internal.MakeKeyMsg('a'))
	fv, _ = fv.Update(applyFilterKeyMsg)

	// First match focused (apple=1, banana=3, apricot=1 = 5 total matches)
	expectedView := internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"[exact] Filter: a  (1/5 matches on 3 items)",
		focusedStyle.Render("a") + "pple",
		"b" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		unfocusedStyle.Render("a") + "pricot",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())

	// Navigate to next match
	fv, _ = fv.Update(nextMatchKeyMsg)
	expectedView = internal.Pad(fv.GetWidth(), fv.GetHeight(), []string{
		"[exact] Filter: a  (2/5 matches on 3 items)",
		unfocusedStyle.Render("a") + "pple",
		"b" + focusedStyle.Render("a") + "n" + unfocusedStyle.Render("a") + "n" + unfocusedStyle.Render("a"),
		unfocusedStyle.Render("a") + "pricot",
		footerStyle.Render("100% (3/3)"),
	})
	internal.CmpStr(t, expectedView, fv.View())
}
