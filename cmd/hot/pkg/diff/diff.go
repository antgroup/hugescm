// Package diff provides a parser for git diff output.
// It parses the output of: git diff --raw --full-index --find-renames
// Based on gitaly's implementation (MIT License).
package diff

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/antgroup/hugescm/modules/diferenco"
	"github.com/antgroup/hugescm/modules/git"
)

// Patch represents a single parsed diff entry, extending diferenco.Patch with git metadata.
type Patch struct {
	*diferenco.Patch
	Status          byte // 'A', 'D', 'M', 'R', 'C', 'T' etc.
	Binary          bool
	OverflowMarker  bool
	Collapsed       bool
	TooLarge        bool
	CollectAllPaths bool
	PatchSize       int32
	LinesAdded      int32
	LinesRemoved    int32
	lineCount       int
	byteCount       int
}

// Reset clears all fields of p in a way that lets the underlying memory be reused.
func (p *Patch) Reset() {
	*p = Patch{Patch: &diferenco.Patch{}}
}

// ClearPatch clears only the patch content.
func (p *Patch) ClearPatch() {
	if p.Patch != nil {
		p.Hunks = nil
	}
}

// Parser holds necessary state for parsing a diff stream.
type Parser struct {
	hashFormat          git.HashFormat
	limits              Limits
	patchReader         *bufio.Reader
	rawLines            [][]byte
	currentPatch        Patch
	nextPatchFromPath   []byte
	unreadLine          []byte
	filesProcessed      int
	scannedLines        int // Total lines scanned (never decreases)
	scannedBytes        int // Total bytes scanned (never decreases)
	finished            bool
	stopPatchCollection bool
	err                 error
}

// Limits holds the limits at which either parsing stops or patches are collapsed.
type Limits struct {
	// EnforceLimits causes parsing to stop if Max{Files,Lines,Bytes} is reached.
	EnforceLimits bool
	// CollapseDiffs causes patches to be emptied after SafeMax{Files,Lines,Bytes} reached.
	CollapseDiffs bool
	// CollectAllPaths parses all diffs but info outside of path may be empty.
	CollectAllPaths bool
	// MaxFiles is the maximum number of files to parse.
	MaxFiles int
	// MaxLines is the maximum number of diff lines to parse.
	MaxLines int
	// MaxBytes is the maximum number of bytes to parse.
	MaxBytes int
	// SafeMaxFiles is the number of files after which subsequent files are collapsed.
	SafeMaxFiles int
	// SafeMaxLines is the number of lines after which subsequent files are collapsed.
	SafeMaxLines int
	// SafeMaxBytes is the number of bytes after which subsequent files are collapsed.
	SafeMaxBytes int
	// MaxPatchBytes is the maximum bytes a single patch can have.
	MaxPatchBytes int
	// MaxPatchBytesForFileExtension overrides MaxPatchBytes for specific file types.
	MaxPatchBytesForFileExtension map[string]int
	// PatchLimitsOnly uses only MaxPatchBytes limits, ignoring cumulative limits.
	PatchLimitsOnly bool
}

const (
	maxFilesUpperBound      = 5000
	maxLinesUpperBound      = 250000
	maxBytesUpperBound      = 5000 * 5120 // 24MB
	safeMaxFilesUpperBound  = 500
	safeMaxLinesUpperBound  = 25000
	safeMaxBytesUpperBound  = 500 * 5120 // 2.4MB
	maxPatchBytesUpperBound = 512000     // 500KB
)

var (
	rawSHA1LineRegexp   = regexp.MustCompile(`(?m)^:(\d+) (\d+) ([[:xdigit:]]{40}) ([[:xdigit:]]{40}) ([ADTUXMRC]\d*)\t(.*?)(?:\t(.*?))?$`)
	rawSHA256LineRegexp = regexp.MustCompile(`(?m)^:(\d+) (\d+) ([[:xdigit:]]{64}) ([[:xdigit:]]{64}) ([ADTUXMRC]\d*)\t(.*?)(?:\t(.*?))?$`)
)

// NewParser returns a new Parser.
func NewParser(hashFormat git.HashFormat, src io.Reader, limits Limits) *Parser {
	limits.enforceUpperBound()

	parser := &Parser{
		hashFormat: hashFormat,
		limits:     limits,
	}
	reader := bufio.NewReader(src)
	parser.cacheRawLines(reader)
	parser.patchReader = reader

	return parser
}

