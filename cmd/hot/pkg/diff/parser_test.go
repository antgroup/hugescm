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
			count, patch.Status, patch.From, patch.To, len(patch.Hunks), patch.Binary)
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
	if !patch.Binary {
		t.Error("expected binary flag to be true")
	}

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
	lines := []string{
		"--- a/main.go\n",
		"+++ b/main.go\n",
		"@@ -1,5 +1,6 @@\n",
		" package main\n",
		"\n",
		"+import \"fmt\"\n",
		" func main() {\n",
		"-	println(\"hello\")\n",
		"+	fmt.Println(\"hello world\")\n",
		" }\n",
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
	lines := []string{
		"@@ -1,3 +1,4 @@ function main() {\n",
		" package main\n",
		"+import \"fmt\"\n",
		" func main() {}\n",
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
	lines := []string{
		"@@ -1,3 +1,4 @@\n",
		" package main\n",
		"+import \"fmt\"\n",
		" func main() {}\n",
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
			count, patch.Status, path, patch.Binary, len(patch.Hunks))

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
