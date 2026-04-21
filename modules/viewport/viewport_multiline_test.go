package viewport

import (
	"testing"

	"github.com/antgroup/hugescm/modules/viewport/internal"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

// multiLineObject wraps a MultiLineItem for use in viewport tests
type multiLineObject struct {
	item item.Item
}

func (o multiLineObject) GetItem() item.Item {
	return o.item
}

var _ Object = multiLineObject{}

func setMixedContent(vp *Model[object], items []item.Item) {
	objects := make([]object, len(items))
	for i := range items {
		objects[i] = object{item: items[i]}
	}
	vp.SetObjects(objects)
}

func TestViewport_MultiLine_WrapOn_Basic(t *testing.T) {
	w, h := 15, 7
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)

	// Object 0: multi-line item with 3 segments
	// Object 1: regular single-line item
	setMixedContent(vp, []item.Item{
		item.NewMultiLineItem(
			item.NewItem("{"),
			item.NewItem("  \"k\": \"val\""), // 12 cells, fits in 15-wide viewport
			item.NewItem("}"),
		),
		item.NewItem("single line"),
	})

	expectedView := internal.Pad(w, h, []string{
		"header",
		internal.BlueFg.Render("{"), // segment 0 (selected)
		internal.BlueFg.Render("  \"k\": \"val\""), // segment 1 (selected, 12 cells)
		internal.BlueFg.Render("}"),                // segment 2 (selected)
		"single line",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_MultiLine_WrapOn_SelectionMovement(t *testing.T) {
	w, h := 20, 7
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)

	setMixedContent(vp, []item.Item{
		item.NewMultiLineItem(
			item.NewItem("line one"),
			item.NewItem("line two"),
		),
		item.NewItem("after"),
	})

	// Initially selected: first item (multi-line)
	expectedView := internal.Pad(w, h, []string{
		"header",
		internal.BlueFg.Render("line one"),
		internal.BlueFg.Render("line two"),
		"after",
		"",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// Move selection down to "after"
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(w, h, []string{
		"header",
		"line one",
		"line two",
		internal.BlueFg.Render("after"),
		"",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_MultiLine_WrapOn_EmptySegment(t *testing.T) {
	w, h := 20, 7
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)

	// Multi-line item with an empty segment in the middle
	setMixedContent(vp, []item.Item{
		item.NewMultiLineItem(
			item.NewItem("above"),
			item.NewItem(""),
			item.NewItem("below"),
		),
	})

	expectedView := internal.Pad(w, h, []string{
		"header",
		internal.BlueFg.Render("above"),
		internal.BlueFg.Render(" "), // empty segment shows selection marker
		internal.BlueFg.Render("below"),
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_MultiLine_WrapOn_SegmentWrapping(t *testing.T) {
	w, h := 10, 8
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)

	// Each segment wraps independently
	setMixedContent(vp, []item.Item{
		item.NewMultiLineItem(
			item.NewItem("abcdefghij12"), // 12 cells, wraps to 2 lines at width 10
			item.NewItem("xyz"),          // 3 cells, 1 line
		),
	})

	expectedView := internal.Pad(w, h, []string{
		"header",
		internal.BlueFg.Render("abcdefghij"), // segment 0, line 1
		internal.BlueFg.Render("12"),         // segment 0, line 2
		internal.BlueFg.Render("xyz"),        // segment 1
		"",
		"",
		"",
		"100% (1/1)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_MultiLine_WrapOn_ScrollDown(t *testing.T) {
	w, h := 20, 5
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)

	setMixedContent(vp, []item.Item{
		item.NewMultiLineItem(
			item.NewItem("seg1"),
			item.NewItem("seg2"),
			item.NewItem("seg3"),
		),
		item.NewItem("next item"),
	})

	// Initial view: header + 3 segment lines, fills viewport
	expectedView := internal.Pad(w, h, []string{
		"header",
		internal.BlueFg.Render("seg1"),
		internal.BlueFg.Render("seg2"),
		internal.BlueFg.Render("seg3"),
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// Scroll down to next item
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(w, h, []string{
		"header",
		"seg2",
		"seg3",
		internal.BlueFg.Render("next item"),
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_MultiLine_WrapOn_NoSelection(t *testing.T) {
	w, h := 20, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(false)

	setMixedContent(vp, []item.Item{
		item.NewMultiLineItem(
			item.NewItem("first"),
			item.NewItem("second"),
		),
		item.NewItem("third"),
	})

	expectedView := internal.Pad(w, h, []string{
		"header",
		"first",
		"second",
		"third",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_MultiLine_WrapOn_SingleLineItemsUnchanged(t *testing.T) {
	// Verify that single-line items behave identically with the new code
	w, h := 15, 6
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
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
		internal.RedFg.Render("a really really"),
		internal.RedFg.Render(" long line"),
		"25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestViewport_MultiLine_WrapOn_MultipleMultiLineItems(t *testing.T) {
	w, h := 20, 8
	vp := newViewport(w, h)
	vp.SetHeader([]string{"header"})
	vp.SetWrapText(true)
	vp.SetSelectionEnabled(true)

	setMixedContent(vp, []item.Item{
		item.NewMultiLineItem(
			item.NewItem("a1"),
			item.NewItem("a2"),
		),
		item.NewMultiLineItem(
			item.NewItem("b1"),
			item.NewItem("b2"),
		),
	})

	// First multi-line item selected
	expectedView := internal.Pad(w, h, []string{
		"header",
		internal.BlueFg.Render("a1"),
		internal.BlueFg.Render("a2"),
		"b1",
		"b2",
		"",
		"",
		"50% (1/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	// Move down to second multi-line item
	vp, _ = vp.Update(downKeyMsg)
	expectedView = internal.Pad(w, h, []string{
		"header",
		"a1",
		"a2",
		internal.BlueFg.Render("b1"),
		internal.BlueFg.Render("b2"),
		"",
		"",
		"100% (2/2)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}