// Parse parses a single diff. It returns true if successful, false if finished or error.
func (parser *Parser) Parse() bool {
	if parser.finished || len(parser.rawLines) == 0 {
		return false
	}

	if err := parser.initializeCurrentPatch(); err != nil {
		return false
	}

	if parser.nextPatchFromPath == nil {
		path, err := parser.readDiffHeaderFromPath()
		if err != nil {
			parser.err = err
			return false
		}
		parser.nextPatchFromPath = path
	}

	if !bytes.Equal(parser.nextPatchFromPath, parser.currentPatchFromPath()) {
		// The current diff has an empty patch
		return true
	}

	parser.nextPatchFromPath = nil

	if err := readNextDiff(parser.patchReader, &parser.currentPatch, parser.stopPatchCollection); err != nil {
		parser.err = err
		return false
	}

	parser.scannedLines += parser.currentPatch.lineCount
	parser.scannedBytes += parser.currentPatch.byteCount

	// Calculate PatchSize from hunks
	parser.currentPatch.PatchSize = int32(parser.currentPatch.byteCount)

	if parser.limits.CollapseDiffs && parser.isOverSafeLimits() && parser.currentPatch.lineCount > 0 {
		parser.prunePatch()
		parser.currentPatch.Collapsed = true
		if parser.limits.CollectAllPaths {
			parser.currentPatch.CollectAllPaths = true
		}
	}

	if parser.limits.EnforceLimits {
		maxPatchBytesExceeded := parser.limits.MaxPatchBytes > 0 && parser.currentPatch.byteCount >= parser.maxPatchBytesForCurrentFile()
		if maxPatchBytesExceeded {
			parser.prunePatch()
			parser.currentPatch.TooLarge = true
		}

		maxFilesExceeded := exceeded(parser.filesProcessed, parser.limits.MaxFiles)
		maxLinesExceeded := exceeded(parser.scannedLines, parser.limits.MaxLines)
		maxBytesExceeded := exceeded(parser.scannedBytes, parser.limits.MaxBytes)
		maxLimitsExceeded := maxLinesExceeded || maxBytesExceeded || maxFilesExceeded
		if maxLimitsExceeded && !parser.limits.PatchLimitsOnly {
			if parser.limits.CollectAllPaths {
				parser.currentPatch.CollectAllPaths = true
				parser.currentPatch.ClearPatch()
				parser.stopPatchCollection = true
			} else {
				parser.finished = true
				parser.currentPatch.Reset()
			}
			parser.currentPatch.OverflowMarker = true
		}
	}

	return true
}

// Patch returns a successfully parsed patch. Valid until next Parse() call.
func (parser *Parser) Patch() *Patch {
	return &parser.currentPatch
}

// Err returns the error encountered during parsing.
func (parser *Parser) Err() error {
	return parser.err
}

func (parser *Parser) currentPatchFromPath() []byte {
	if parser.currentPatch.From != nil {
		return []byte(parser.currentPatch.From.Name)
	}
	if parser.currentPatch.To != nil {
		return []byte(parser.currentPatch.To.Name)
	}
	return nil
}

func (parser *Parser) cacheRawLines(reader *bufio.Reader) {
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				// Handle EOF with data - last line without newline
				if len(line) > 0 {
					if bytes.HasPrefix(line, []byte(":")) {
						parser.rawLines = append(parser.rawLines, line)
					} else {
						parser.unreadLine = line
					}
				}
			} else {
				parser.err = err
				parser.finished = true
			}
			return
		}

		if !bytes.HasPrefix(line, []byte(":")) {
			// Store the non-raw line for later use
			parser.unreadLine = line
			return
		}

		parser.rawLines = append(parser.rawLines, line)
	}
}

func (parser *Parser) nextRawLine() []byte {
	if len(parser.rawLines) == 0 {
		return nil
	}
	line := parser.rawLines[0]
	parser.rawLines = parser.rawLines[1:]
	return line
}

func (parser *Parser) initializeCurrentPatch() error {
	parser.currentPatch.Reset()

	line := parser.nextRawLine()
	if line == nil {
		return nil
	}

	if err := parseRawLine(parser.hashFormat, line, &parser.currentPatch); err != nil {
		parser.err = err
		return err
	}

	if parser.currentPatch.Status == 'T' {
		parser.handleTypeChangeDiff()
	}

	parser.filesProcessed++
	return nil
}

