package systemproxy

import (
	"reflect"
	"strings"
	"testing"
)

// parseProxyOverrideForTest is a copy of parseProxyOverride for testing on non-Windows platforms
// Windows format: "localhost;127.0.0.1;<local>;*.example.com"
// <local> means bypass proxy for all local addresses (simple hostnames without dots)
func parseProxyOverrideForTest(proxyOverride string) (hosts []string, bypassLocal bool) {
	items := strings.SplitSeq(proxyOverride, ";")
	for item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.EqualFold(item, "<local>") {
			bypassLocal = true
			continue
		}
		hosts = append(hosts, item)
	}
	return hosts, bypassLocal
}

// TestParseProxyOverride tests the Windows ProxyOverride parsing logic
func TestParseProxyOverride(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedHosts []string
		expectedLocal bool
	}{
		{
			name:          "empty string",
			input:         "",
			expectedHosts: nil,
			expectedLocal: false,
		},
		{
			name:          "simple hosts",
			input:         "localhost;127.0.0.1;192.168.0.0/16",
			expectedHosts: []string{"localhost", "127.0.0.1", "192.168.0.0/16"},
			expectedLocal: false,
		},
		{
			name:          "with local tag",
			input:         "localhost;127.0.0.1;<local>",
			expectedHosts: []string{"localhost", "127.0.0.1"},
			expectedLocal: true,
		},
		{
			name:          "local tag only",
			input:         "<local>",
			expectedHosts: nil,
			expectedLocal: true,
		},
		{
			name:          "local tag with different case",
			input:         "<LOCAL>",
			expectedHosts: nil,
			expectedLocal: true,
		},
		{
			name:          "local tag mixed case",
			input:         "<Local>",
			expectedHosts: nil,
			expectedLocal: true,
		},
		{
			name:          "with wildcards",
			input:         "*.example.com;*.test.com;<local>",
			expectedHosts: []string{"*.example.com", "*.test.com"},
			expectedLocal: true,
		},
		{
			name:          "with spaces",
			input:         " localhost ; 127.0.0.1 ; <local> ",
			expectedHosts: []string{"localhost", "127.0.0.1"},
			expectedLocal: true,
		},
		{
			name:          "multiple semicolons",
			input:         "localhost;;127.0.0.1;;",
			expectedHosts: []string{"localhost", "127.0.0.1"},
			expectedLocal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hosts, bypassLocal := parseProxyOverrideForTest(tt.input)
			if !reflect.DeepEqual(hosts, tt.expectedHosts) {
				t.Errorf("hosts = %v, want %v", hosts, tt.expectedHosts)
			}
			if bypassLocal != tt.expectedLocal {
				t.Errorf("bypassLocal = %v, want %v", bypassLocal, tt.expectedLocal)
			}
		})
	}
}
