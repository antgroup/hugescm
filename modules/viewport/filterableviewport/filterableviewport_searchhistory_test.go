package filterableviewport

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/antgroup/hugescm/modules/viewport"
	"github.com/antgroup/hugescm/modules/viewport/internal"
)

var upKeyMsg = tea.KeyPressMsg{Code: tea.KeyUp, Text: "up"}

func makeSearchHistoryFV() *Model[object] {
	fv := makeFilterableViewport(
		40,
		10,
		[]viewport.Option[object]{},
		[]Option[object]{
			WithPrefixText[object]("Filter:"),
			WithEmptyText[object]("No Filter"),
		},
	)
	fv.SetObjects(stringsToItems([]string{
		"alpha",
		"bravo",
		"charlie",
		"delta",
		"echo",
	}))
	return fv
}

func typeFilter(fv *Model[object], text string) {
	for _, ch := range text {
		fv.Update(internal.MakeKeyMsg(ch))
	}
}

func applyFilter(fv *Model[object], text string) {
	fv.Update(cancelFilterKeyMsg) // clear any existing filter text
	fv.Update(filterKeyMsg)
	typeFilter(fv, text)
	fv.Update(applyFilterKeyMsg)
}

func TestSearchHistoryBasic(t *testing.T) {
	fv := makeSearchHistoryFV()

	applyFilter(fv, "alpha")
	applyFilter(fv, "bravo")

	// re-enter filter mode, Up shows most recent
	fv.Update(filterKeyMsg)
	fv.Update(upKeyMsg)
	if fv.filterTextInput.Value() != "bravo" {
		t.Errorf("expected 'bravo', got %q", fv.filterTextInput.Value())
	}

	// Up again shows older
	fv.Update(upKeyMsg)
	if fv.filterTextInput.Value() != "alpha" {
		t.Errorf("expected 'alpha', got %q", fv.filterTextInput.Value())
	}
}

func TestSearchHistoryNoConsecutiveDuplicates(t *testing.T) {
	fv := makeSearchHistoryFV()

	applyFilter(fv, "alpha")
	applyFilter(fv, "alpha")

	if len(fv.searchHistory) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(fv.searchHistory))
	}

	// non-consecutive duplicate is allowed
	applyFilter(fv, "bravo")
	applyFilter(fv, "alpha")

	if len(fv.searchHistory) != 3 {
		t.Errorf("expected 3 history entries, got %d: %v", len(fv.searchHistory), fv.searchHistory)
	}
}

func TestSearchHistoryDraftPreserved(t *testing.T) {
	fv := makeSearchHistoryFV()

	applyFilter(fv, "alpha")

	// enter filter mode with clean text and type a draft
	fv.Update(cancelFilterKeyMsg)
	fv.Update(filterKeyMsg)
	typeFilter(fv, "draft")

	// Up should save draft and show history
	fv.Update(upKeyMsg)
	if fv.filterTextInput.Value() != "alpha" {
		t.Errorf("expected 'alpha', got %q", fv.filterTextInput.Value())
	}

	// Down should return to draft
	fv.Update(downKeyMsg)
	if fv.filterTextInput.Value() != "draft" {
		t.Errorf("expected 'draft', got %q", fv.filterTextInput.Value())
	}
}

func TestSearchHistoryUpAtOldest(t *testing.T) {
	fv := makeSearchHistoryFV()

	applyFilter(fv, "alpha")
	applyFilter(fv, "bravo")

	fv.Update(filterKeyMsg)
	fv.Update(upKeyMsg) // bravo
	fv.Update(upKeyMsg) // alpha
	fv.Update(upKeyMsg) // should stay at alpha

	if fv.filterTextInput.Value() != "alpha" {
		t.Errorf("expected 'alpha', got %q", fv.filterTextInput.Value())
	}
}

func TestSearchHistoryDownAtDraft(t *testing.T) {
	fv := makeSearchHistoryFV()

	applyFilter(fv, "alpha")

	fv.Update(cancelFilterKeyMsg)
	fv.Update(filterKeyMsg)
	typeFilter(fv, "current")

	// Down at draft position should be no-op
	fv.Update(downKeyMsg)
	if fv.filterTextInput.Value() != "current" {
		t.Errorf("expected 'current', got %q", fv.filterTextInput.Value())
	}
}

func TestSearchHistoryEmptyNotSaved(t *testing.T) {
	fv := makeSearchHistoryFV()

	// apply with empty text
	fv.Update(filterKeyMsg)
	fv.Update(applyFilterKeyMsg)

	if len(fv.searchHistory) != 0 {
		t.Errorf("expected 0 history entries, got %d", len(fv.searchHistory))
	}
}