func (parser *Parser) readDiffHeaderFromPath() ([]byte, error) {
	var line []byte
	var err error

	for {
		// Use unread line if available
		if len(parser.unreadLine) > 0 {
			line = parser.unreadLine
			parser.unreadLine = nil
		} else {
			line, err = parser.patchReader.ReadBytes('\n')
			if err != nil {
				if errors.Is(err, io.EOF) {
					// Handle EOF with data - last line without newline
					if len(line) > 0 {
						// Process the last line
					} else {
						return nil, nil
					}
				} else {
					return nil, fmt.Errorf("read diff header line: %w", err)
				}
			}
		}

		// Skip empty lines
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}

		// Skip non-diff-header lines (index, ---, +++, new file mode, deleted file mode, etc.)
		if bytes.HasPrefix(line, []byte("index ")) ||
			bytes.HasPrefix(line, []byte("---")) ||
			bytes.HasPrefix(line, []byte("+++")) ||
			bytes.HasPrefix(line, []byte("new file mode ")) ||
			bytes.HasPrefix(line, []byte("deleted file mode ")) ||
			bytes.HasPrefix(line, []byte("old mode ")) ||
			bytes.HasPrefix(line, []byte("new mode ")) ||
			bytes.HasPrefix(line, []byte("similarity index ")) ||
			bytes.HasPrefix(line, []byte("copy from ")) ||
			bytes.HasPrefix(line, []byte("copy to ")) ||
			bytes.HasPrefix(line, []byte("rename from ")) ||
			bytes.HasPrefix(line, []byte("rename to ")) {
			continue
		}

		// Hand-parse diff --git header instead of regex
		path, err := parseDiffHeaderPath(line)
		if err != nil {
			return nil, err
		}
		return path, nil
	}
}

// parseDiffHeaderPath hand-parses "diff --git a/path b/path" to extract the from-path
// This function properly handles quoted paths with escape sequences
func parseDiffHeaderPath(line []byte) ([]byte, error) {
	// Must start with "diff --git "
	if !bytes.HasPrefix(line, []byte("diff --git ")) {
		return nil, fmt.Errorf("not a diff --git header: %q", line)
	}

	line = line[11:] // Skip "diff --git "

	// Parse two paths: "a/path" "b/path" or "a/path" b/path or a/path b/path
	paths, err := parseTwoPaths(line)
	if err != nil {
		return nil, err
	}

	if len(paths) != 2 {
		return nil, fmt.Errorf("expected 2 paths in diff header, got %d", len(paths))
	}

	// Extract first path (from-path)
	path1 := paths[0]

	// Verify it starts with "a/"
	if !bytes.HasPrefix(path1, []byte("a/")) {
		return nil, fmt.Errorf("first path must start with a/: %q", path1)
	}

	// Verify second path starts with "b/"
	if len(paths) > 1 && !bytes.HasPrefix(paths[1], []byte("b/")) {
		return nil, fmt.Errorf("second path must start with b/: %q", paths[1])
	}

	// Strip "a/" prefix and unescape
	path := path1[2:]
	return unescape(path), nil
}

// parseTwoPaths parses two paths from a diff header line
// Handles both quoted and unquoted paths
func parseTwoPaths(line []byte) ([][]byte, error) {
	var paths [][]byte

	for len(line) > 0 && len(paths) < 2 {
		// Skip leading whitespace
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			break
		}

		var path []byte
		var err error

		if line[0] == '"' {
			// Quoted path: find matching quote handling escape sequences
			path, line, err = parseQuotedPath(line)
			if err != nil {
				return nil, err
			}
			// Unquote after extracting the path
			path = unquoteBytes(path)
		} else {
			// Unquoted path: find next whitespace or end
			path, line = parseUnquotedPath(line)
		}

		if len(path) > 0 {
			paths = append(paths, path)
		}
	}

	return paths, nil
}

// parseQuotedPath parses a quoted path, handling escape sequences
func parseQuotedPath(line []byte) ([]byte, []byte, error) {
	if len(line) == 0 || line[0] != '"' {
		return nil, line, fmt.Errorf("expected quoted path")
	}

	// Find matching quote, handling escape sequences
	i := 1
	for i < len(line) {
		if line[i] == '\\' && i+1 < len(line) {
			// Skip escaped character (handles \", \\, and other escapes)
			i += 2
			continue
		}
		if line[i] == '"' {
			// Found matching quote
			path := line[:i+1]
			remaining := line[i+1:]
			return path, remaining, nil
		}
		i++
	}

	return nil, line, fmt.Errorf("unclosed quote in path: %q", line)
}

