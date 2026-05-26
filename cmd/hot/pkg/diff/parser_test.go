package diff

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/git"
)

// rawLine generates a raw diff line with proper tab character
func rawLine(oldMode, newMode int, oldOID, newOID, status, path string) string {
	return fmt.Sprintf(":%06o %06o %s %s %s\t%s\n", oldMode, newMode, oldOID, newOID, status, path)
}

// rawLineRename generates a raw diff line for rename/copy with from and to paths
func rawLineRename(oldMode, newMode int, oldOID, newOID, status, fromPath, toPath string) string {
	return fmt.Sprintf(":%06o %06o %s %s %s\t%s\t%s\n", oldMode, newMode, oldOID, newOID, status, fromPath, toPath)
}

var sha1ZeroOID = "0000000000000000000000000000000000000000"

func TestParserBasic(t *testing.T) {
	input := rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "main.go") +
		"diff --git a/main.go b/main.go\n" +
		"index abcdef12..12345678 100644\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" package main\n" +
		"+import \"fmt\"\n" +
		" func main() {}\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	count := 0
	for parser.Parse() {
		patch := parser.Patch()
		count++
		t.Logf("Patch %d: status=%c, from=%v, to=%v, hunks=%d, binary=%v",
			count, patch.Status, patch.From, patch.To, len(patch.Hunks), patch.IsBinary)
	}

	if err := parser.Err(); err != nil {
		t.Fatalf("parser error: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 patch, got %d", count)
	}
}

func TestParserModify(t *testing.T) {
	input := rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "main.go") +
		"diff --git a/main.go b/main.go\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1,3 +1,4 @@\n" +
		" package main\n" +
		"+import \"fmt\"\n" +
		" func main() {}\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	if !parser.Parse() {
		if err := parser.Err(); err != nil {
			t.Fatalf("expected to parse one patch, error: %v", err)
		}
		t.Fatal("expected to parse one patch")
	}

	patch := parser.Patch()
	if patch.Status != 'M' {
		t.Errorf("expected status M, got %c", patch.Status)
	}

	if patch.From == nil || patch.From.Name != "main.go" {
		t.Errorf("expected from file 'main.go', got %v", patch.From)
	}

	if patch.To == nil || patch.To.Name != "main.go" {
		t.Errorf("expected to file 'main.go', got %v", patch.To)
	}

	if len(patch.Hunks) == 0 {
		t.Error("expected hunks to be parsed")
	}
}

func TestParserAdd(t *testing.T) {
	input := rawLine(0, 0100644, sha1ZeroOID, "1234567890abcdef1234567890abcdef12345678", "A", "new.go") +
		"diff --git a/new.go b/new.go\n" +
		"--- /dev/null\n" +
		"+++ b/new.go\n" +
		"@@ -0,0 +1,2 @@\n" +
		"+package main\n" +
		"+func newFunc() {}\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	if !parser.Parse() {
		if err := parser.Err(); err != nil {
			t.Fatalf("expected to parse one patch, error: %v", err)
		}
		t.Fatal("expected to parse one patch")
	}

	patch := parser.Patch()
	if patch.Status != 'A' {
		t.Errorf("expected status A, got %c", patch.Status)
	}

	if patch.From != nil {
		t.Errorf("expected from file to be nil for new file, got %v", patch.From)
	}

	if patch.To == nil || patch.To.Name != "new.go" {
		t.Errorf("expected to file 'new.go', got %v", patch.To)
	}
}

func TestParserDelete(t *testing.T) {
	input := rawLine(0100644, 0, "abcdef1234567890abcdef1234567890abcdef12", sha1ZeroOID, "D", "old.go") +
		"diff --git a/old.go b/old.go\n" +
		"--- a/old.go\n" +
		"+++ /dev/null\n" +
		"@@ -1,2 +0,0 @@\n" +
		"-package main\n" +
		"-func oldFunc() {}\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	if !parser.Parse() {
		if err := parser.Err(); err != nil {
			t.Fatalf("expected to parse one patch, error: %v", err)
		}
		t.Fatal("expected to parse one patch")
	}

	patch := parser.Patch()
	if patch.Status != 'D' {
		t.Errorf("expected status D, got %c", patch.Status)
	}

	if patch.From == nil || patch.From.Name != "old.go" {
		t.Errorf("expected from file 'old.go', got %v", patch.From)
	}

	if patch.To != nil {
		t.Errorf("expected to file to be nil for deleted file, got %v", patch.To)
	}
}

func TestParserBinary(t *testing.T) {
	input := rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "image.png") +
		"diff --git a/image.png b/image.png\n" +
		"Binary files a/image.png and b/image.png differ\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	if !parser.Parse() {
		if err := parser.Err(); err != nil {
			t.Fatalf("expected to parse one patch, error: %v", err)
		}
		t.Fatal("expected to parse one patch")
	}

	patch := parser.Patch()
	if !patch.IsBinary {
		t.Error("expected IsBinary to be true")
	}
}

func TestParserLimits(t *testing.T) {
	input := rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "file1.go") +
		"diff --git a/file1.go b/file1.go\n" +
		"--- a/file1.go\n" +
		"+++ b/file1.go\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n" +
		rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "file2.go") +
		"diff --git a/file2.go b/file2.go\n" +
		"--- a/file2.go\n" +
		"+++ b/file2.go\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n"

	limits := Limits{
		EnforceLimits: true,
		MaxFiles:      1,
	}
	parser := NewParser(git.HashSHA1, strings.NewReader(input), limits)

	count := 0
	for parser.Parse() {
		count++
	}

	if count > 2 {
		t.Errorf("expected at most 2 patches with limit, got %d", count)
	}
}

func TestParseHunks(t *testing.T) {
	lines := [][]byte{
		[]byte("--- a/main.go\n"),
		[]byte("+++ b/main.go\n"),
		[]byte("@@ -1,5 +1,6 @@\n"),
		[]byte(" package main\n"),
		[]byte("\n"),
		[]byte("+import \"fmt\"\n"),
		[]byte(" func main() {\n"),
		[]byte("-	println(\"hello\")\n"),
		[]byte("+	fmt.Println(\"hello world\")\n"),
		[]byte(" }\n"),
	}

	hunks, err := parseHunks(lines)
	if err != nil {
		t.Fatalf("parseHunks error: %v", err)
	}

	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}

	hunk := hunks[0]
	if hunk.FromLine != 1 {
		t.Errorf("expected FromLine=1, got %d", hunk.FromLine)
	}

	if hunk.ToLine != 1 {
		t.Errorf("expected ToLine=1, got %d", hunk.ToLine)
	}

	if len(hunk.Lines) == 0 {
		t.Fatal("expected hunk to have lines")
	}

	var added, removed int
	for _, line := range hunk.Lines {
		switch line.Kind {
		case 1: // Insert
			added++
		case -1: // Delete
			removed++
		}
	}

	if added != 2 {
		t.Errorf("expected 2 added lines, got %d", added)
	}

	if removed != 1 {
		t.Errorf("expected 1 removed line, got %d", removed)
	}
}

