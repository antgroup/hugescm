package patchview

import (
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/diferenco"
)

// --- helpers ---------------------------------------------------------------

func mustPatch(t *testing.T, fromName, toName string, lines []diferenco.Line) *diferenco.Patch {
	t.Helper()
	var from, to *diferenco.File
	if fromName != "" {
		from = &diferenco.File{Name: fromName}
	}
	if toName != "" {
		to = &diferenco.File{Name: toName}
	}
	return &diferenco.Patch{
		From: from,
		To:   to,
		Hunks: []*diferenco.Hunk{{
			FromLine: 1,
			ToLine:   1,
			Lines:    lines,
		}},
	}
}

// --- scrollPercent ---------------------------------------------------------

func TestScrollPercent(t *testing.T) {
	cases := []struct {
		name              string
		y, vpH, total, ex int
	}{
		{"empty content", 0, 10, 0, 0},
		{"negative total clamps", 0, 10, -1, 0},
		{"at top", 0, 10, 100, 10},
		{"at bottom exactly", 90, 10, 100, 100},
		{"past bottom clamps", 200, 10, 100, 100},
		{"viewport larger than content", 0, 50, 10, 100},
		{"middle", 50, 10, 100, 60},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := scrollPercent(tc.y, tc.vpH, tc.total); got != tc.ex {
				t.Fatalf("scrollPercent(%d,%d,%d)=%d want %d",
					tc.y, tc.vpH, tc.total, got, tc.ex)
			}
		})
	}
}

// --- layout decision -------------------------------------------------------

func TestPatchRendererLayoutDecisionConsistency(t *testing.T) {
	// Build a patch with a few lines so beforeNumDigits/afterNumDigits are
	// stable and small.
	p := mustPatch(t, "a.txt", "a.txt", []diferenco.Line{
		{Kind: diferenco.Equal, Content: "x\n"},
		{Kind: diferenco.Insert, Content: "y\n"},
	})
	r := NewPatchRenderer(true)
	r.SetPatch(p)
	r.SetSize(80, 5)

	cases := []struct {
		name     string
		width    int
		wantNums bool
	}{
		{"wide", 80, true},
		{"narrow forces hide", 5, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r.SetSize(tc.width, 5)
			d := r.layoutDecision()
			if d.showLineNums != tc.wantNums {
				t.Fatalf("showLineNums=%v want %v", d.showLineNums, tc.wantNums)
			}
			// codeWidth + lineNumWidth must always equal width (within
			// the chosen decision), so the renderer never overflows or
			// under-fills.
			if d.codeWidth+d.lineNumWidth != tc.width {
				t.Fatalf("codeWidth(%d)+lineNumWidth(%d) != width(%d)",
					d.codeWidth, d.lineNumWidth, tc.width)
			}
			if d.codeWidth < 0 {
				t.Fatalf("codeWidth went negative: %d", d.codeWidth)
			}
		})
	}
}

func TestPatchRendererLayoutZeroWidth(t *testing.T) {
	r := NewPatchRenderer(true)
	r.SetSize(0, 0)
	d := r.layoutDecision()
	if d.showLineNums || d.codeWidth != 0 || d.lineNumWidth != 0 {
		t.Fatalf("expected zero layout for zero width, got %+v", d)
	}
}

// --- patchName / patchStatus ----------------------------------------------

func TestPatchNameAndStatus(t *testing.T) {
	cases := []struct {
		name       string
		p          *diferenco.Patch
		wantName   string
		wantStatus string
	}{
		{
			name:       "added (no from)",
			p:          &diferenco.Patch{To: &diferenco.File{Name: "new.go"}},
			wantName:   "new.go",
			wantStatus: "A",
		},
		{
			name:       "deleted (no to)",
			p:          &diferenco.Patch{From: &diferenco.File{Name: "old.go"}},
			wantName:   "old.go",
			wantStatus: "D",
		},
		{
			name: "renamed",
			p: &diferenco.Patch{
				From: &diferenco.File{Name: "a.go"},
				To:   &diferenco.File{Name: "b.go"},
			},
			wantName:   "a.go → b.go",
			wantStatus: "R",
		},
		{
			name: "modified (same name)",
			p: &diferenco.Patch{
				From: &diferenco.File{Name: "a.go"},
				To:   &diferenco.File{Name: "a.go"},
			},
			wantName:   "a.go",
			wantStatus: "M",
		},
		{
			name:       "nil patch",
			p:          nil,
			wantName:   "",
			wantStatus: "M",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := patchName(tc.p); got != tc.wantName {
				t.Errorf("patchName=%q want %q", got, tc.wantName)
			}
			if got := patchStatus(tc.p); got != tc.wantStatus {
				t.Errorf("patchStatus=%q want %q", got, tc.wantStatus)
			}
		})
	}
}

