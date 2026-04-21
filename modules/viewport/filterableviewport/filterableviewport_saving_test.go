package filterableviewport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/antgroup/hugescm/modules/viewport"
	"github.com/antgroup/hugescm/modules/viewport/internal"
	"github.com/antgroup/hugescm/modules/viewport/item"
)

type saveTestObject struct {
	item item.Item
}

func (o saveTestObject) GetItem() item.Item {
	return o.item
}

var (
	saveKey            = key.NewBinding(key.WithKeys("ctrl+s"))
	saveKeyMsg         = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
	savingEnterKeyMsg  = tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"}
	savingEscapeKeyMsg = tea.KeyPressMsg{Code: tea.KeyEscape, Text: "esc"}
)

func newSaveTestFilterableViewport(t *testing.T) (*Model[saveTestObject], string) {
	t.Helper()
	tmpDir := t.TempDir()
	vp := viewport.New[saveTestObject](80, 24,
		viewport.WithFileSaving[saveTestObject](tmpDir, saveKey),
	)
	fv := New[saveTestObject](vp)
	return fv, tmpDir
}

func setSaveTestObjects(fv *Model[saveTestObject], lines []string) {
	objects := make([]saveTestObject, len(lines))
	for i, line := range lines {
		objects[i] = saveTestObject{item: item.NewItem(line)}
	}
	fv.SetObjects(objects)
}

func TestFilterableViewport_AllHotkeysTypedIntoFilename(t *testing.T) {
	fv, tmpDir := newSaveTestFilterableViewport(t)
	setSaveTestObjects(fv, []string{"test content"})

	// enter filename mode
	fv, _ = fv.Update(saveKeyMsg)
	if !strings.Contains(fv.View(), "Save as:") {
		t.Fatal("expected to be in filename entry mode")
	}

	// type all filterableviewport hotkeys - should go into filename, not trigger actions
	fv, _ = fv.Update(internal.MakeKeyMsg('/')) // filter key
	fv, _ = fv.Update(internal.MakeKeyMsg('r')) // regex filter key
	fv, _ = fv.Update(internal.MakeKeyMsg('n')) // next match key
	fv, _ = fv.Update(internal.MakeKeyMsg('N')) // prev match key
	fv, _ = fv.Update(internal.MakeKeyMsg('o')) // toggle matching items only key

	// filter should not be activated
	if fv.FilterFocused() {
		t.Error("filter should not be focused during filename entry")
	}

	// save and verify filename contains all typed keys
	_, cmd := fv.Update(savingEnterKeyMsg)
	cmd()

	expectedPath := filepath.Join(tmpDir, "/rnNo.txt")
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", expectedPath)
	}
}

func TestFilterableViewport_FilterWorksAfterCancelingSave(t *testing.T) {
	fv, _ := newSaveTestFilterableViewport(t)
	setSaveTestObjects(fv, []string{"line1", "line2"})

	// enter save mode then cancel
	fv, _ = fv.Update(saveKeyMsg)
	fv, _ = fv.Update(savingEscapeKeyMsg)

	// filter should work normally
	fv, _ = fv.Update(internal.MakeKeyMsg('/'))
	if !fv.FilterFocused() {
		t.Error("expected filter to be focused after canceling save")
	}
}

func TestFilterableViewport_SaveDuringActiveFilter(t *testing.T) {
	fv, tmpDir := newSaveTestFilterableViewport(t)
	setSaveTestObjects(fv, []string{"foo one", "bar two", "foo three"})

	// apply a filter
	fv, _ = fv.Update(internal.MakeKeyMsg('/'))
	for _, r := range "foo" {
		fv, _ = fv.Update(internal.MakeKeyMsg(r))
	}
	fv, _ = fv.Update(savingEnterKeyMsg)

	// save with default filename
	fv, _ = fv.Update(saveKeyMsg)
	_, cmd := fv.Update(savingEnterKeyMsg)
	cmd()

	// find and read the saved file
	files, _ := os.ReadDir(tmpDir)
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	content, _ := os.ReadFile(filepath.Join(tmpDir, files[0].Name())) //nolint:gosec // test file path is safe
	contentStr := string(content)

	// should contain all lines, not just filtered ones
	if !strings.Contains(contentStr, "foo one") ||
		!strings.Contains(contentStr, "bar two") ||
		!strings.Contains(contentStr, "foo three") {
		t.Errorf("expected all lines in saved content, got: %s", contentStr)
	}
}
