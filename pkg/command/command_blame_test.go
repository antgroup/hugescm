// Copyright ©️ Ant Group. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/antgroup/hugescm/modules/plumbing"
	"github.com/antgroup/hugescm/modules/term"
	"github.com/antgroup/hugescm/pkg/zeta"
)

func TestRenderBlame(t *testing.T) {
	hash := plumbing.NewHash("4b2982c5c8835dfc3c1a8d0eddca9100e1aee1b7e7b9da44160bc9de99aa0b77")
	date := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	b := &zeta.BlameResult{
		Path: "a.go",
		Lines: []*zeta.Line{
			{Hash: hash, AuthorName: "Alice", Date: date, Text: "hello"},
			{Hash: hash, AuthorName: "Bob", Date: date, Text: "world"},
		},
	}

	plain := renderBlame(b, term.LevelNone)
	if strings.Contains(plain, "\x1b[") {
		t.Fatalf("plain output must not contain ANSI escapes:\n%s", plain)
	}
	// git-blame layout: "<hash8> (<author padded> <date> <line>) <text>"
	wantLine1 := "4b2982c5 (Alice 2026-01-01 12:00:00 +0000 1) hello"
	wantLine2 := "4b2982c5 (Bob   2026-01-01 12:00:00 +0000 2) world"
	if !strings.Contains(plain, wantLine1+"\n") || !strings.Contains(plain, wantLine2+"\n") {
		t.Fatalf("plain output layout mismatch:\n%s", plain)
	}

	colored := renderBlame(b, term.Level256)
	if !strings.Contains(colored, "\x1b[33m4b2982c5\x1b[0m") {
		t.Fatalf("colored output must colorize the commit hash in yellow:\n%q", colored)
	}
	if !strings.Contains(colored, "\x1b[2m2026-01-01 12:00:00 +0000 1\x1b[0m") {
		t.Fatalf("colored output must dim the date and line number:\n%q", colored)
	}
	// strip the escapes and compare against the plain rendering
	stripped := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(colored, "")
	if stripped != plain {
		t.Fatalf("colored output after stripping escapes must equal plain:\nstripped: %q\nplain:    %q", stripped, plain)
	}
}