// --- truncatePath ----------------------------------------------------------

func TestTruncatePath(t *testing.T) {
	cases := []struct {
		name   string
		input  string
		maxW   int
		expect string
	}{
		{"empty width", "long/path.go", 0, ""},
		{"single ellipsis", "long/path.go", 1, "…"},
		{"fits exactly", "abc.go", 6, "abc.go"},
		{"fits with room", "a.go", 10, "a.go"},
		{"truncates from left", "a/b/c.go", 5, "…c.go"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := truncatePath(tc.input, tc.maxW); got != tc.expect {
				t.Fatalf("truncatePath(%q,%d)=%q want %q", tc.input, tc.maxW, got, tc.expect)
			}
		})
	}
}

// --- patchItem selection through shared cursor ----------------------------

func TestPatchItemSelectionFollowsCursorPointer(t *testing.T) {
	style := DefaultStyle()
	cursor := 0
	p := &diferenco.Patch{To: &diferenco.File{Name: "a.go"}}

	item0 := newPatchItem(p, 0, &cursor, 40, style)
	item1 := newPatchItem(p, 1, &cursor, 40, style)

	if !item0.isSelected() {
		t.Fatalf("expected item0 to be selected when cursor=0")
	}
	if item1.isSelected() {
		t.Fatalf("expected item1 to NOT be selected when cursor=0")
	}

	cursor = 1
	if item0.isSelected() {
		t.Fatalf("expected item0 to NOT be selected when cursor=1")
	}
	if !item1.isSelected() {
		t.Fatalf("expected item1 to be selected when cursor=1")
	}
}

func TestPatchItemNilCursorPointerIsNotSelected(t *testing.T) {
	style := DefaultStyle()
	p := &diferenco.Patch{To: &diferenco.File{Name: "a.go"}}
	item := newPatchItem(p, 0, nil, 40, style)
	if item.isSelected() {
		t.Fatalf("expected item with nil cursor to be unselected")
	}
}

// --- NewPatchView constructor side effects --------------------------------

func TestNewPatchViewIsConstructionPure(t *testing.T) {
	// Constructing a PatchView must not perform terminal IO. We can only
	// observe this indirectly: the default dark theme should be applied
	// and no panic should occur with arbitrary stdin/stdout state. The
	// real value of this test is regression protection against someone
	// re-introducing hasDarkBackground() into NewPatchView.
	patches := []*diferenco.Patch{
		{To: &diferenco.File{Name: "a.go"}, Hunks: []*diferenco.Hunk{{
			FromLine: 1, ToLine: 1,
			Lines: []diferenco.Line{{Kind: diferenco.Insert, Content: "x\n"}},
		}}},
	}
	pv := NewPatchView(patches)
	if !pv.darkBackground {
		t.Fatalf("expected default theme to be dark")
	}
	if pv.darkBackgroundExplicit {
		t.Fatalf("expected darkBackgroundExplicit to be false")
	}
}

func TestWithDarkBackgroundOption(t *testing.T) {
	patches := []*diferenco.Patch{
		{To: &diferenco.File{Name: "a.go"}, Hunks: []*diferenco.Hunk{{
			FromLine: 1, ToLine: 1,
			Lines: []diferenco.Line{{Kind: diferenco.Insert, Content: "x\n"}},
		}}},
	}
	pv := NewPatchView(patches, WithDarkBackground(false))
	if pv.darkBackground {
		t.Fatalf("expected darkBackground=false")
	}
	if !pv.darkBackgroundExplicit {
		t.Fatalf("expected darkBackgroundExplicit=true")
	}
	// The renderer's isDark must follow.
	if pv.renderer.isDark {
		t.Fatalf("expected renderer.isDark=false after WithDarkBackground(false)")
	}
}

// --- rebuildFileList + syncListSelection ----------------------------------

