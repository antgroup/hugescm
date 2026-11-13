// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"strings"
	"testing"

	"github.com/antgroup/hugescm/modules/term"
)

func TestApplyColorOverride(t *testing.T) {
	saveStdout, saveStderr := term.StdoutLevel, term.StderrLevel
	t.Cleanup(func() {
		term.StdoutLevel, term.StderrLevel = saveStdout, saveStderr
	})

	t.Run("auto keeps detected levels", func(t *testing.T) {
		term.StdoutLevel, term.StderrLevel = saveStdout, saveStderr
		g := &Globals{Color: "auto"}
		if err := g.ApplyColorOverride(); err != nil {
			t.Fatalf("auto: %v", err)
		}
		if term.StdoutLevel != saveStdout || term.StderrLevel != saveStderr {
			t.Fatalf("auto must not touch the detected levels")
		}
	})

	t.Run("always forces at least 256 colors", func(t *testing.T) {
		term.StdoutLevel, term.StderrLevel = term.LevelNone, term.LevelNone
		g := &Globals{Color: "always"}
		if err := g.ApplyColorOverride(); err != nil {
			t.Fatalf("always: %v", err)
		}
		if term.StdoutLevel < term.Level256 || term.StderrLevel < term.Level256 {
			t.Fatalf("always must force >= Level256, got stdout=%v stderr=%v", term.StdoutLevel, term.StderrLevel)
		}
	})

	t.Run("always overrides env-suppressed levels (NO_COLOR)", func(t *testing.T) {
		// modules/term consumes NO_COLOR during package init, which has
		// already run before any test executes; simulate its effect (both
		// levels suppressed to LevelNone) and require the explicit flag to
		// win. NO_COLOR + zeta --color=always is the combination users hit
		// most, and no env mutation here can reach the already-run init,
		// so the override logic is what this test pins.
		term.StdoutLevel, term.StderrLevel = term.LevelNone, term.LevelNone
		g := &Globals{Color: "always"}
		if err := g.ApplyColorOverride(); err != nil {
			t.Fatalf("always: %v", err)
		}
		if term.StdoutLevel < term.Level256 || term.StderrLevel < term.Level256 {
			t.Fatalf("always must override NO_COLOR suppression, got stdout=%v stderr=%v", term.StdoutLevel, term.StderrLevel)
		}
	})

	t.Run("never disables all colors", func(t *testing.T) {
		term.StdoutLevel, term.StderrLevel = term.Level16M, term.Level16M
		g := &Globals{Color: "never"}
		if err := g.ApplyColorOverride(); err != nil {
			t.Fatalf("never: %v", err)
		}
		if term.StdoutLevel != term.LevelNone || term.StderrLevel != term.LevelNone {
			t.Fatalf("never must set both levels to LevelNone")
		}
	})

	t.Run("invalid value is rejected", func(t *testing.T) {
		for _, v := range []string{"sometimes", "TRUE", "0"} {
			g := &Globals{Color: v}
			err := g.ApplyColorOverride()
			if err == nil {
				t.Fatalf("value %q must be rejected", v)
			}
			if !strings.Contains(err.Error(), "unsupported --color") {
				t.Fatalf("unexpected error for %q: %v", v, err)
			}
		}
	})

	t.Run("empty means auto", func(t *testing.T) {
		term.StdoutLevel, term.StderrLevel = saveStdout, saveStderr
		g := &Globals{}
		if err := g.ApplyColorOverride(); err != nil {
			t.Fatalf("empty: %v", err)
		}
		if term.StdoutLevel != saveStdout || term.StderrLevel != saveStderr {
			t.Fatalf("empty must behave like auto")
		}
	})
}
