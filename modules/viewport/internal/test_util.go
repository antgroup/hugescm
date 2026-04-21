package internal

import (
	"fmt"
	"os"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

// Test helper colors and styles
var (
	Blue    = lipgloss.Color("#0000FF")
	BlueBg  = lipgloss.NewStyle().Background(Blue)
	BlueFg  = lipgloss.NewStyle().Foreground(Blue)
	Green   = lipgloss.Color("#00FF00")
	GreenBg = lipgloss.NewStyle().Background(Green)
	GreenFg = lipgloss.NewStyle().Foreground(Green)
	Red     = lipgloss.Color("#FF0000")
	RedBg   = lipgloss.NewStyle().Background(Red)
	RedFg   = lipgloss.NewStyle().Foreground(Red)
)

// CmpStr compares two strings and fails the test if they are not equal
func CmpStr(t *testing.T, expected, actual string, extra ...string) {
	_, file, line, _ := runtime.Caller(1)
	testName := t.Name()
	diff := cmp.Diff(expected, actual)
	if len(expected) > 80 {
		diff = cmp.Diff(expected, actual, cmpopts.AcyclicTransformer("SplitLines", func(s string) []string {
			return strings.Split(s, "\n")
		}))
	}
	if diff != "" {
		t.Errorf("\nTest %q failed at %s:%d\nDiff (-expected +actual):\n%s%s", testName, file, line, diff, strings.Join(extra, "\n"))
	}
}

// RunWithTimeout runs a test function with a timeout.
func RunWithTimeout(t *testing.T, runTest func(t *testing.T), timeout time.Duration) {
	t.Helper()

	// warmup runs
	for range 3 {
		runTest(t)
	}

	// actual measured runs
	var durations []time.Duration
	for range 3 {
		done := make(chan struct{})
		var testErr error
		start := time.Now()

		go func() {
			defer func() {
				if r := recover(); r != nil {
					testErr = fmt.Errorf("test panicked: %v", r)
				}
				close(done)
			}()

			subT := &testing.T{}
			runTest(subT)
			if subT.Failed() {
				testErr = fmt.Errorf("test failed in goroutine")
			}
		}()

		select {
		case <-done:
			if testErr != nil {
				t.Fatal(testErr)
			}
			durations = append(durations, time.Since(start))
		case <-time.After(timeout):
			if os.Getenv("CI") != "" {
				t.Logf("Test took too long (%v) but not failing in CI", timeout)
				return
			}
			t.Fatalf("Test took too long: %v", timeout)
		}

		runtime.GC()
		time.Sleep(time.Millisecond * 10)
	}

	slices.Sort(durations)
	median := durations[len(durations)/2]
	t.Logf("Test timing: median=%v min=%v max=%v",
		median, durations[0], durations[len(durations)-1])
}

// Pad pads the given lines to the specified width and height
func Pad(width, height int, lines []string) string {
	var res []string
	for _, line := range lines {
		resLine := line
		numSpaces := width - lipgloss.Width(line)
		if numSpaces > 0 {
			resLine += strings.Repeat(" ", numSpaces)
		}
		res = append(res, resLine)
	}
	numEmptyLines := height - len(lines)
	for range numEmptyLines {
		res = append(res, strings.Repeat(" ", width))
	}
	return strings.Join(res, "\n")
}

// MakeKeyMsg creates a tea.KeyPressMsg for the given rune.
// For uppercase letters, it sets the shift modifier and uses the lowercase code.
func MakeKeyMsg(k rune) tea.KeyPressMsg {
	if unicode.IsUpper(k) {
		return tea.KeyPressMsg{Code: unicode.ToLower(k), Text: string(k), Mod: tea.ModShift}
	}
	return tea.KeyPressMsg{Code: k, Text: string(k)}
}