// parseUnquotedPath parses an unquoted path up to next whitespace
func parseUnquotedPath(line []byte) ([]byte, []byte) {
	i := 0
	for i < len(line) && line[i] != ' ' && line[i] != '\t' {
		i++
	}
	return line[:i], line[i:]
}

func (parser *Parser) handleTypeChangeDiff() {
	// Type change: split into deletion + addition
	// Use To.Name for synthetic add path, not From.Name
	newRawLine := fmt.Sprintf(
		":%o %o %s %s A\t%s\n",
		0,
		parser.currentPatch.To.Mode,
		parser.hashFormat.ZeroOID(),
		parser.currentPatch.To.Hash,
		parser.currentPatch.To.Name,
	)

	parser.currentPatch.From = &diferenco.File{
		Name: parser.currentPatch.From.Name,
		Hash: parser.currentPatch.From.Hash,
		Mode: 0,
	}
	parser.currentPatch.To = nil

	parser.rawLines = append([][]byte{[]byte(newRawLine)}, parser.rawLines...)
}

func parseRawLine(hashFormat git.HashFormat, line []byte, patch *Patch) error {
	var re *regexp.Regexp
	switch hashFormat {
	case git.HashSHA1:
		re = rawSHA1LineRegexp
	case git.HashSHA256:
		re = rawSHA256LineRegexp
	default:
		return fmt.Errorf("cannot parse raw diff line with unknown hash format %q", hashFormat)
	}

	matches := re.FindSubmatch(line)
	if len(matches) == 0 {
		return fmt.Errorf("raw line regexp mismatch")
	}

	oldMode, err := strconv.ParseInt(string(matches[1]), 8, 32)
	if err != nil {
		return fmt.Errorf("parse old mode: %w", err)
	}

	newMode, err := strconv.ParseInt(string(matches[2]), 8, 32)
	if err != nil {
		return fmt.Errorf("parse new mode: %w", err)
	}

	oldOID := string(matches[3])
	newOID := string(matches[4])
	status := matches[5][0]

	fromPath := unescape(unquoteBytes(matches[6]))
	var toPath []byte

	if status == 'C' || status == 'R' {
		if len(matches) < 8 || len(matches[7]) == 0 {
			return fmt.Errorf("raw line missing target path for status %c", status)
		}
		toPath = unescape(unquoteBytes(matches[7]))
	} else {
		toPath = fromPath
	}

	// Build From file info
	if oldOID != hashFormat.ZeroOID() {
		patch.From = &diferenco.File{
			Name: string(fromPath),
			Hash: oldOID,
			Mode: uint32(oldMode),
		}
	}

	// Build To file info
	if newOID != hashFormat.ZeroOID() {
		patch.To = &diferenco.File{
			Name: string(toPath),
			Hash: newOID,
			Mode: uint32(newMode),
		}
	}

	patch.Status = status
	return nil
}

func readNextDiff(reader *bufio.Reader, patch *Patch, skipPatch bool) error {
	var patchLines []string
	for currentPatchDone := false; !currentPatchDone || reader.Buffered() > 0; {
		line, err := reader.Peek(10)
		if errors.Is(err, io.EOF) {
			currentPatchDone = true
		} else if err != nil {
			return fmt.Errorf("peek diff line: %w", err)
		}

		switch {
		case bytes.HasPrefix(line, []byte("diff --git")):
			// Parse hunks before returning
			if !skipPatch && len(patchLines) > 0 {
				hunks, err := parseHunks(patchLines)
				if err != nil {
					return err
				}
				patch.Hunks = hunks
			}
			return nil
		case bytes.HasPrefix(line, []byte("---")) || bytes.HasPrefix(line, []byte("+++")):
			if len(patchLines) == 0 {
				if err := discardLine(reader); err != nil {
					return err
				}
				continue
			}
		case bytes.HasPrefix(line, []byte("@@")):
			if err := consumeChunkLine(reader, patch, skipPatch, false, &patchLines); err != nil {
				return err
			}
		case bytes.HasPrefix(line, []byte("Binary")):
			patch.Binary = true
			patch.IsBinary = true
			fallthrough
		case bytes.HasPrefix(line, []byte("-")) ||
			bytes.HasPrefix(line, []byte("+")) ||
			bytes.HasPrefix(line, []byte(" ")) ||
			bytes.HasPrefix(line, []byte("\\")) ||
			bytes.HasPrefix(line, []byte("~\n")):
			if err := consumeChunkLine(reader, patch, skipPatch, true, &patchLines); err != nil {
				return err
			}
		default:
			if err := discardLine(reader); err != nil {
				return err
			}
		}
	}

	// Parse hunks for the last patch
	if !skipPatch && len(patchLines) > 0 {
		hunks, err := parseHunks(patchLines)
		if err != nil {
			return err
		}
		patch.Hunks = hunks
	}
	return nil
}