func TestParseHunksWithSection(t *testing.T) {
	lines := [][]byte{
		[]byte("@@ -1,3 +1,4 @@ function main() {\n"),
		[]byte(" package main\n"),
		[]byte("+import \"fmt\"\n"),
		[]byte(" func main() {}\n"),
	}

	hunks, err := parseHunks(lines)
	if err != nil {
		t.Fatalf("parseHunks error: %v", err)
	}

	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}

	hunk := hunks[0]
	if hunk.Section != "function main() {" {
		t.Errorf("expected Section='function main() {', got %q", hunk.Section)
	}

	if hunk.FromLine != 1 {
		t.Errorf("expected FromLine=1, got %d", hunk.FromLine)
	}

	if hunk.ToLine != 1 {
		t.Errorf("expected ToLine=1, got %d", hunk.ToLine)
	}
}

func TestParseHunksWithEmptySection(t *testing.T) {
	lines := [][]byte{
		[]byte("@@ -1,3 +1,4 @@\n"),
		[]byte(" package main\n"),
		[]byte("+import \"fmt\"\n"),
		[]byte(" func main() {}\n"),
	}

	hunks, err := parseHunks(lines)
	if err != nil {
		t.Fatalf("parseHunks error: %v", err)
	}

	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}

	hunk := hunks[0]
	if hunk.Section != "" {
		t.Errorf("expected empty Section, got %q", hunk.Section)
	}
}

func TestUnescape(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"simple.txt", "simple.txt"},
		{"file\\040with\\040spaces.txt", "file with spaces.txt"},
		{"file\\twith\\ttabs.txt", "file\twith\ttabs.txt"},
		{"file\\nwith\\nnewline.txt", "file\nwith\nnewline.txt"},
		{"file\\\"quotes\\\".txt", "file\"quotes\".txt"},
		{"file\\\\backslash.txt", "file\\backslash.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := unescape([]byte(tt.input))
			if string(result) != tt.expect {
				t.Errorf("unescape(%q) = %q, want %q", tt.input, result, tt.expect)
			}
		})
	}
}

func TestLimitsEnforceUpperBound(t *testing.T) {
	limits := Limits{
		MaxFiles:      10000,
		MaxLines:      500000,
		MaxBytes:      100 * 1024 * 1024,
		SafeMaxFiles:  1000,
		SafeMaxLines:  50000,
		SafeMaxBytes:  10 * 1024 * 1024,
		MaxPatchBytes: 1024 * 1024,
	}
	limits.enforceUpperBound()

	if limits.MaxFiles > maxFilesUpperBound {
		t.Errorf("MaxFiles should be <= %d, got %d", maxFilesUpperBound, limits.MaxFiles)
	}
}

// TestParserConsecutiveEmptyPatches tests consecutive files with mode changes only (no diff content)
func TestParserConsecutiveEmptyPatches(t *testing.T) {
	// Two files with only mode changes - no actual diff content
	input := rawLine(0100644, 0100755, "abcdef1234567890abcdef1234567890abcdef12", "abcdef1234567890abcdef1234567890abcdef12", "M", "script.sh") +
		rawLine(0100644, 0100755, "1234567890abcdef1234567890abcdef12345678", "1234567890abcdef1234567890abcdef12345678", "M", "tool.sh")

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	count := 0
	for parser.Parse() {
		patch := parser.Patch()
		count++
		path := "<nil>"
		if patch.From != nil {
			path = patch.From.Name
		} else if patch.To != nil {
			path = patch.To.Name
		}
		t.Logf("Patch %d: status=%c, path=%s, binary=%v, hunks=%d",
			count, patch.Status, path, patch.IsBinary, len(patch.Hunks))

		// Mode-only changes should have no hunks
		if len(patch.Hunks) > 0 {
			t.Errorf("patch %d: expected no hunks for mode-only change, got %d", count, len(patch.Hunks))
		}
	}

	if err := parser.Err(); err != nil {
		t.Fatalf("parser error: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 patches for mode-only changes, got %d", count)
	}
}

// TestParserQuotedPaths tests handling of paths with special characters
func TestParserQuotedPaths(t *testing.T) {
	// Paths with spaces and special characters are quoted in git diff
	input := rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "file with spaces.go") +
		"diff --git \"a/file with spaces.go\" \"b/file with spaces.go\"\n" +
		"index abcdef12..12345678 100644\n" +
		"--- \"a/file with spaces.go\"\n" +
		"+++ \"b/file with spaces.go\"\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	if !parser.Parse() {
		if err := parser.Err(); err != nil {
			t.Fatalf("expected to parse one patch, error: %v", err)
		}
		t.Fatal("expected to parse one patch")
	}

	patch := parser.Patch()
	if patch.Status != 'M' {
		t.Errorf("expected status M, got %c", patch.Status)
	}

	// Path should be correctly extracted (unquoted)
	if patch.From == nil || patch.From.Name != "file with spaces.go" {
		t.Errorf("expected from file 'file with spaces.go', got %v", patch.From)
	}

	if patch.To == nil || patch.To.Name != "file with spaces.go" {
		t.Errorf("expected to file 'file with spaces.go', got %v", patch.To)
	}

	t.Logf("Quoted path parsed: %s", patch.To.Name)
}