func TestRebuildFileListBuildsItemsWithSharedCursor(t *testing.T) {
	patches := []*diferenco.Patch{
		{To: &diferenco.File{Name: "a.go"}},
		{To: &diferenco.File{Name: "b.go"}},
		{To: &diferenco.File{Name: "c.go"}},
	}
	pv := NewPatchView(patches)
	pv.width = 80
	pv.height = 24
	pv.setupLayout()

	if got, want := len(pv.listItems), len(patches); got != want {
		t.Fatalf("listItems length=%d want %d", got, want)
	}
	for i, it := range pv.listItems {
		if it.index != i {
			t.Errorf("item[%d].index=%d want %d", i, it.index, i)
		}
		if it.cursorPtr != &pv.cursor {
			t.Errorf("item[%d].cursorPtr should point at pv.cursor", i)
		}
	}

	// After selectFile, no rebuild should happen: the slice identity
	// (the backing items) must be the same.
	itemsBefore := pv.listItems
	pv.selectFile(2)
	if &pv.listItems[0] != &itemsBefore[0] {
		t.Fatalf("selectFile should not rebuild listItems slice")
	}
	if pv.cursor != 2 {
		t.Fatalf("cursor=%d want 2", pv.cursor)
	}
	// Visible selection on item[2] must follow the cursor.
	if !pv.listItems[2].isSelected() {
		t.Fatalf("item[2] should be selected after selectFile(2)")
	}
	if pv.listItems[0].isSelected() {
		t.Fatalf("item[0] should be unselected after selectFile(2)")
	}
}

func TestRebuildFileListHandlesEmpty(t *testing.T) {
	pv := NewPatchView(nil)
	pv.width = 80
	pv.height = 24
	pv.setupLayout()
	if pv.listItems != nil {
		t.Fatalf("expected nil listItems for empty patches, got %v", pv.listItems)
	}
}

// --- borderColor fallback --------------------------------------------------

func TestBorderColorUsesStyleWhenPresent(t *testing.T) {
	pv := NewPatchView(nil) // dark default, both border colors set
	if got := pv.borderColor(true); got != pv.style.BorderActive {
		t.Fatalf("focused borderColor=%v want %v", got, pv.style.BorderActive)
	}
	if got := pv.borderColor(false); got != pv.style.BorderInactive {
		t.Fatalf("unfocused borderColor=%v want %v", got, pv.style.BorderInactive)
	}
}

func TestBorderColorFallsBackWhenStyleNil(t *testing.T) {
	// A style with nil border colors must not crash the renderer; the
	// fallback constants are used instead.
	custom := DefaultDarkStyle()
	custom.BorderActive = nil
	custom.BorderInactive = nil
	pv := NewPatchView(nil, WithStyle(custom))
	if got := pv.borderColor(true); got != fallbackBorderActive {
		t.Fatalf("focused fallback=%v want %v", got, fallbackBorderActive)
	}
	if got := pv.borderColor(false); got != fallbackBorderInactive {
		t.Fatalf("unfocused fallback=%v want %v", got, fallbackBorderInactive)
	}
}

// --- rendered file list does not double-render selection ------------------

func TestPatchItemRenderRespectsCursor(t *testing.T) {
	patches := []*diferenco.Patch{
		{To: &diferenco.File{Name: "alpha.go"}, Hunks: []*diferenco.Hunk{{
			FromLine: 1, ToLine: 1,
			Lines: []diferenco.Line{{Kind: diferenco.Insert, Content: "x\n"}},
		}}},
		{To: &diferenco.File{Name: "beta.go"}, Hunks: []*diferenco.Hunk{{
			FromLine: 1, ToLine: 1,
			Lines: []diferenco.Line{{Kind: diferenco.Insert, Content: "y\n"}},
		}}},
	}
	pv := NewPatchView(patches)
	pv.width = 80
	pv.height = 24
	pv.setupLayout()

	// Cursor at 0: item[0]'s rendered string should contain the selected
	// glyph "▌" while item[1]'s should not.
	r0 := pv.listItems[0].render()
	r1 := pv.listItems[1].render()
	if !strings.Contains(r0, "▌") {
		t.Fatalf("expected selected glyph in item[0] render; got %q", r0)
	}
	if strings.Contains(r1, "▌") {
		t.Fatalf("did not expect selected glyph in item[1] render; got %q", r1)
	}

	// Move cursor without rebuilding the slice.
	pv.cursor = 1
	r0b := pv.listItems[0].render()
	r1b := pv.listItems[1].render()
	if strings.Contains(r0b, "▌") {
		t.Fatalf("expected item[0] to lose selection after cursor moved; got %q", r0b)
	}
	if !strings.Contains(r1b, "▌") {
		t.Fatalf("expected item[1] to gain selection after cursor moved; got %q", r1b)
	}
}

// --- style application ----------------------------------------------------

func TestWithStyleSuppressesThemeOverride(t *testing.T) {
	// Build a unique style we can identify by its addition foreground.
	custom := DefaultDarkStyle()
	custom.BorderActive = nil // sentinel to detect propagation

	pv := NewPatchView(nil, WithStyle(custom))
	if !pv.styleExplicit {
		t.Fatalf("expected styleExplicit=true when WithStyle is used")
	}
	pv.applyTheme(false)
	// styleExplicit means applyTheme must NOT overwrite our style.
	if pv.style.BorderActive != nil {
		t.Fatalf("applyTheme overwrote user-provided style")
	}
}

