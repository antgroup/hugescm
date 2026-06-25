package git

import (
	"fmt"
	"os"
	"testing"
	"time"
)

func TestParseVersionOutput(t *testing.T) {
	tests := []struct {
		input   string
		want    Version
		wantErr bool
	}{
		{
			input: "git version 2.39.1",
			want:  Version{versionString: "2.39.1", major: 2, minor: 39, patch: 1},
		},
		{
			input: "git version 2.50.1 (Apple Git-155)",
			want:  Version{versionString: "2.50.1", major: 2, minor: 50, patch: 1},
		},
		{
			input: "git version 2.39.1.rc1",
			want:  Version{versionString: "2.39.1.rc1", major: 2, minor: 39, patch: 1, rc: true},
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ParseVersionOutput([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseVersionOutput(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if got != tt.want {
				t.Errorf("ParseVersionOutput(%q) = %+v, want %+v", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseVersionAppleGit(t *testing.T) {
	// ParseVersion should only accept pure version strings; the Apple Git suffix
	// is handled by ParseVersionOutput via space-splitting.
	_, err := ParseVersion("2.50.1 (Apple Git-155)")
	if err == nil {
		t.Fatal("ParseVersion should reject version strings with parenthesized suffixes")
	}
}

func TestVersion(t *testing.T) {
	for range 10 {
		now := time.Now()
		v, err := VersionDetect()
		if err != nil {
			fmt.Fprintf(os.Stderr, "%v\n", err)
			return
		}
		fmt.Fprintf(os.Stderr, "%s use time: %v\n", v, time.Since(now))
	}
}

func TestIsGitVersionAtLeast(t *testing.T) {
	fmt.Fprintf(os.Stderr, ">= 2.36.0: %v\n", IsGitVersionAtLeast(NewVersion(2, 36, 0)))
}