func consumeChunkLine(reader *bufio.Reader, patch *Patch, skipPatch, updateStats bool, patchLines *[]string) error {
	var byteCount int
	for done := false; !done; {
		line, err := reader.ReadSlice('\n')
		if updateStats && byteCount == 0 && len(line) > 0 {
			switch line[0] {
			case '+':
				patch.LinesAdded++
			case '-':
				patch.LinesRemoved++
			}
		}
		byteCount += len(line)

		switch {
		case errors.Is(err, bufio.ErrBufferFull):
			// long line: keep reading
		case err != nil && !errors.Is(err, io.EOF):
			return fmt.Errorf("read chunk line: %w", err)
		default:
			done = true
		}

		if !skipPatch {
			*patchLines = append(*patchLines, string(line))
		}
	}

	if updateStats {
		patch.byteCount += byteCount
		patch.lineCount++
	}
	return nil
}

func discardLine(reader *bufio.Reader) error {
	_, err := reader.ReadBytes('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("read line: %w", err)
	}
	return nil
}

func (limit *Limits) enforceUpperBound() {
	limit.MaxFiles = min(limit.MaxFiles, maxFilesUpperBound)
	limit.MaxLines = min(limit.MaxLines, maxLinesUpperBound)
	limit.MaxBytes = min(limit.MaxBytes, maxBytesUpperBound)
	limit.SafeMaxFiles = min(limit.SafeMaxFiles, safeMaxFilesUpperBound)
	limit.SafeMaxLines = min(limit.SafeMaxLines, safeMaxLinesUpperBound)
	limit.SafeMaxBytes = min(limit.SafeMaxBytes, safeMaxBytesUpperBound)
	limit.MaxPatchBytes = min(limit.MaxPatchBytes, maxPatchBytesUpperBound)
}

func (parser *Parser) prunePatch() {
	// Only clear patch content, do NOT decrease scannedLines/scannedBytes
	// Cumulative limits track what was actually read, not what is kept
	parser.currentPatch.ClearPatch()
}

// exceeded returns true if current > limit and limit > 0.
// A limit of 0 means "no limit", so it never triggers exceeded.
func exceeded(current, limit int) bool {
	return limit > 0 && current > limit
}

func (parser *Parser) isOverSafeLimits() bool {
	return exceeded(parser.filesProcessed, parser.limits.SafeMaxFiles) ||
		exceeded(parser.scannedLines, parser.limits.SafeMaxLines) ||
		exceeded(parser.scannedBytes, parser.limits.SafeMaxBytes)
}

func (parser *Parser) maxPatchBytesForCurrentFile() int {
	if len(parser.limits.MaxPatchBytesForFileExtension) > 0 {
		var toPath string
		if parser.currentPatch.To != nil {
			toPath = parser.currentPatch.To.Name
		} else if parser.currentPatch.From != nil {
			toPath = parser.currentPatch.From.Name
		}

		if toPath != "" {
			fileName := filepath.Base(toPath)
			key := filepath.Ext(fileName)
			if key == "" {
				key = fileName
			}
			if limit, ok := parser.limits.MaxPatchBytesForFileExtension[key]; ok {
				return limit
			}
		}
	}
	return parser.limits.MaxPatchBytes
}

// unescape unescapes the escape codes used by 'git diff'.
func unescape(s []byte) []byte {
	var unescaped []byte

	for i := 0; i < len(s); i++ {
		if s[i] == '\\' {
			if i+3 < len(s) && isOctalDigit(s[i+1]) && isOctalDigit(s[i+2]) && isOctalDigit(s[i+3]) {
				octalByte, err := strconv.ParseUint(string(s[i+1:i+4]), 8, 8)
				if err == nil {
					unescaped = append(unescaped, byte(octalByte))
					i += 3
					continue
				}
			}

			if i+1 < len(s) {
				var unescapedByte byte

				switch s[i+1] {
				case '"', '\\', '/', '\'':
					unescapedByte = s[i+1]
				case 'a':
					unescapedByte = '\a'
				case 'b':
					unescapedByte = '\b'
				case 'f':
					unescapedByte = '\f'
				case 'n':
					unescapedByte = '\n'
				case 'r':
					unescapedByte = '\r'
				case 't':
					unescapedByte = '\t'
				case 'v':
					unescapedByte = '\v'
				default:
					unescaped = append(unescaped, '\\')
					unescapedByte = s[i+1]
				}

				unescaped = append(unescaped, unescapedByte)
				i++
				continue
			}
		}

		unescaped = append(unescaped, s[i])
	}

	return unescaped
}