func TestApplyThemeUpdatesItemStyle(t *testing.T) {
	patches := []*diferenco.Patch{
		{To: &diferenco.File{Name: "a.go"}, Hunks: []*diferenco.Hunk{{
			FromLine: 1, ToLine: 1,
			Lines: []diferenco.Line{{Kind: diferenco.Insert, Content: "x\n"}},
		}}},
	}
	pv := NewPatchView(patches)
	pv.width = 80
	pv.height = 24
	pv.setupLayout()

	// Switch theme to light and verify the items now hold the light
	// style. We compare by Addition foreground which differs between
	// the bundled dark/light themes.
	pv.applyTheme(false)
	wantFg := DefaultLightStyle().Addition.GetForeground()
	gotFg := pv.listItems[0].style.Addition.GetForeground()
	if gotFg != wantFg {
		t.Fatalf("applyTheme did not propagate to listItems; got %v want %v", gotFg, wantFg)
	}
}

// TestWithDarkBackgroundPropagatesToStatusBar is a regression test for an
// option-ordering bug: WithDarkBackground used to invoke applyTheme inline
// at option-application time, before NewPatchView had constructed the
// default status bar. The result was that the status bar kept the dark
// default style even when the caller requested a light theme. This test
// pins the desired behaviour: after NewPatchView returns, the status bar
// must reflect the requested theme regardless of option order.
func TestWithDarkBackgroundPropagatesToStatusBar(t *testing.T) {
	patches := []*diferenco.Patch{
		{To: &diferenco.File{Name: "a.go"}, Hunks: []*diferenco.Hunk{{
			FromLine: 1, ToLine: 1,
			Lines: []diferenco.Line{{Kind: diferenco.Insert, Content: "x\n"}},
		}}},
	}

	pv := NewPatchView(patches, WithDarkBackground(false))
	sb, ok := pv.statusBar.(*DefaultStatusBar)
	if !ok {
		t.Fatalf("expected DefaultStatusBar, got %T", pv.statusBar)
	}

	wantFg := DefaultLightStyle().Addition.GetForeground()
	gotFg := sb.style.Addition.GetForeground()
	if gotFg != wantFg {
		t.Fatalf("status bar style not updated by WithDarkBackground(false); got %v want %v", gotFg, wantFg)
	}

	// Reverse order: explicit dark must also propagate even though dark is
	// already the construction default — guards against the symmetric
	// regression.
	pv2 := NewPatchView(patches, WithDarkBackground(true))
	sb2 := pv2.statusBar.(*DefaultStatusBar)
	wantFg2 := DefaultDarkStyle().Addition.GetForeground()
	if got := sb2.style.Addition.GetForeground(); got != wantFg2 {
		t.Fatalf("status bar style not updated by WithDarkBackground(true); got %v want %v", got, wantFg2)
	}
}

// TestWithDarkBackgroundOptionOrderDoesNotMatter ensures the user-facing
// theme is applied regardless of whether WithDarkBackground comes before
// or after WithStatusBar. Previously WithDarkBackground required ordering
// because it touched components inline.
func TestWithDarkBackgroundOptionOrderDoesNotMatter(t *testing.T) {
	patches := []*diferenco.Patch{
		{To: &diferenco.File{Name: "a.go"}},
	}

	// Custom status bar provided BEFORE the theme option.
	sb := NewDefaultStatusBar()
	pv := NewPatchView(patches, WithStatusBar(sb), WithDarkBackground(false))
	if pv.statusBar != sb {
		t.Fatalf("WithStatusBar should be honoured")
	}
	wantFg := DefaultLightStyle().Addition.GetForeground()
	gotFg := sb.style.Addition.GetForeground()
	if gotFg != wantFg {
		t.Fatalf("custom status bar must receive the light style; got %v want %v", gotFg, wantFg)
	}

	// Custom status bar provided AFTER the theme option.
	sb2 := NewDefaultStatusBar()
	pv2 := NewPatchView(patches, WithDarkBackground(false), WithStatusBar(sb2))
	if pv2.statusBar != sb2 {
		t.Fatalf("WithStatusBar should be honoured (reverse order)")
	}
	if got := sb2.style.Addition.GetForeground(); got != wantFg {
		t.Fatalf("custom status bar must receive light style regardless of option order; got %v want %v", got, wantFg)
	}
}