// TestParserQuotedPathsWithEscapes tests handling of quoted paths with escape sequences
func TestParserQuotedPathsWithEscapes(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fromPath string
		toPath   string
	}{
		{
			name: "quoted path with spaces",
			input: rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "foo bar.go") +
				"diff --git \"a/foo bar.go\" \"b/foo bar.go\"\n" +
				"index abcdef12..12345678 100644\n" +
				"--- \"a/foo bar.go\"\n" +
				"+++ \"b/foo bar.go\"\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			fromPath: "foo bar.go",
			toPath:   "foo bar.go",
		},
		{
			name: "quoted path with octal escape",
			input: rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "foo bar.go") +
				"diff --git \"a/foo\\040bar.go\" \"b/foo\\040bar.go\"\n" +
				"index abcdef12..12345678 100644\n" +
				"--- \"a/foo\\040bar.go\"\n" +
				"+++ \"b/foo\\040bar.go\"\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			fromPath: "foo bar.go",
			toPath:   "foo bar.go",
		},
		{
			name: "quoted path with escaped quote",
			input: rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "foo\"bar.go") +
				"diff --git \"a/foo\\\"bar.go\" \"b/foo\\\"bar.go\"\n" +
				"index abcdef12..12345678 100644\n" +
				"--- \"a/foo\\\"bar.go\"\n" +
				"+++ \"b/foo\\\"bar.go\"\n" +
				"@@ -1 +1 @@\n" +
				"-old\n" +
				"+new\n",
			fromPath: "foo\"bar.go",
			toPath:   "foo\"bar.go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(git.HashSHA1, strings.NewReader(tt.input), Limits{})

			if !parser.Parse() {
				if err := parser.Err(); err != nil {
					t.Fatalf("expected to parse one patch, error: %v", err)
				}
				t.Fatal("expected to parse one patch")
			}

			patch := parser.Patch()
			if patch.From == nil || patch.From.Name != tt.fromPath {
				t.Errorf("expected from file %q, got %v", tt.fromPath, patch.From)
			}
			if patch.To == nil || patch.To.Name != tt.toPath {
				t.Errorf("expected to file %q, got %v", tt.toPath, patch.To)
			}
			t.Logf("Parsed quoted path with escapes: %s -> %s", patch.From.Name, patch.To.Name)
		})
	}
}

// TestParserRenameWithPatch tests rename operations with content changes
func TestParserRenameWithPatch(t *testing.T) {
	// Rename with content modification - R100 means 100% similarity
	input := rawLineRename(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "R100", "old.go", "new.go") +
		"diff --git a/old.go b/new.go\n" +
		"similarity index 100%\n" +
		"rename from old.go\n" +
		"rename to new.go\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	if !parser.Parse() {
		if err := parser.Err(); err != nil {
			t.Fatalf("expected to parse one patch, error: %v", err)
		}
		t.Fatal("expected to parse one patch")
	}

	patch := parser.Patch()
	if patch.Status != 'R' {
		t.Errorf("expected status R, got %c", patch.Status)
	}

	if patch.From == nil || patch.From.Name != "old.go" {
		t.Errorf("expected from file 'old.go', got %v", patch.From)
	}

	if patch.To == nil || patch.To.Name != "new.go" {
		t.Errorf("expected to file 'new.go', got %v", patch.To)
	}

	t.Logf("Rename: %s -> %s, similarity=100%%", patch.From.Name, patch.To.Name)
}

// TestParserCopyWithPatch tests copy operations
func TestParserCopyWithPatch(t *testing.T) {
	input := rawLineRename(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "C100", "original.go", "copy.go") +
		"diff --git a/original.go b/copy.go\n" +
		"similarity index 100%\n" +
		"copy from original.go\n" +
		"copy to copy.go\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	if !parser.Parse() {
		if err := parser.Err(); err != nil {
			t.Fatalf("expected to parse one patch, error: %v", err)
		}
		t.Fatal("expected to parse one patch")
	}

	patch := parser.Patch()
	if patch.Status != 'C' {
		t.Errorf("expected status C, got %c", patch.Status)
	}

	if patch.From == nil || patch.From.Name != "original.go" {
		t.Errorf("expected from file 'original.go', got %v", patch.From)
	}

	if patch.To == nil || patch.To.Name != "copy.go" {
		t.Errorf("expected to file 'copy.go', got %v", patch.To)
	}

	t.Logf("Copy: %s -> %s, similarity=100%%", patch.From.Name, patch.To.Name)
}

// TestParserNoNewlineAtEOF tests handling of files without newline at end
func TestParserNoNewlineAtEOF(t *testing.T) {
	input := rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "file.go") +
		"diff --git a/file.go b/file.go\n" +
		"--- a/file.go\n" +
		"+++ b/file.go\n" +
		"@@ -1,2 +1,2 @@\n" +
		" line1\n" +
		"-line2\n" +
		"\\ No newline at end of file\n" +
		"+line2new\n" +
		"\\ No newline at end of file\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	if !parser.Parse() {
		if err := parser.Err(); err != nil {
			t.Fatalf("expected to parse one patch, error: %v", err)
		}
		t.Fatal("expected to parse one patch")
	}

	patch := parser.Patch()
	if len(patch.Hunks) == 0 {
		t.Fatal("expected hunks to be parsed")
	}

	// The "No newline at end of file" marker should not create extra lines
	hunk := patch.Hunks[0]
	var deleteCount, insertCount int
	for _, line := range hunk.Lines {
		if line.Kind == -1 {
			deleteCount++
			continue
		}
		if line.Kind == 1 {
			insertCount++
		}
	}

	if deleteCount != 1 {
		t.Errorf("expected 1 deleted line, got %d", deleteCount)
	}

	if insertCount != 1 {
		t.Errorf("expected 1 inserted line, got %d", insertCount)
	}

	t.Logf("No-newline-at-EOF handled correctly: %d deletes, %d inserts", deleteCount, insertCount)
}

// TestParserTypeChange tests type-change (file to symlink, etc.) handling
func TestParserTypeChange(t *testing.T) {
	// Type change: regular file to symlink
	input := rawLine(0100644, 0120755, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "T", "link") +
		"diff --git a/link b/link\n" +
		"deleted file mode 100644\n" +
		"index abcdef12..12345678\n" +
		"--- a/link\n" +
		"+++ /dev/null\n" +
		"@@ -1 +0,0 @@\n" +
		"-content\n" +
		"diff --git a/link b/link\n" +
		"new file mode 120755\n" +
		"index 00000000..12345678\n" +
		"--- /dev/null\n" +
		"+++ b/link\n" +
		"@@ -0,0 +1 @@\n" +
		"+content\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	patches := make([]*Patch, 0)
	for parser.Parse() {
		patches = append(patches, parser.Patch())
	}

	if err := parser.Err(); err != nil {
		t.Fatalf("parser error: %v", err)
	}

	// Type change may produce multiple patches
	t.Logf("Type-change produced %d patches", len(patches))
	for i, p := range patches {
		t.Logf("  Patch %d: status=%c", i+1, p.Status)
	}
}

