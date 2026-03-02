// Copyright 2011 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package systemproxy

import (
	"context"
	"net"
	"testing"
)

func TestNormalizeHost(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Example.COM", "example.com"},
		{"EXAMPLE.COM", "example.com"},
		{"example.com.", "example.com"},
		{"Example.COM.", "example.com"},
		{"localhost", "localhost"},
		{"LocalHost", "localhost"},
		{"LOCALHOST.", "localhost"},
		{"192.168.1.1", "192.168.1.1"},
		{"::1", "::1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeHost(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeHost(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

// testDialer is a simple dialer used to identify which dialer was selected
type testDialer struct {
	name string
}

func (d *testDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return nil, nil
}

func TestPerHostAddZoneCaseInsensitive(t *testing.T) {
	bypass := &testDialer{name: "bypass"}
	def := &testDialer{name: "default"}
	p := NewPerHost(def, bypass)

	// Add zone with mixed case
	p.AddZone("Example.COM")

	// Test that lowercase version matches
	d := p.dialerForRequest("www.example.com")
	if d != bypass {
		t.Error("expected bypass dialer for www.example.com")
	}

	// Test that uppercase version matches
	d = p.dialerForRequest("WWW.EXAMPLE.COM")
	if d != bypass {
		t.Error("expected bypass dialer for WWW.EXAMPLE.COM")
	}

	// Test that zone itself matches
	d = p.dialerForRequest("example.com")
	if d != bypass {
		t.Error("expected bypass dialer for example.com")
	}
}

func TestPerHostAddHostCaseInsensitive(t *testing.T) {
	bypass := &testDialer{name: "bypass"}
	def := &testDialer{name: "default"}
	p := NewPerHost(def, bypass)

	// Add host with mixed case
	p.AddHost("LocalHost")

	// Test that lowercase version matches
	d := p.dialerForRequest("localhost")
	if d != bypass {
		t.Error("expected bypass dialer for localhost")
	}

	// Test that uppercase version matches
	d = p.dialerForRequest("LOCALHOST")
	if d != bypass {
		t.Error("expected bypass dialer for LOCALHOST")
	}
}

func TestPerHostTrailingDot(t *testing.T) {
	bypass := &testDialer{name: "bypass"}
	def := &testDialer{name: "default"}
	p := NewPerHost(def, bypass)

	// Add host without trailing dot
	p.AddHost("example.com")

	// Test that version with trailing dot matches
	d := p.dialerForRequest("example.com.")
	if d != bypass {
		t.Error("expected bypass dialer for example.com.")
	}

	// Add zone
	p.AddZone("test.com")

	// Test that FQDN with trailing dot matches zone
	d = p.dialerForRequest("www.test.com.")
	if d != bypass {
		t.Error("expected bypass dialer for www.test.com.")
	}
}

func TestPerHostAddFromStringCaseInsensitive(t *testing.T) {
	bypass := &testDialer{name: "bypass"}
	def := &testDialer{name: "default"}
	p := NewPerHost(def, bypass)

	// Add hosts from string with mixed case
	p.AddFromString("LocalHost,*.Example.COM")

	// Test exact host match with different case
	d := p.dialerForRequest("LOCALHOST")
	if d != bypass {
		t.Error("expected bypass dialer for LOCALHOST")
	}

	// Test zone match with different case
	d = p.dialerForRequest("www.example.com")
	if d != bypass {
		t.Error("expected bypass dialer for www.example.com")
	}

	d = p.dialerForRequest("WWW.EXAMPLE.COM")
	if d != bypass {
		t.Error("expected bypass dialer for WWW.EXAMPLE.COM")
	}
}

func TestPerHostNotMatch(t *testing.T) {
	bypass := &testDialer{name: "bypass"}
	def := &testDialer{name: "default"}
	p := NewPerHost(def, bypass)

	// Add some bypass rules
	p.AddHost("localhost")
	p.AddZone("example.com")

	// Test that unrelated host goes to default
	d := p.dialerForRequest("other.com")
	if d != def {
		t.Error("expected default dialer for other.com")
	}

	d = p.dialerForRequest("www.other.com")
	if d != def {
		t.Error("expected default dialer for www.other.com")
	}
}

func TestPerHostBypassSimpleHostnames(t *testing.T) {
	bypass := &testDialer{name: "bypass"}
	def := &testDialer{name: "default"}
	p := NewPerHost(def, bypass)

	// Enable bypass for simple hostnames
	p.SetBypassSimpleHostnames(true)

	// Test simple hostnames (no dots) should bypass
	tests := []struct {
		host     string
		expected Dialer
	}{
		{"localhost", bypass},
		{"server", bypass},
		{"printer", bypass},
		{"myserver", bypass},
		{"LOCALHOST", bypass}, // case insensitive
		{"Server", bypass},    // case insensitive
		// FQDNs and IPs should NOT bypass
		{"example.com", def},
		{"www.example.com", def},
		{"sub.domain.example.com", def},
		{"192.168.1.1", def},
		{"::1", def},
	}

	for _, tt := range tests {
		t.Run(tt.host, func(t *testing.T) {
			d := p.dialerForRequest(tt.host)
			if d != tt.expected {
				t.Errorf("dialerForRequest(%q) = %v, want %v", tt.host, d, tt.expected)
			}
		})
	}
}

func TestPerHostBypassSimpleHostnamesDisabled(t *testing.T) {
	bypass := &testDialer{name: "bypass"}
	def := &testDialer{name: "default"}
	p := NewPerHost(def, bypass)

	// Default is disabled, so simple hostnames should NOT bypass
	if d := p.dialerForRequest("localhost"); d != def {
		t.Error("expected default dialer for localhost when bypassSimpleHostnames is disabled")
	}
	if d := p.dialerForRequest("server"); d != def {
		t.Error("expected default dialer for server when bypassSimpleHostnames is disabled")
	}
}

func TestPerHostBypassSimpleHostnamesWithExplicitHosts(t *testing.T) {
	bypass := &testDialer{name: "bypass"}
	def := &testDialer{name: "default"}
	p := NewPerHost(def, bypass)

	// Enable bypass for simple hostnames AND add explicit hosts
	p.SetBypassSimpleHostnames(true)
	p.AddHost("explicit.example.com")

	// Simple hostname should bypass
	if d := p.dialerForRequest("server"); d != bypass {
		t.Error("expected bypass dialer for simple hostname 'server'")
	}

	// Explicit host should also bypass
	if d := p.dialerForRequest("explicit.example.com"); d != bypass {
		t.Error("expected bypass dialer for explicit.example.com")
	}

	// FQDN not in list should use default
	if d := p.dialerForRequest("other.example.com"); d != def {
		t.Error("expected default dialer for other.example.com")
	}
}