func isOctalDigit(b byte) bool {
	return b >= '0' && b <= '7'
}

// unquoteBytes removes surrounding quotes from a byte slice
func unquoteBytes(s []byte) []byte {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	return s
}

// parseHunks parses collected patch lines into diferenco.Hunk structures.
func parseHunks(lines []string) ([]*diferenco.Hunk, error) {
	if len(lines) == 0 {
		return nil, nil
	}

	var hunks []*diferenco.Hunk
	var currentHunk *diferenco.Hunk

	for _, line := range lines {
		if strings.HasPrefix(line, "@@") {
			if currentHunk != nil {
				hunks = append(hunks, currentHunk)
			}

			fromLine, fromCount, toLine, toCount, section, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}

			currentHunk = &diferenco.Hunk{
				FromLine: fromLine,
				ToLine:   toLine,
				Section:  section,
			}
			_ = fromCount // Reserved for future validation
			_ = toCount   // Reserved for future validation
			continue
		}

		if currentHunk == nil || len(line) == 0 {
			continue
		}

		// Skip "\ No newline at end of file" marker - it's metadata, not content
		if strings.HasPrefix(line, "\\ No newline at end of file") {
			continue
		}

		var kind diferenco.Operation
		switch line[0] {
		case '+':
			kind = diferenco.Insert
		case '-':
			kind = diferenco.Delete
		case ' ':
			kind = diferenco.Equal
		default:
			continue
		}

		currentHunk.Lines = append(currentHunk.Lines, diferenco.Line{
			Kind:    kind,
			Content: line[1:],
		})
	}

	if currentHunk != nil {
		hunks = append(hunks, currentHunk)
	}

	return hunks, nil
}

// parseHunkHeader parses a hunk header line.
// Format: @@ -start,count +start,count @@ section
func parseHunkHeader(header string) (fromLine, fromCount, toLine, toCount int, section string, err error) {
	if !strings.HasPrefix(header, "@@ ") {
		return 0, 0, 0, 0, "", fmt.Errorf("malformed hunk header: %q", header)
	}

	rest := strings.TrimPrefix(header, "@@ ")
	before, after, ok := strings.Cut(rest, " @@")
	if !ok {
		return 0, 0, 0, 0, "", fmt.Errorf("malformed hunk header: %q", header)
	}

	body := before
	remain := after // skip " @@"
	if len(remain) > 0 && remain[0] == ' ' {
		section = strings.TrimRight(remain[1:], "\r\n")
	}

	fields := strings.Fields(body)
	if len(fields) != 2 {
		return 0, 0, 0, 0, "", fmt.Errorf("malformed hunk header: %q", header)
	}
	if !strings.HasPrefix(fields[0], "-") || !strings.HasPrefix(fields[1], "+") {
		return 0, 0, 0, 0, "", fmt.Errorf("malformed hunk header: %q", header)
	}

	fromLine, fromCount, err = parseRange(fields[0], '-')
	if err != nil {
		return 0, 0, 0, 0, "", fmt.Errorf("malformed hunk header: %q", header)
	}
	toLine, toCount, err = parseRange(fields[1], '+')
	if err != nil {
		return 0, 0, 0, 0, "", fmt.Errorf("malformed hunk header: %q", header)
	}

	return fromLine, fromCount, toLine, toCount, section, nil
}

// parseRange parses a line range specification.
// Format: -start,count or +start,count or -start or +start
func parseRange(s string, prefix byte) (start, count int, err error) {
	if len(s) < 2 || s[0] != prefix {
		return 0, 0, fmt.Errorf("invalid range: %q", s)
	}
	s = s[1:]

	before, after, ok := strings.Cut(s, ",")
	if !ok {
		start, err = strconv.Atoi(s)
		if err != nil {
			return 0, 0, err
		}
		return start, 1, nil
	}

	start, err = strconv.Atoi(before)
	if err != nil {
		return 0, 0, err
	}
	count, err = strconv.Atoi(after)
	if err != nil {
		return 0, 0, err
	}
	if count < 0 {
		return 0, 0, fmt.Errorf("invalid count: %d", count)
	}
	return start, count, nil
}