// TestParserEnforceLimitsZeroMeansUnlimited verifies that zero-value limits mean "no limit"
func TestParserEnforceLimitsZeroMeansUnlimited(t *testing.T) {
	input := rawLine(0100644, 0100644,
		"abcdef1234567890abcdef1234567890abcdef12",
		"1234567890abcdef1234567890abcdef12345678",
		"M", "main.go") +
		"diff --git a/main.go b/main.go\n" +
		"--- a/main.go\n" +
		"+++ b/main.go\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{
		EnforceLimits: true,
		// all max values left as zero - should mean "no limit"
	})

	if !parser.Parse() {
		t.Fatalf("expected first patch to parse, err=%v", parser.Err())
	}

	patch := parser.Patch()
	if patch.OverflowMarker {
		t.Fatalf("did not expect overflow marker with zero-value limits")
	}
	if patch.From == nil || patch.From.Name != "main.go" {
		t.Errorf("expected from file 'main.go', got %v", patch.From)
	}
}

// TestParserPatchObjectIsReused verifies that Patch() returns a reused object
// This test documents the API behavior that callers should not retain the pointer
func TestParserPatchObjectIsReused(t *testing.T) {
	// Note: raw lines must come BEFORE all patch content in git diff --raw --patch output
	input := rawLine(0100644, 0100644,
		"abcdef1234567890abcdef1234567890abcdef12",
		"1234567890abcdef1234567890abcdef12345678",
		"M", "file1.go") +
		rawLine(0100644, 0100644,
			"abcdef1234567890abcdef1234567890abcdef12",
			"1234567890abcdef1234567890abcdef12345678",
			"M", "file2.go") +
		"diff --git a/file1.go b/file1.go\n" +
		"--- a/file1.go\n" +
		"+++ b/file1.go\n" +
		"@@ -1 +1 @@\n-old\n+new\n" +
		"diff --git a/file2.go b/file2.go\n" +
		"--- a/file2.go\n" +
		"+++ b/file2.go\n" +
		"@@ -1 +1 @@\n-old\n+new\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	if !parser.Parse() {
		t.Fatalf("first parse failed: %v", parser.Err())
	}
	first := parser.Patch()
	if first.From == nil || first.From.Name != "file1.go" {
		t.Fatalf("expected first patch to be file1.go, got %+v", first.From)
	}

	if !parser.Parse() {
		t.Fatalf("second parse failed: %v", parser.Err())
	}

	// first has now been overwritten because Patch() is reused
	if first.From == nil || first.From.Name != "file2.go" {
		t.Fatalf("expected reused patch object to now point to file2.go, got %+v", first.From)
	}
}

func BenchmarkParser(b *testing.B) {
	var buf bytes.Buffer
	for i := range 100 {
		buf.WriteString(rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", fmt.Sprintf("file%d.go", i%10)))
		fmt.Fprintf(&buf, "diff --git a/file%d.go b/file%d.go\n", i%10, i%10)
		buf.WriteString("--- a/file.go\n+++ b/file.go\n@@ -1,3 +1,4 @@\n package main\n func main() {\n+println(\"test\")\n }\n")
	}

	input := buf.String()

	for b.Loop() {
		parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})
		for parser.Parse() {
		}
	}
}

// TestParserTypeChangeProducesTwoPatches verifies that a type-change ('T')
// is correctly split into two patches (delete + add) without reading the
// wrong diff header.
func TestParserTypeChangeProducesTwoPatches(t *testing.T) {
	input := rawLine(0100644, 0120000, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "T", "link") +
		"diff --git a/link b/link\n" +
		"deleted file mode 100644\n" +
		"index abcdef12..12345678\n" +
		"--- a/link\n" +
		"+++ /dev/null\n" +
		"@@ -1 +0,0 @@\n" +
		"-content\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	// First patch: the deletion part (status T, To=nil)
	if !parser.Parse() {
		t.Fatalf("expected first patch, err=%v", parser.Err())
	}
	p1 := parser.Patch()
	if p1.Status != 'T' {
		t.Errorf("first patch: expected status T, got %c", p1.Status)
	}
	if p1.From == nil {
		t.Fatal("first patch: expected From to be set")
	}
	if p1.To != nil {
		t.Error("first patch: expected To to be nil (deletion half)")
	}

	// Second patch: the synthetic addition (status A, From=nil)
	if !parser.Parse() {
		t.Fatalf("expected second patch, err=%v", parser.Err())
	}
	p2 := parser.Patch()
	if p2.Status != 'A' {
		t.Errorf("second patch: expected status A, got %c", p2.Status)
	}
	if p2.To == nil {
		t.Fatal("second patch: expected To to be set")
	}
	if p2.To.Name != "link" {
		t.Errorf("second patch: expected To.Name='link', got %q", p2.To.Name)
	}

	// No more patches
	if parser.Parse() {
		t.Error("expected no more patches")
	}
	if err := parser.Err(); err != nil {
		t.Fatalf("parser error: %v", err)
	}
}

// TestParserUnquotedPathWithSpaces tests parsing of unquoted paths that
// contain spaces (core.quotepath=false).
func TestParserUnquotedPathWithSpaces(t *testing.T) {
	input := rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "path with spaces.go") +
		"diff --git a/path with spaces.go b/path with spaces.go\n" +
		"--- a/path with spaces.go\n" +
		"+++ b/path with spaces.go\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})

	if !parser.Parse() {
		t.Fatalf("expected to parse one patch, err=%v", parser.Err())
	}

	patch := parser.Patch()
	if patch.From == nil || patch.From.Name != "path with spaces.go" {
		t.Errorf("expected from='path with spaces.go', got %v", patch.From)
	}
	if patch.To == nil || patch.To.Name != "path with spaces.go" {
		t.Errorf("expected to='path with spaces.go', got %v", patch.To)
	}
}