func TestSearchHistoryResetOnReEnter(t *testing.T) {
	fv := makeSearchHistoryFV()

	applyFilter(fv, "alpha")
	applyFilter(fv, "bravo")

	// enter filter mode, browse history
	fv.Update(filterKeyMsg)
	fv.Update(upKeyMsg) // bravo
	fv.Update(upKeyMsg) // alpha

	// cancel and re-enter
	fv.Update(cancelFilterKeyMsg)
	fv.Update(filterKeyMsg)

	// should start at draft (empty), not mid-browse
	fv.Update(upKeyMsg)
	if fv.filterTextInput.Value() != "bravo" {
		t.Errorf("expected 'bravo' (most recent), got %q", fv.filterTextInput.Value())
	}
}

func TestSearchHistoryUpDownNoHistory(t *testing.T) {
	fv := makeSearchHistoryFV()

	// enter filter mode with no history
	fv.Update(filterKeyMsg)
	typeFilter(fv, "test")

	// Up/Down should not change text (no history to browse)
	fv.Update(upKeyMsg)
	if fv.filterTextInput.Value() != "test" {
		t.Errorf("expected 'test' unchanged, got %q", fv.filterTextInput.Value())
	}

	fv.Update(downKeyMsg)
	if fv.filterTextInput.Value() != "test" {
		t.Errorf("expected 'test' unchanged, got %q", fv.filterTextInput.Value())
	}
}

func TestSearchHistoryLimit(t *testing.T) {
	fv := makeSearchHistoryFV()

	for i := range maxSearchHistorySize + 1 {
		applyFilter(fv, fmt.Sprintf("search%d", i))
	}

	if len(fv.searchHistory) != maxSearchHistorySize {
		t.Errorf("expected %d history entries, got %d", maxSearchHistorySize, len(fv.searchHistory))
	}

	// oldest entry should have been trimmed
	if fv.searchHistory[0] != "search1" {
		t.Errorf("expected oldest entry 'search1', got %q", fv.searchHistory[0])
	}

	// newest should be the last one
	if fv.searchHistory[len(fv.searchHistory)-1] != fmt.Sprintf("search%d", maxSearchHistorySize) {
		t.Errorf("expected newest entry 'search%d', got %q", maxSearchHistorySize, fv.searchHistory[len(fv.searchHistory)-1])
	}
}

func TestSearchHistoryUpDownNotEditingDoesNotBrowseHistory(t *testing.T) {
	fv := makeSearchHistoryFV()

	applyFilter(fv, "alpha")
	applyFilter(fv, "bravo")

	// cancel filter so we're not editing
	fv.Update(cancelFilterKeyMsg)

	// verify we're not in editing mode
	if fv.filterMode != filterModeOff {
		t.Fatalf("expected filterModeOff, got %d", fv.filterMode)
	}

	// down/up should not change filter text input (should go to viewport)
	fv.Update(downKeyMsg)
	fv.Update(upKeyMsg)

	// re-enter filter mode - text should be empty (cleared by cancel), not a history entry
	fv.Update(filterKeyMsg)
	if fv.filterTextInput.Value() != "" {
		t.Errorf("expected empty filter text, got %q", fv.filterTextInput.Value())
	}
}

func TestSearchHistoryCaseInsensitiveNoPrefix(t *testing.T) {
	fv := makeSearchHistoryFV()

	// apply a plain exact search
	applyFilter(fv, "butt")

	// enter case-insensitive mode and browse history
	fv.Update(cancelFilterKeyMsg)
	fv.Update(caseInsensitiveFilterKeyMsg)
	fv.Update(upKeyMsg)

	// history text is stored without any prefix — mode is separate
	if fv.filterTextInput.Value() != "butt" {
		t.Errorf("expected 'butt', got %q", fv.filterTextInput.Value())
	}
}

func TestSearchHistoryCaseInsensitiveStoredPlain(t *testing.T) {
	fv := makeSearchHistoryFV()

	// apply a case-insensitive search
	fv.Update(cancelFilterKeyMsg)
	fv.Update(caseInsensitiveFilterKeyMsg)
	typeFilter(fv, "butt")
	fv.Update(applyFilterKeyMsg)

	// re-enter case-insensitive mode and browse history
	fv.Update(cancelFilterKeyMsg)
	fv.Update(caseInsensitiveFilterKeyMsg)
	fv.Update(upKeyMsg)

	// stored as plain "butt", no (?i) prefix
	if fv.filterTextInput.Value() != "butt" {
		t.Errorf("expected 'butt', got %q", fv.filterTextInput.Value())
	}
}

func TestSearchHistoryRegexModeNoPrefix(t *testing.T) {
	fv := makeSearchHistoryFV()

	// apply a plain exact search
	applyFilter(fv, "butt")

	// enter regex mode and browse history
	fv.Update(cancelFilterKeyMsg)
	fv.Update(regexFilterKeyMsg)
	fv.Update(upKeyMsg)

	// should show plain text without any prefix
	if fv.filterTextInput.Value() != "butt" {
		t.Errorf("expected 'butt', got %q", fv.filterTextInput.Value())
	}
}
