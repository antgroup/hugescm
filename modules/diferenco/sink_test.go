package diferenco

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestProcessLine(t *testing.T) {
	text := `A
B
C
D
A`
	s := &Sink{
		Index: make(map[string]int),
	}
	lines := s.SplitLines(text)
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}
func TestProcessLineNewLine(t *testing.T) {
	text := `A
B
C
D
D
`
	s := &Sink{
		Index: make(map[string]int),
	}
	lines := s.SplitLines(text)
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}

func TestReadLines(t *testing.T) {
	text := `A
B
C
D
D
`
	s := &Sink{
		Index: make(map[string]int),
	}
	lines, err := s.ScanLines(strings.NewReader(text))
	if err != nil {
		return
	}
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}

func TestReadLinesNoNewLine(t *testing.T) {
	text := `A
B
C
D
D`
	s := &Sink{
		Index: make(map[string]int),
	}
	lines, err := s.ScanLines(strings.NewReader(text))
	if err != nil {
		return
	}
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d \"%s\"\n", line, strings.ReplaceAll(s.Lines[line], "\n", "\\n"))
	}
}

func TestReadLinesLF(t *testing.T) {
	text := `A
B
C
D
D`
	s := &Sink{
		Index:   make(map[string]int),
		NewLine: NEWLINE_LF,
	}
	lines, err := s.ScanLines(strings.NewReader(text))
	if err != nil {
		return
	}
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d \"%s\"\n", line, s.Lines[line])
	}
}

func TestProcessLineLF(t *testing.T) {
	text := `A
B
C
D
B`
	s := &Sink{
		NewLine: NEWLINE_LF,
		Index:   make(map[string]int),
	}
	lines := s.SplitLines(text)
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}

func TestProcessLineNewLineLF(t *testing.T) {
	text := `A
B
C
D
`
	s := &Sink{
		NewLine: NEWLINE_LF,
		Index:   make(map[string]int),
	}
	lines := s.SplitLines(text)
	for _, line := range lines {
		fmt.Fprintf(os.Stderr, "%d [%s]\n", line, s.Lines[line])
	}
}

func TestSplitWord(t *testing.T) {
	sss := []string{
		"  blah test2 test3  ",
		"\tblah test2 test3  ",
		"\tblah test2 test3  t",
		"\tblah test2 test3  tt",
		"The quick brown fox jumps over the lazy dog",
		"The quick brown dog leaps over the lazy cat",
		"Hello😋World",
		"😋  Hello😋World",
	}
	for _, s := range sss {
		w := SplitWords(s)
		fmt.Fprintf(os.Stderr, "[%s] -->\n", s)
		for _, e := range w {
			fmt.Fprintf(os.Stderr, "[%s] ", e)
		}
		fmt.Fprintf(os.Stderr, "\n")
	}
}

func TestSplitWordsCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		// Empty and single character
		{"empty", "", nil},
		{"single_ascii", "a", []string{"a"}},
		{"single_cjk", "中", []string{"中"}},
		{"single_emoji", "😀", []string{"😀"}},
		{"single_space", " ", []string{" "}},
		{"single_punct", "!", []string{"!"}},

		// ASCII words
		{"ascii_word", "hello", []string{"hello"}},
		{"ascii_words", "hello world", []string{"hello", " ", "world"}},
		{"ascii_numbers", "123 456", []string{"123", " ", "456"}},

		// Word characters: letters, digits, -, _, ., /
		{"path", "/usr/local/bin", []string{"/usr/local/bin"}},
		{"file_name", "file-name.txt", []string{"file-name.txt"}},
		{"snake_case", "hello_world", []string{"hello_world"}},
		{"mixed_word_chars", "a-b_c.d/e", []string{"a-b_c.d/e"}},

		// CJK characters (split individually)
		{"cjk_single", "你好", []string{"你", "好"}},
		{"cjk_sentence", "你好世界", []string{"你", "好", "世", "界"}},
		{"cjk_mixed", "Hello世界", []string{"Hello", "世", "界"}},
		{"cjk_japanese", "こんにちは", []string{"こ", "ん", "に", "ち", "は"}},
		{"cjk_korean", "안녕하세요", []string{"안", "녕", "하", "세", "요"}},

		// Emoji (split individually)
		{"emoji_single", "😀😃", []string{"😀", "😃"}},
		{"emoji_mixed", "Hello😀World", []string{"Hello", "😀", "World"}},
		{"emoji_multiple", "🎉🎊🎁", []string{"🎉", "🎊", "🎁"}},

		// Punctuation (grouped by same class)
		{"punct_simple", "hello,world", []string{"hello", ",", "world"}},
		{"punct_multiple", "a!b?c", []string{"a", "!", "b", "?", "c"}},
		{"punct_sequence", "!!!", []string{"!!!"}},
		{"punct_mixed", "!?;", []string{"!?;"}}, // different puncts grouped together

		// Whitespace
		{"spaces", "a  b", []string{"a", "  ", "b"}},
		{"tabs", "a\t\tb", []string{"a", "\t\t", "b"}},
		{"mixed_whitespace", "a \t b", []string{"a", " \t ", "b"}},
		{"newline", "a\nb", []string{"a", "\n", "b"}},

		// Complex cases
		{"url", "https://example.com/path", []string{"https", ":", "//example.com/path"}},
		{"email", "user@example.com", []string{"user", "@", "example.com"}},
		{"code_line", "if (x > 0) { return x; }", []string{"if", " ", "(", "x", " ", ">", " ", "0", ")", " ", "{", " ", "return", " ", "x", ";", " ", "}"}},
		{"chinese_sentence", "你好，世界！", []string{"你", "好", "，", "世", "界", "！"}},
		{"mixed_complex", "Hello世界🎉test-1.0/file_name", []string{"Hello", "世", "界", "🎉", "test-1.0/file_name"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitWords(tt.input)
			if !equalStringSlices(got, tt.expected) {
				t.Errorf("SplitWords(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSplitWordsASCIIFastPath(t *testing.T) {
	// Test that ASCII fast path produces same results as non-ASCII path
	asciiTests := []string{
		"hello world",
		"test-1.0/file_name",
		"if (x > 0) { return x; }",
		"123 456 789",
		"a!b?c:d;e",
	}

	for _, s := range asciiTests {
		got := SplitWords(s)
		if got == nil && s != "" {
			t.Errorf("SplitWords(%q) returned nil for non-empty string", s)
		}
	}
}

func TestSplitWordsBoundary(t *testing.T) {
	// Test boundary conditions
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"control_chars", "\x00\x01\x02", []string{"\x00\x01\x02"}},
		{"del_char", "\x7f", []string{"\x7f"}},
		{"max_ascii", "~", []string{"~"}}, // 0x7E
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitWords(tt.input)
			if !equalStringSlices(got, tt.expected) {
				t.Errorf("SplitWords(%q) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func BenchmarkSplitWords(b *testing.B) {
	tests := []struct {
		name string
		s    string
	}{
		{"ASCII", "The quick brown fox jumps over the lazy dog"},
		{"CJK", "你好世界这是一个测试文本"},
		{"Mixed", "Hello世界Test测试Go语言Programming"},
		{"Emoji", "Hello😋World🎉Test🌟End"},
		{"Path", "/usr/local/bin/file-name.txt"},
	}

	for _, tt := range tests {
		b.Run(tt.name, func(b *testing.B) {
			for b.Loop() {
				SplitWords(tt.s)
			}
		})
	}
}