// TestParserOverflowPreservesFileInfo verifies that when enforce limits
// triggers overflow, the file metadata (From/To) is preserved.
func TestParserOverflowPreservesFileInfo(t *testing.T) {
	input := rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "file1.go") +
		"diff --git a/file1.go b/file1.go\n" +
		"--- a/file1.go\n" +
		"+++ b/file1.go\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n" +
		rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "file2.go") +
		"diff --git a/file2.go b/file2.go\n" +
		"--- a/file2.go\n" +
		"+++ b/file2.go\n" +
		"@@ -1 +1 @@\n" +
		"-old\n" +
		"+new\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{
		EnforceLimits: true,
		MaxFiles:      1,
	})

	var lastPatch *Patch
	count := 0
	for parser.Parse() {
		count++
		// Copy the patch data we care about before next Parse overwrites it
		p := *parser.Patch()
		lastPatch = &p
	}

	if err := parser.Err(); err != nil {
		t.Fatalf("parser error: %v", err)
	}

	// The second patch should have OverflowMarker=true but still have file info
	if lastPatch == nil {
		t.Fatal("expected at least one patch")
	}
	if lastPatch.OverflowMarker {
		// When overflow is triggered, From/To should still be available
		if lastPatch.From == nil && lastPatch.To == nil {
			t.Error("overflow patch should preserve From/To metadata")
		}
	}
}

// FuzzParser exercises the parser with arbitrary input to find panics,
// infinite loops, or other crashes.
func FuzzParser(f *testing.F) {
	// Seed corpus with representative inputs
	seeds := []string{
		// Basic modify
		rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "main.go") +
			"diff --git a/main.go b/main.go\n" +
			"--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n",
		// Add
		rawLine(0, 0100644, sha1ZeroOID, "1234567890abcdef1234567890abcdef12345678", "A", "new.go") +
			"diff --git a/new.go b/new.go\n--- /dev/null\n+++ b/new.go\n@@ -0,0 +1 @@\n+line\n",
		// Delete
		rawLine(0100644, 0, "abcdef1234567890abcdef1234567890abcdef12", sha1ZeroOID, "D", "old.go") +
			"diff --git a/old.go b/old.go\n--- a/old.go\n+++ /dev/null\n@@ -1 +0,0 @@\n-line\n",
		// Rename
		rawLineRename(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "R100", "a.go", "b.go") +
			"diff --git a/a.go b/b.go\nsimilarity index 100%\nrename from a.go\nrename to b.go\n",
		// Binary
		rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "img.png") +
			"diff --git a/img.png b/img.png\nBinary files a/img.png and b/img.png differ\n",
		// Type change
		rawLine(0100644, 0120000, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "T", "link") +
			"diff --git a/link b/link\n--- a/link\n+++ /dev/null\n@@ -1 +0,0 @@\n-x\n",
		// Empty input
		"",
		// Only raw lines, no patches
		rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "abcdef1234567890abcdef1234567890abcdef12", "M", "f.go"),
		// Quoted path
		rawLine(0100644, 0100644, "abcdef1234567890abcdef1234567890abcdef12", "1234567890abcdef1234567890abcdef12345678", "M", "a b.go") +
			"diff --git \"a/a b.go\" \"b/a b.go\"\n--- \"a/a b.go\"\n+++ \"b/a b.go\"\n@@ -1 +1 @@\n-x\n+y\n",
	}

	for _, s := range seeds {
		f.Add([]byte(s))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		// The parser must never panic regardless of input.
		parser := NewParser(git.HashSHA1, bytes.NewReader(data), Limits{
			EnforceLimits: true,
			MaxFiles:      100,
			MaxLines:      1000,
			MaxBytes:      1 << 20,
			MaxPatchBytes: 1 << 16,
		})

		for parser.Parse() {
			p := parser.Patch()
			// Access fields to trigger any nil-pointer panics
			_ = p.Status
			_ = p.OverflowMarker
			_ = p.Collapsed
			_ = p.TooLarge
			_ = p.PatchSize
			_ = p.LinesAdded
			_ = p.LinesRemoved
			if p.From != nil {
				_ = p.From.Name
				_ = p.From.Hash
			}
			if p.To != nil {
				_ = p.To.Name
				_ = p.To.Hash
			}
			if p.Patch != nil {
				for _, h := range p.Hunks {
					_ = h.FromLine
					_ = h.ToLine
					_ = h.Section
					for _, l := range h.Lines {
						_ = l.Kind
						_ = l.Content
					}
				}
			}
		}
		// Error is acceptable; panics are not.
		_ = parser.Err()
	})
}

// FuzzUnescape exercises the unescape function with arbitrary input.
func FuzzUnescape(f *testing.F) {
	f.Add([]byte("simple.txt"))
	f.Add([]byte("file\\040with\\040spaces.txt"))
	f.Add([]byte("file\\twith\\ttabs.txt"))
	f.Add([]byte("file\\\"quotes\\\".txt"))
	f.Add([]byte("file\\\\backslash.txt"))
	f.Add([]byte("\\377\\000\\177"))
	f.Add([]byte("\\"))
	f.Add([]byte("\\x"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Must never panic
		result := unescape(data)
		_ = result
	})
}

// FuzzParseHunkHeader exercises hunk header parsing with arbitrary input.
func FuzzParseHunkHeader(f *testing.F) {
	f.Add("@@ -1,3 +1,4 @@\n")
	f.Add("@@ -1,3 +1,4 @@ func main() {\n")
	f.Add("@@ -0,0 +1 @@\n")
	f.Add("@@ -1 +0,0 @@\n")
	f.Add("@@ -100,200 +150,300 @@ section text\n")
	f.Add("")
	f.Add("not a hunk header")
	f.Add("@@ garbage @@")

	f.Fuzz(func(t *testing.T, header string) {
		// Must never panic
		_, _, _, _, _, _ = parseHunkHeader(header)
	})
}

// ---------------------------------------------------------------------------
// Protocol tests: verify the correctness of the refactored parser against
// the expected output contract for every supported diff status.
// ---------------------------------------------------------------------------

// TestProtocolBinaryUsesIsBinary verifies that after removing the Patch.Binary
// field, binary detection is correctly reported via the embedded IsBinary field.
func TestProtocolBinaryUsesIsBinary(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		isBinary bool
	}{
		{
			name: "binary file",
			input: rawLine(0100644, 0100644,
				"abcdef1234567890abcdef1234567890abcdef12",
				"1234567890abcdef1234567890abcdef12345678", "M", "image.png") +
				"diff --git a/image.png b/image.png\n" +
				"Binary files a/image.png and b/image.png differ\n",
			isBinary: true,
		},
		{
			name: "text file",
			input: rawLine(0100644, 0100644,
				"abcdef1234567890abcdef1234567890abcdef12",
				"1234567890abcdef1234567890abcdef12345678", "M", "main.go") +
				"diff --git a/main.go b/main.go\n" +
				"--- a/main.go\n+++ b/main.go\n@@ -1 +1 @@\n-old\n+new\n",
			isBinary: false,
		},
		{
			name: "binary add",
			input: rawLine(0, 0100644,
				sha1ZeroOID,
				"1234567890abcdef1234567890abcdef12345678", "A", "data.bin") +
				"diff --git a/data.bin b/data.bin\n" +
				"Binary files /dev/null and b/data.bin differ\n",
			isBinary: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(git.HashSHA1, strings.NewReader(tt.input), Limits{})
			if !parser.Parse() {
				t.Fatalf("expected to parse one patch, err=%v", parser.Err())
			}
			patch := parser.Patch()
			if patch.IsBinary != tt.isBinary {
				t.Errorf("IsBinary = %v, want %v", patch.IsBinary, tt.isBinary)
			}
		})
	}
}

