package viewport

import (
	"testing"

	"github.com/antgroup/hugescm/modules/viewport/internal"
)

func TestProgressBarDefaultDisabled(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{"line 1", "line 2", "line 3"})

	expectedView := internal.Pad(w, h, []string{
		"line 1",
		"line 2",
		"line 3",
		"",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestProgressBarEnabled100Percent(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h, WithProgressBarEnabled[object](true))
	setContent(vp, []string{"line 1", "line 2", "line 3"})

	// "100% (3/3)" = 10 chars, barSpace=19, barWidth=min(10,19)=10, filled=10
	expectedView := internal.Pad(w, h, []string{
		"line 1",
		"line 2",
		"line 3",
		"",
		"██████████ 100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestProgressBarEnabledPartialScrolling(t *testing.T) {
	w, h := 30, 8
	vp := newViewport(w, h)
	vp.SetProgressBarEnabled(true)
	vp.SetSelectionEnabled(true)
	setContent(vp, []string{"line 1", "line 2", "line 3", "line 4"})

	// "25% (1/4)" = 9 chars, barSpace=20, barWidth=10, filled=int(10*25/100)=2
	expectedView := internal.Pad(w, h, []string{
		selectionStyle.Render("line 1"),
		"line 2",
		"line 3",
		"line 4",
		"",
		"",
		"",
		"██░░░░░░░░ 25% (1/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetSelectedItemIdx(1)
	// "50% (2/4)" = 9 chars, barWidth=10, filled=int(10*50/100)=5
	expectedView = internal.Pad(w, h, []string{
		"line 1",
		selectionStyle.Render("line 2"),
		"line 3",
		"line 4",
		"",
		"",
		"",
		"█████░░░░░ 50% (2/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetSelectedItemIdx(2)
	// "75% (3/4)" = 9 chars, barWidth=10, filled=int(10*75/100)=7
	expectedView = internal.Pad(w, h, []string{
		"line 1",
		"line 2",
		selectionStyle.Render("line 3"),
		"line 4",
		"",
		"",
		"",
		"███████░░░ 75% (3/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())

	vp.SetSelectedItemIdx(3)
	// "100% (4/4)" = 10 chars, barSpace=19, barWidth=10, filled=10
	expectedView = internal.Pad(w, h, []string{
		"line 1",
		"line 2",
		"line 3",
		selectionStyle.Render("line 4"),
		"",
		"",
		"",
		"██████████ 100% (4/4)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestProgressBarTooNarrowOmitted(t *testing.T) {
	w, h := 13, 5
	vp := newViewport(w, h, WithProgressBarEnabled[object](true))
	setContent(vp, []string{"line 1", "line 2", "line 3"})

	// "100% (3/3)" = 10 chars, barSpace = 13-10-1 = 2 < 3, no bar
	expectedView := internal.Pad(w, h, []string{
		"line 1",
		"line 2",
		"line 3",
		"",
		"100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestProgressBarMinimumWidth(t *testing.T) {
	w, h := 14, 5
	vp := newViewport(w, h, WithProgressBarEnabled[object](true))
	setContent(vp, []string{"line 1", "line 2", "line 3"})

	// "100% (3/3)" = 10 chars, barSpace = 14-10-1 = 3, barWidth=min(10,3)=3, filled=3
	expectedView := internal.Pad(w, h, []string{
		"line 1",
		"line 2",
		"line 3",
		"",
		"███ 100% (3/3)",
	})
	internal.CmpStr(t, expectedView, vp.View())
}

func TestProgressBarToggle(t *testing.T) {
	w, h := 30, 5
	vp := newViewport(w, h)
	setContent(vp, []string{"line 1", "line 2", "line 3"})

	plainFooter := internal.Pad(w, h, []string{
		"line 1",
		"line 2",
		"line 3",
		"",
		"100% (3/3)",
	})
	internal.CmpStr(t, plainFooter, vp.View())

	vp.SetProgressBarEnabled(true)
	withBar := internal.Pad(w, h, []string{
		"line 1",
		"line 2",
		"line 3",
		"",
		"██████████ 100% (3/3)",
	})
	internal.CmpStr(t, withBar, vp.View())

	vp.SetProgressBarEnabled(false)
	internal.CmpStr(t, plainFooter, vp.View())
}

func TestBuildProgressBar(t *testing.T) {
	cases := []struct {
		pct, width int
		expected   string
	}{
		{100, 10, "██████████"},
		{0, 10, "░░░░░░░░░░"},
		{50, 10, "█████░░░░░"},
		{75, 10, "███████░░░"},
		{25, 10, "██░░░░░░░░"},
		{33, 6, "█░░░░░"},
		{100, 3, "███"},
		{0, 3, "░░░"},
		{100, 0, ""},
		{50, 0, ""},
	}
	for _, c := range cases {
		got := buildProgressBar(c.pct, c.width)
		if got != c.expected {
			t.Errorf("buildProgressBar(%d, %d) = %q, want %q", c.pct, c.width, got, c.expected)
		}
	}
}
