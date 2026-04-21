package viewport

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	enterKeyMsg  = tea.KeyPressMsg{Code: tea.KeyEnter, Text: "enter"}
	escapeKeyMsg = tea.KeyPressMsg{Code: tea.KeyEscape, Text: "esc"}
	saveKey      = key.NewBinding(key.WithKeys("ctrl+s"))
	saveKeyMsg   = tea.KeyPressMsg{Code: 's', Mod: tea.ModCtrl}
)

func newSaveTestViewport(t *testing.T) (*Model[saveTestObject], string) {
	t.Helper()
	tmpDir := t.TempDir()

	vp := New[saveTestObject](80, 24,
		WithFileSaving[saveTestObject](tmpDir, saveKey),
	)
	return vp, tmpDir
}

func setSaveTestContent(vp *Model[saveTestObject], lines []string) {
	objects := make([]saveTestObject, len(lines))
	for i, line := range lines {
		objects[i] = saveTestObject{item: item.NewItem(line)}
	}
	vp.SetObjects(objects)
}

func TestFileSaving_PressingSaveKeyEntersFilenameMode(t *testing.T) {
	vp, _ := newSaveTestViewport(t)
	setSaveTestContent(vp, []string{"line1", "line2"})

	if vp.IsCapturingInput() {
		t.Error("expected IsCapturingInput to be false initially")
	}

	vp, cmd := vp.Update(saveKeyMsg)

	if !vp.IsCapturingInput() {
		t.Error("expected IsCapturingInput to be true after pressing save key")
	}
	if cmd == nil {
		t.Error("expected a command (textinput.Blink) to be returned")
	}

	// view should show save prompt
	view := vp.View()
	if !strings.Contains(view, "Save as:") {
		t.Error("expected view to contain 'Save as:' prompt")
	}
}

func TestFileSaving_EscapeCancelsFilenameEntry(t *testing.T) {
	vp, _ := newSaveTestViewport(t)
	setSaveTestContent(vp, []string{"line1", "line2"})

	vp, _ = vp.Update(saveKeyMsg)
	if !vp.IsCapturingInput() {
		t.Fatal("expected to be in filename entry mode")
	}

	vp, _ = vp.Update(escapeKeyMsg)

	if vp.IsCapturingInput() {
		t.Error("expected IsCapturingInput to be false after escape")
	}

	// view should no longer show save prompt
	view := vp.View()
	if strings.Contains(view, "Save as:") {
		t.Error("expected view to not contain 'Save as:' after escape")
	}
}

func TestFileSaving_EnterWithEmptyInputUsesTimestampDefault(t *testing.T) {
	vp, tmpDir := newSaveTestViewport(t)
	setSaveTestContent(vp, []string{"content line 1", "content line 2"})

	vp, _ = vp.Update(saveKeyMsg)
	if !vp.IsCapturingInput() {
		t.Fatal("expected to be in filename entry mode")
	}

	beforeSave := time.Now()
	vp, cmd := vp.Update(enterKeyMsg)
	afterSave := time.Now()

	if vp.IsCapturingInput() {
		t.Error("expected IsCapturingInput to be false after enter")
	}
	if cmd == nil {
		t.Fatal("expected saveToFile command to be returned")
	}

	// view should show "Saving..."
	view := vp.View()
	if !strings.Contains(view, "Saving...") {
		t.Error("expected view to show 'Saving...' status")
	}

	msg := cmd()
	savedMsg, ok := msg.(fileSavedMsg)
	if !ok {
		t.Fatalf("expected fileSavedMsg, got %T", msg)
	}
	if savedMsg.err != nil {
		t.Fatalf("unexpected save error: %v", savedMsg.err)
	}

	filename := filepath.Base(savedMsg.filename)
	if !strings.HasSuffix(filename, ".txt") {
		t.Errorf("expected .txt extension, got %s", filename)
	}

	// verify file exists and has correct content
	content, err := os.ReadFile(savedMsg.filename)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}
	expectedContent := "content line 1\ncontent line 2\n"
	if string(content) != expectedContent {
		t.Errorf("expected content %q, got %q", expectedContent, string(content))
	}

	// verify file is in the correct directory
	if filepath.Dir(savedMsg.filename) != tmpDir {
		t.Errorf("expected file in %s, got %s", tmpDir, filepath.Dir(savedMsg.filename))
	}

	// verify timestamp is reasonable (within test execution window)
	timestampPart := strings.TrimSuffix(filename, ".txt")
	fileTime, err := time.ParseInLocation("20060102-150405", timestampPart, time.Local)
	if err != nil {
		t.Errorf("filename %s doesn't match timestamp format: %v", filename, err)
	} else {
		if fileTime.Before(beforeSave.Add(-2*time.Second)) || fileTime.After(afterSave.Add(2*time.Second)) {
			t.Errorf("timestamp %v not within expected range [%v, %v]", fileTime, beforeSave, afterSave)
		}
	}
}