// TestProtocolEndToEnd is a comprehensive protocol test that verifies the
// parser produces the correct output for every supported diff status (A, D,
// M, R, C, T) and edge case (binary, empty patch, multi-hunk).
func TestProtocolEndToEnd(t *testing.T) {
	oid1 := "abcdef1234567890abcdef1234567890abcdef12"
	oid2 := "1234567890abcdef1234567890abcdef12345678"

	tests := []struct {
		name        string
		input       string
		wantPatches int
		checks      []func(t *testing.T, patches []Patch)
	}{
		{
			name: "modify with hunks",
			input: rawLine(0100644, 0100644, oid1, oid2, "M", "main.go") +
				"diff --git a/main.go b/main.go\n" +
				"--- a/main.go\n+++ b/main.go\n" +
				"@@ -1,3 +1,4 @@\n package main\n+import \"fmt\"\n func main() {}\n",
			wantPatches: 1,
			checks: []func(t *testing.T, patches []Patch){
				func(t *testing.T, ps []Patch) {
					p := ps[0]
					if p.Status != 'M' {
						t.Errorf("status = %c, want M", p.Status)
					}
					if p.From == nil || p.From.Name != "main.go" {
						t.Errorf("From = %v, want main.go", p.From)
					}
					if p.To == nil || p.To.Name != "main.go" {
						t.Errorf("To = %v, want main.go", p.To)
					}
					if len(p.Hunks) != 1 {
						t.Fatalf("hunks = %d, want 1", len(p.Hunks))
					}
					if p.Hunks[0].FromLine != 1 || p.Hunks[0].ToLine != 1 {
						t.Errorf("hunk range = -%d +%d, want -1 +1", p.Hunks[0].FromLine, p.Hunks[0].ToLine)
					}
					if p.LinesAdded != 1 {
						t.Errorf("LinesAdded = %d, want 1", p.LinesAdded)
					}
					if p.IsBinary {
						t.Error("IsBinary should be false for text file")
					}
				},
			},
		},
		{
			name: "add new file",
			input: rawLine(0, 0100644, sha1ZeroOID, oid2, "A", "new.go") +
				"diff --git a/new.go b/new.go\n" +
				"--- /dev/null\n+++ b/new.go\n@@ -0,0 +1,2 @@\n+line1\n+line2\n",
			wantPatches: 1,
			checks: []func(t *testing.T, patches []Patch){
				func(t *testing.T, ps []Patch) {
					p := ps[0]
					if p.Status != 'A' {
						t.Errorf("status = %c, want A", p.Status)
					}
					if p.From != nil {
						t.Errorf("From should be nil for add, got %v", p.From)
					}
					if p.To == nil || p.To.Name != "new.go" {
						t.Errorf("To = %v, want new.go", p.To)
					}
					if p.LinesAdded != 2 {
						t.Errorf("LinesAdded = %d, want 2", p.LinesAdded)
					}
				},
			},
		},
		{
			name: "delete file",
			input: rawLine(0100644, 0, oid1, sha1ZeroOID, "D", "old.go") +
				"diff --git a/old.go b/old.go\n" +
				"--- a/old.go\n+++ /dev/null\n@@ -1,2 +0,0 @@\n-line1\n-line2\n",
			wantPatches: 1,
			checks: []func(t *testing.T, patches []Patch){
				func(t *testing.T, ps []Patch) {
					p := ps[0]
					if p.Status != 'D' {
						t.Errorf("status = %c, want D", p.Status)
					}
					if p.From == nil || p.From.Name != "old.go" {
						t.Errorf("From = %v, want old.go", p.From)
					}
					if p.To != nil {
						t.Errorf("To should be nil for delete, got %v", p.To)
					}
					if p.LinesRemoved != 2 {
						t.Errorf("LinesRemoved = %d, want 2", p.LinesRemoved)
					}
				},
			},
		},
		{
			name: "rename",
			input: rawLineRename(0100644, 0100644, oid1, oid2, "R100", "old.go", "new.go") +
				"diff --git a/old.go b/new.go\n" +
				"similarity index 100%\nrename from old.go\nrename to new.go\n",
			wantPatches: 1,
			checks: []func(t *testing.T, patches []Patch){
				func(t *testing.T, ps []Patch) {
					p := ps[0]
					if p.Status != 'R' {
						t.Errorf("status = %c, want R", p.Status)
					}
					if p.From == nil || p.From.Name != "old.go" {
						t.Errorf("From = %v, want old.go", p.From)
					}
					if p.To == nil || p.To.Name != "new.go" {
						t.Errorf("To = %v, want new.go", p.To)
					}
					if len(p.Hunks) != 0 {
						t.Errorf("hunks = %d, want 0 for pure rename", len(p.Hunks))
					}
				},
			},
		},
		{
			name: "copy",
			input: rawLineRename(0100644, 0100644, oid1, oid2, "C100", "src.go", "dst.go") +
				"diff --git a/src.go b/dst.go\n" +
				"similarity index 100%\ncopy from src.go\ncopy to dst.go\n",
			wantPatches: 1,
			checks: []func(t *testing.T, patches []Patch){
				func(t *testing.T, ps []Patch) {
					p := ps[0]
					if p.Status != 'C' {
						t.Errorf("status = %c, want C", p.Status)
					}
					if p.From == nil || p.From.Name != "src.go" {
						t.Errorf("From = %v, want src.go", p.From)
					}
					if p.To == nil || p.To.Name != "dst.go" {
						t.Errorf("To = %v, want dst.go", p.To)
					}
				},
			},
		},
		{
			name: "binary file",
			input: rawLine(0100644, 0100644, oid1, oid2, "M", "img.png") +
				"diff --git a/img.png b/img.png\n" +
				"Binary files a/img.png and b/img.png differ\n",
			wantPatches: 1,
			checks: []func(t *testing.T, patches []Patch){
				func(t *testing.T, ps []Patch) {
					p := ps[0]
					if !p.IsBinary {
						t.Error("IsBinary should be true")
					}
					if len(p.Hunks) != 0 {
						t.Errorf("hunks = %d, want 0 for binary", len(p.Hunks))
					}
				},
			},
		},
		{
			name: "type change splits into two patches",
			input: rawLine(0100644, 0120000, oid1, oid2, "T", "link") +
				"diff --git a/link b/link\n" +
				"deleted file mode 100644\n" +
				"--- a/link\n+++ /dev/null\n@@ -1 +0,0 @@\n-content\n",
			wantPatches: 2,
			checks: []func(t *testing.T, patches []Patch){
				func(t *testing.T, ps []Patch) {
					// First: deletion half
					if ps[0].Status != 'T' {
						t.Errorf("patch[0] status = %c, want T", ps[0].Status)
					}
					if ps[0].From == nil {
						t.Error("patch[0] From should be set")
					}
					if ps[0].To != nil {
						t.Error("patch[0] To should be nil (deletion half)")
					}
					// Second: synthetic addition
					if ps[1].Status != 'A' {
						t.Errorf("patch[1] status = %c, want A", ps[1].Status)
					}
					if ps[1].To == nil || ps[1].To.Name != "link" {
						t.Errorf("patch[1] To = %v, want link", ps[1].To)
					}
				},
			},
		},
		{
			name: "multi-hunk diff",
			input: rawLine(0100644, 0100644, oid1, oid2, "M", "big.go") +
				"diff --git a/big.go b/big.go\n" +
				"--- a/big.go\n+++ b/big.go\n" +
				"@@ -1,3 +1,4 @@ package main\n context1\n+add1\n context2\n" +
				"@@ -10,3 +11,4 @@ func foo() {\n context3\n+add2\n context4\n",
			wantPatches: 1,
			checks: []func(t *testing.T, patches []Patch){
				func(t *testing.T, ps []Patch) {
					p := ps[0]
					if len(p.Hunks) != 2 {
						t.Fatalf("hunks = %d, want 2", len(p.Hunks))
					}
					if p.Hunks[0].FromLine != 1 {
						t.Errorf("hunk[0].FromLine = %d, want 1", p.Hunks[0].FromLine)
					}
					if p.Hunks[0].Section != "package main" {
						t.Errorf("hunk[0].Section = %q, want 'package main'", p.Hunks[0].Section)
					}
					if p.Hunks[1].FromLine != 10 {
						t.Errorf("hunk[1].FromLine = %d, want 10", p.Hunks[1].FromLine)
					}
					if p.Hunks[1].Section != "func foo() {" {
						t.Errorf("hunk[1].Section = %q, want 'func foo() {'", p.Hunks[1].Section)
					}
					if p.LinesAdded != 2 {
						t.Errorf("LinesAdded = %d, want 2", p.LinesAdded)
					}
				},
			},
		},
		{
			name: "multiple files in one stream",
			input: rawLine(0100644, 0100644, oid1, oid2, "M", "a.go") +
				rawLine(0100644, 0100644, oid1, oid2, "M", "b.go") +
				"diff --git a/a.go b/a.go\n--- a/a.go\n+++ b/a.go\n@@ -1 +1 @@\n-old\n+new\n" +
				"diff --git a/b.go b/b.go\n--- a/b.go\n+++ b/b.go\n@@ -1 +1 @@\n-x\n+y\n",
			wantPatches: 2,
			checks: []func(t *testing.T, patches []Patch){
				func(t *testing.T, ps []Patch) {
					if ps[0].From.Name != "a.go" {
						t.Errorf("patch[0] From = %q, want a.go", ps[0].From.Name)
					}
					if ps[1].From.Name != "b.go" {
						t.Errorf("patch[1] From = %q, want b.go", ps[1].From.Name)
					}
				},
			},
		},
		{
			name:        "empty patch (mode-only change)",
			input:       rawLine(0100644, 0100755, oid1, oid1, "M", "script.sh"),
			wantPatches: 1,
			checks: []func(t *testing.T, patches []Patch){
				func(t *testing.T, ps []Patch) {
					if len(ps[0].Hunks) != 0 {
						t.Errorf("hunks = %d, want 0 for mode-only change", len(ps[0].Hunks))
					}
					if ps[0].IsBinary {
						t.Error("mode-only change should not be binary")
					}
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parser := NewParser(git.HashSHA1, strings.NewReader(tt.input), Limits{})

			var patches []Patch
			for parser.Parse() {
				// Copy the patch since it's reused
				p := *parser.Patch()
				patches = append(patches, p)
			}

			if err := parser.Err(); err != nil {
				t.Fatalf("parser error: %v", err)
			}

			if len(patches) != tt.wantPatches {
				t.Fatalf("got %d patches, want %d", len(patches), tt.wantPatches)
			}

			for _, check := range tt.checks {
				check(t, patches)
			}
		})
	}
}

// TestProtocolHunkLineContent verifies that the []string → [][]byte
// refactoring in parseHunks produces identical Line.Content values.
func TestProtocolHunkLineContent(t *testing.T) {
	input := rawLine(0100644, 0100644,
		"abcdef1234567890abcdef1234567890abcdef12",
		"1234567890abcdef1234567890abcdef12345678", "M", "main.go") +
		"diff --git a/main.go b/main.go\n" +
		"--- a/main.go\n+++ b/main.go\n" +
		"@@ -1,5 +1,6 @@ package main\n" +
		" import \"os\"\n" +
		"+import \"fmt\"\n" +
		" \n" +
		"-func old() {}\n" +
		"+func new() { fmt.Println(os.Args) }\n" +
		" // end\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})
	if !parser.Parse() {
		t.Fatalf("parse failed: %v", parser.Err())
	}

	patch := parser.Patch()
	if len(patch.Hunks) != 1 {
		t.Fatalf("hunks = %d, want 1", len(patch.Hunks))
	}

	hunk := patch.Hunks[0]
	// Expected lines (content after the +/-/space prefix):
	wantLines := []struct {
		kind    int // 0=equal, 1=insert, -1=delete
		content string
	}{
		{0, "import \"os\"\n"},
		{1, "import \"fmt\"\n"},
		{0, "\n"},
		{-1, "func old() {}\n"},
		{1, "func new() { fmt.Println(os.Args) }\n"},
		{0, "// end\n"},
	}

	if len(hunk.Lines) != len(wantLines) {
		t.Fatalf("lines = %d, want %d", len(hunk.Lines), len(wantLines))
	}

	for i, want := range wantLines {
		got := hunk.Lines[i]
		if int(got.Kind) != want.kind {
			t.Errorf("line[%d].Kind = %d, want %d", i, got.Kind, want.kind)
		}
		if got.Content != want.content {
			t.Errorf("line[%d].Content = %q, want %q", i, got.Content, want.content)
		}
	}
}

// TestProtocolPatchStats verifies LinesAdded, LinesRemoved, and PatchSize
// are correctly computed after the refactoring.
func TestProtocolPatchStats(t *testing.T) {
	input := rawLine(0100644, 0100644,
		"abcdef1234567890abcdef1234567890abcdef12",
		"1234567890abcdef1234567890abcdef12345678", "M", "stats.go") +
		"diff --git a/stats.go b/stats.go\n" +
		"--- a/stats.go\n+++ b/stats.go\n" +
		"@@ -1,4 +1,5 @@\n" +
		" line1\n" +
		"-line2\n" +
		"-line3\n" +
		"+line2new\n" +
		"+line3new\n" +
		"+line4new\n" +
		" line5\n"

	parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})
	if !parser.Parse() {
		t.Fatalf("parse failed: %v", parser.Err())
	}

	patch := parser.Patch()
	if patch.LinesAdded != 3 {
		t.Errorf("LinesAdded = %d, want 3", patch.LinesAdded)
	}
	if patch.LinesRemoved != 2 {
		t.Errorf("LinesRemoved = %d, want 2", patch.LinesRemoved)
	}
	if patch.PatchSize <= 0 {
		t.Errorf("PatchSize = %d, want > 0", patch.PatchSize)
	}
}

