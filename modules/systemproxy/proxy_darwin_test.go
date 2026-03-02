//go:build darwin

package systemproxy

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"testing"
	"time"
)

func TestFindSystemProxy(t *testing.T) {
	settings, err := findSystemProxy()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	enc := json.NewEncoder(os.Stderr)
	enc.SetIndent("", "  ")
	_ = enc.Encode(settings)
}

func TestSystemProxyConfig(t *testing.T) {
	cfg := systemProxyConfig()
	fmt.Fprintf(os.Stderr, "%v\n", cfg)
}

func TestConnectHackNews(t *testing.T) {
	client := &http.Client{
		Transport: &http.Transport{
			Proxy:                 NewSystemProxy(""),
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}
	resp, err := client.Get("https://news.ycombinator.com/")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	defer resp.Body.Close() // nolint
	fmt.Fprintf(os.Stderr, "%d %s\n", resp.StatusCode, resp.Status)
	for k, v := range resp.Header {
		if len(v) != 0 {
			fmt.Fprintf(os.Stderr, "%s: %s\n", k, v[0])
		}
	}
}

func TestParseOut(t *testing.T) {
	tests := []struct {
		name              string
		input             string
		expectedHTTPProxy string
		expectedHTTPPort  string
		expectedArray     []string
	}{
		{
			name: "simple dictionary",
			input: `<dictionary> {
  HTTPEnable : 1
  HTTPProxy : 127.0.0.1
  HTTPPort : 7890
}`,
			expectedHTTPProxy: "127.0.0.1",
			expectedHTTPPort:  "7890",
		},
		{
			name: "array with numeric indices",
			input: `<dictionary> {
  ExceptionsList : <array> {
    0 : first.com
    1 : second.com
    2 : third.com
    10 : tenth.com
    11 : eleventh.com
  }
}`,
			expectedArray: []string{
				"first.com",
				"second.com",
				"third.com",
				"tenth.com",
				"eleventh.com",
			},
		},
		{
			name: "complex proxy settings",
			input: `<dictionary> {
  ExceptionsList : <array> {
    0 : 127.0.0.1
    1 : localhost
    2 : 192.168.0.0/16
  }
  ExcludeSimpleHostnames : 1
  FTPPassive : 1
  SOCKSEnable : 1
  SOCKSPort : 13659
  SOCKSProxy : 127.0.0.1
}`,
			expectedArray: []string{
				"127.0.0.1",
				"localhost",
				"192.168.0.0/16",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			se := parseOut(tt.input)
			if se == nil {
				t.Fatal("parseOut returned nil")
			}

			if tt.expectedHTTPProxy != "" {
				got := se.string("HTTPProxy")
				if got != tt.expectedHTTPProxy {
					t.Errorf("HTTPProxy = %q, want %q", got, tt.expectedHTTPProxy)
				}
			}

			if tt.expectedHTTPPort != "" {
				got := se.string("HTTPPort")
				if got != tt.expectedHTTPPort {
					t.Errorf("HTTPPort = %q, want %q", got, tt.expectedHTTPPort)
				}
			}

			if tt.expectedArray != nil {
				got := se.array("ExceptionsList")
				if !reflect.DeepEqual(got, tt.expectedArray) {
					t.Errorf("ExceptionsList = %v, want %v", got, tt.expectedArray)
				}
			}
		})
	}
}

func TestArraySortingWithLargeIndices(t *testing.T) {
	// Test sorting with large indices to verify numeric sorting instead of string sorting
	input := `<dictionary> {
  ExceptionsList : <array> {
    0 : item0
    1 : item1
    2 : item2
    9 : item9
    10 : item10
    11 : item11
    100 : item100
    101 : item101
  }
}`

	se := parseOut(input)
	if se == nil {
		t.Fatal("parseOut returned nil")
	}

	got := se.array("ExceptionsList")
	expected := []string{
		"item0", "item1", "item2", "item9",
		"item10", "item11",
		"item100", "item101",
	}

	if !reflect.DeepEqual(got, expected) {
		t.Errorf("ExceptionsList sorting failed:\ngot:      %v\nexpected: %v", got, expected)
	}
}

func TestParseOutEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		shouldPanic bool
	}{
		{
			name:        "empty string",
			input:       "",
			shouldPanic: false,
		},
		{
			name:        "only newlines",
			input:       "\n\n\n",
			shouldPanic: false,
		},
		{
			name:        "malformed input without dictionary start",
			input:       "HTTPEnable : 1",
			shouldPanic: false,
		},
		{
			name:        "malformed input with only field assignment",
			input:       "SomeField : SomeValue\nAnotherField : AnotherValue",
			shouldPanic: false,
		},
		{
			name: "unclosed dictionary",
			input: `<dictionary> {
  HTTPEnable : 1`,
			shouldPanic: false,
		},
		{
			name: "extra closing braces",
			input: `<dictionary> {
  HTTPEnable : 1
}
}`,
			shouldPanic: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This should not panic
			result := parseOut(tt.input)
			// Result can be nil or empty section, both are valid
			t.Logf("parseOut returned: %v (nil=%v)", result, result == nil)
		})
	}
}