func TestFileSaving_EnterWithCustomFilename(t *testing.T) {
	vp, tmpDir := newSaveTestViewport(t)
	setSaveTestContent(vp, []string{"test content"})

	vp, _ = vp.Update(saveKeyMsg)

	// type custom filename
	for _, r := range "myfile" {
		vp, _ = vp.Update(internal.MakeKeyMsg(r))
	}

	_, cmd := vp.Update(enterKeyMsg)

	if cmd == nil {
		t.Fatal("expected saveToFile command")
	}

	msg := cmd()
	savedMsg, ok := msg.(fileSavedMsg)
	if !ok {
		t.Fatalf("expected fileSavedMsg, got %T", msg)
	}
	if savedMsg.err != nil {
		t.Fatalf("unexpected save error: %v", savedMsg.err)
	}

	expectedPath := filepath.Join(tmpDir, "myfile.txt")
	if savedMsg.filename != expectedPath {
		t.Errorf("expected filename %s, got %s", expectedPath, savedMsg.filename)
	}

	// verify file exists
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", expectedPath)
	}
}

func TestFileSaving_CustomFilenameWithExtension(t *testing.T) {
	vp, tmpDir := newSaveTestViewport(t)
	setSaveTestContent(vp, []string{"test"})

	vp, _ = vp.Update(saveKeyMsg)

	// type filename with .txt extension already
	for _, r := range "already.txt" {
		vp, _ = vp.Update(internal.MakeKeyMsg(r))
	}

	_, cmd := vp.Update(enterKeyMsg)
	msg := cmd()
	savedMsg := msg.(fileSavedMsg)

	// should not double the extension
	expectedPath := filepath.Join(tmpDir, "already.txt")
	if savedMsg.filename != expectedPath {
		t.Errorf("expected filename %s, got %s", expectedPath, savedMsg.filename)
	}
}

func TestFileSaving_ContentStripsAnsiCodes(t *testing.T) {
	vp, _ := newSaveTestViewport(t)

	// set content with ANSI styling
	redStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000"))
	styledLine := redStyle.Render("styled text")
	objects := []saveTestObject{
		{item: item.NewItem(styledLine)},
		{item: item.NewItem("plain text")},
	}
	vp.SetObjects(objects)

	vp, _ = vp.Update(saveKeyMsg)
	_, cmd := vp.Update(enterKeyMsg)

	msg := cmd()
	savedMsg := msg.(fileSavedMsg)

	content, err := os.ReadFile(savedMsg.filename)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	// content should not contain ANSI escape codes
	if strings.Contains(string(content), "\x1b[") {
		t.Error("saved content should not contain ANSI escape codes")
	}

	expectedContent := "styled text\nplain text\n"
	if string(content) != expectedContent {
		t.Errorf("expected %q, got %q", expectedContent, string(content))
	}
}

func TestFileSaving_SuccessMessageShownAfterSave(t *testing.T) {
	tmpDir := t.TempDir()
	vp := New[saveTestObject](200, 24, // wide viewport to avoid truncation
		WithFileSaving[saveTestObject](tmpDir, saveKey),
	)
	setSaveTestContent(vp, []string{"test"})

	// go through complete save flow
	vp, _ = vp.Update(saveKeyMsg)
	vp, cmd := vp.Update(enterKeyMsg)

	// execute save command
	msg := cmd()
	savedMsg := msg.(fileSavedMsg)

	// send the result message back to viewport
	vp, _ = vp.Update(savedMsg)

	// view should show success message with path
	view := vp.View()
	if !strings.Contains(view, "Saved to") {
		t.Error("expected view to show 'Saved to' message")
	}
	if !strings.Contains(view, tmpDir) {
		t.Errorf("expected view to contain save directory %s", tmpDir)
	}
}

func TestFileSaving_ErrorMessageShownOnFailure(t *testing.T) {
	vp, _ := newSaveTestViewport(t)
	setSaveTestContent(vp, []string{"test"})

	// go through save flow
	vp, _ = vp.Update(saveKeyMsg)
	vp, _ = vp.Update(enterKeyMsg)

	// simulate error response
	vp, _ = vp.Update(fileSavedMsg{err: os.ErrPermission})

	// view should show error message
	view := vp.View()
	if !strings.Contains(view, "failed") && !strings.Contains(view, "Save failed") {
		t.Errorf("expected view to show error message, got: %s", view)
	}
}