// ---------------------------------------------------------------------------
// Benchmark tests: measure parser performance to validate that the
// []string → [][]byte refactoring and Binary field removal do not regress.
// ---------------------------------------------------------------------------

// BenchmarkParserSmall benchmarks parsing a small diff (1 file, 1 hunk).
func BenchmarkParserSmall(b *testing.B) {
	input := rawLine(0100644, 0100644,
		"abcdef1234567890abcdef1234567890abcdef12",
		"1234567890abcdef1234567890abcdef12345678", "M", "main.go") +
		"diff --git a/main.go b/main.go\n" +
		"--- a/main.go\n+++ b/main.go\n" +
		"@@ -1,3 +1,4 @@\n package main\n+import \"fmt\"\n func main() {}\n"

	b.ReportAllocs()
	for b.Loop() {
		parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})
		for parser.Parse() {
			_ = parser.Patch()
		}
	}
}

// BenchmarkParserLargeHunks benchmarks parsing a single file with many hunk lines.
func BenchmarkParserLargeHunks(b *testing.B) {
	var buf bytes.Buffer
	buf.WriteString(rawLine(0100644, 0100644,
		"abcdef1234567890abcdef1234567890abcdef12",
		"1234567890abcdef1234567890abcdef12345678", "M", "big.go"))
	buf.WriteString("diff --git a/big.go b/big.go\n--- a/big.go\n+++ b/big.go\n")
	buf.WriteString("@@ -1,500 +1,600 @@\n")
	for i := range 500 {
		fmt.Fprintf(&buf, "-old line %d\n", i)
	}
	for i := range 600 {
		fmt.Fprintf(&buf, "+new line %d\n", i)
	}
	input := buf.String()

	b.ReportAllocs()
	for b.Loop() {
		parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})
		for parser.Parse() {
			_ = parser.Patch()
		}
	}
}