func TestFileSaving_IgnoresSaveKeyWhenAlreadyCapturingInput(t *testing.T) {
	vp, _ := newSaveTestViewport(t)
	setSaveTestContent(vp, []string{"test"})

	// enter filename mode
	vp, _ = vp.Update(saveKeyMsg)
	if !vp.IsCapturingInput() {
		t.Fatal("expected to be capturing input")
	}

	// type something
	vp, _ = vp.Update(internal.MakeKeyMsg('a'))

	// press save key again - should be ignored, typed text preserved
	vp, cmd := vp.Update(saveKeyMsg)

	// should still be capturing input
	if !vp.IsCapturingInput() {
		t.Error("should still be capturing input")
	}
	if cmd != nil {
		t.Error("expected no command when ignoring duplicate save key")
	}

	// verify we can still complete the save with the typed filename
	_, cmd = vp.Update(enterKeyMsg)
	if cmd == nil {
		t.Fatal("expected save command")
	}
	msg := cmd()
	savedMsg := msg.(fileSavedMsg)
	if !strings.Contains(savedMsg.filename, "a.txt") {
		t.Errorf("expected filename to contain 'a.txt', got %s", savedMsg.filename)
	}
}

func TestFileSaving_TextInputReceivesKeyMessages(t *testing.T) {
	vp, tmpDir := newSaveTestViewport(t)
	setSaveTestContent(vp, []string{"test"})

	vp, _ = vp.Update(saveKeyMsg)

	// type some characters
	vp, _ = vp.Update(internal.MakeKeyMsg('a'))
	vp, _ = vp.Update(internal.MakeKeyMsg('b'))
	vp, _ = vp.Update(internal.MakeKeyMsg('c'))

	// verify by completing the save and checking filename
	_, cmd := vp.Update(enterKeyMsg)
	msg := cmd()
	savedMsg := msg.(fileSavedMsg)

	expectedPath := filepath.Join(tmpDir, "abc.txt")
	if savedMsg.filename != expectedPath {
		t.Errorf("expected filename %s, got %s", expectedPath, savedMsg.filename)
	}
}

func TestFileSaving_NoSaveDirConfigured(t *testing.T) {
	// viewport without file saving configured
	vp := New[saveTestObject](80, 24)
	setSaveTestContent(vp, []string{"test"})

	vp, cmd := vp.Update(saveKeyMsg)

	if vp.IsCapturingInput() {
		t.Error("should not enter filename mode when saveDir not configured")
	}
	if cmd != nil {
		t.Error("expected no command when saveDir not configured")
	}
}

func TestFileSaving_IsCapturingInputReturnsFalse_Initially(t *testing.T) {
	vp, _ := newSaveTestViewport(t)

	if vp.IsCapturingInput() {
		t.Error("expected IsCapturingInput to return false initially")
	}
}

func TestFileSaving_IsCapturingInputReturnsFalse_AfterSaveComplete(t *testing.T) {
	vp, _ := newSaveTestViewport(t)
	setSaveTestContent(vp, []string{"test"})

	// complete a save
	vp, _ = vp.Update(saveKeyMsg)
	vp, cmd := vp.Update(enterKeyMsg)
	msg := cmd()
	vp, _ = vp.Update(msg)

	// should not be capturing input while showing result
	if vp.IsCapturingInput() {
		t.Error("expected IsCapturingInput to return false when showing result")
	}
}

func TestFileSaving_NavigationKeysIgnoredDuringFilenameEntry(t *testing.T) {
	vp, tmpDir := newSaveTestViewport(t)
	vp.SetSelectionEnabled(true)
	setSaveTestContent(vp, []string{"line1", "line2", "line3", "line4", "line5"})

	vp, _ = vp.Update(saveKeyMsg)

	// try navigation keys - these should be typed into filename, not navigate
	vp, _ = vp.Update(internal.MakeKeyMsg('j')) // down
	vp, _ = vp.Update(internal.MakeKeyMsg('k')) // up
	vp, _ = vp.Update(internal.MakeKeyMsg('g')) // top
	vp, _ = vp.Update(internal.MakeKeyMsg('G')) // bottom

	// filename should be jkgG.txt
	_, cmd := vp.Update(enterKeyMsg)
	msg := cmd()
	savedMsg := msg.(fileSavedMsg)

	expectedPath := filepath.Join(tmpDir, "jkgG.txt")
	if savedMsg.filename != expectedPath {
		t.Errorf("expected filename %s, got %s", expectedPath, savedMsg.filename)
	}
}

func TestFileSaving_CreatesDirIfNotExists(t *testing.T) {
	tmpDir := t.TempDir()
	nestedDir := filepath.Join(tmpDir, "nested", "save", "dir")

	vp := New[saveTestObject](80, 24,
		WithFileSaving[saveTestObject](nestedDir, saveKey),
	)
	setSaveTestContent(vp, []string{"test content"})

	vp, _ = vp.Update(saveKeyMsg)
	_, cmd := vp.Update(enterKeyMsg)

	msg := cmd()
	savedMsg := msg.(fileSavedMsg)

	if savedMsg.err != nil {
		t.Fatalf("save failed: %v", savedMsg.err)
	}

	// verify directory was created
	if _, err := os.Stat(nestedDir); os.IsNotExist(err) {
		t.Errorf("expected directory %s to be created", nestedDir)
	}

	// verify file exists
	if _, err := os.Stat(savedMsg.filename); os.IsNotExist(err) {
		t.Errorf("expected file %s to exist", savedMsg.filename)
	}
}