// BenchmarkParserManyFiles benchmarks parsing many files (100 files).
func BenchmarkParserManyFiles(b *testing.B) {
	var buf bytes.Buffer
	for i := range 100 {
		buf.WriteString(rawLine(0100644, 0100644,
			"abcdef1234567890abcdef1234567890abcdef12",
			"1234567890abcdef1234567890abcdef12345678", "M",
			fmt.Sprintf("pkg/file%d.go", i)))
	}
	for i := range 100 {
		fmt.Fprintf(&buf, "diff --git a/pkg/file%d.go b/pkg/file%d.go\n", i, i)
		fmt.Fprintf(&buf, "--- a/pkg/file%d.go\n+++ b/pkg/file%d.go\n", i, i)
		buf.WriteString("@@ -1,3 +1,4 @@\n package main\n+// change\n func init() {}\n")
	}
	input := buf.String()

	b.ReportAllocs()
	for b.Loop() {
		parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})
		for parser.Parse() {
			_ = parser.Patch()
		}
	}
}

// BenchmarkParserBinary benchmarks parsing binary file diffs.
func BenchmarkParserBinary(b *testing.B) {
	var buf bytes.Buffer
	for i := range 50 {
		buf.WriteString(rawLine(0100644, 0100644,
			"abcdef1234567890abcdef1234567890abcdef12",
			"1234567890abcdef1234567890abcdef12345678", "M",
			fmt.Sprintf("assets/img%d.png", i)))
	}
	for i := range 50 {
		fmt.Fprintf(&buf, "diff --git a/assets/img%d.png b/assets/img%d.png\n", i, i)
		fmt.Fprintf(&buf, "Binary files a/assets/img%d.png and b/assets/img%d.png differ\n", i, i)
	}
	input := buf.String()

	b.ReportAllocs()
	for b.Loop() {
		parser := NewParser(git.HashSHA1, strings.NewReader(input), Limits{})
		for parser.Parse() {
			_ = parser.Patch()
		}
	}
}

// BenchmarkParseHunks benchmarks the hunk parsing function directly.
func BenchmarkParseHunks(b *testing.B) {
	var lines [][]byte
	lines = append(lines, []byte("@@ -1,100 +1,120 @@ func main() {\n"))
	for i := range 100 {
		lines = append(lines, []byte(fmt.Sprintf("-old line %d\n", i)))
	}
	for i := range 120 {
		lines = append(lines, []byte(fmt.Sprintf("+new line %d\n", i)))
	}

	b.ReportAllocs()
	for b.Loop() {
		_, _ = parseHunks(lines)
	}
}
