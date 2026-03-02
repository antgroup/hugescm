package systemproxy

import (
	"testing"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name          string
		rawURL        string
		defaultScheme string
		wantScheme    string
		wantHost      string
		wantErr       bool
	}{
		{
			name:          "URL with scheme",
			rawURL:        "http://proxy.example.com:8080",
			defaultScheme: "http://",
			wantScheme:    "http",
			wantHost:      "proxy.example.com:8080",
			wantErr:       false,
		},
		{
			name:          "URL without scheme - http default",
			rawURL:        "proxy.example.com:8080",
			defaultScheme: "http://",
			wantScheme:    "http",
			wantHost:      "proxy.example.com:8080",
			wantErr:       false,
		},
		{
			name:          "URL without scheme - https default",
			rawURL:        "proxy.example.com:8443",
			defaultScheme: "https://",
			wantScheme:    "https",
			wantHost:      "proxy.example.com:8443",
			wantErr:       false,
		},
		{
			name:          "URL without scheme - socks5 default",
			rawURL:        "proxy.example.com:1080",
			defaultScheme: "socks5://",
			wantScheme:    "socks5",
			wantHost:      "proxy.example.com:1080",
			wantErr:       false,
		},
		{
			name:          "URL without scheme - default without suffix",
			rawURL:        "proxy.example.com:8080",
			defaultScheme: "http",
			wantScheme:    "http",
			wantHost:      "proxy.example.com:8080",
			wantErr:       false,
		},
		{
			name:          "SOCKS5 URL with authentication",
			rawURL:        "socks5://user:password@proxy.example.com:1080",
			defaultScheme: "http://",
			wantScheme:    "socks5",
			wantHost:      "proxy.example.com:1080",
			wantErr:       false,
		},
		{
			name:          "HTTP URL with authentication",
			rawURL:        "http://user:password@proxy.example.com:8080",
			defaultScheme: "http://",
			wantScheme:    "http",
			wantHost:      "proxy.example.com:8080",
			wantErr:       false,
		},
		{
			name:          "Empty URL",
			rawURL:        "",
			defaultScheme: "http://",
			wantScheme:    "",
			wantHost:      "",
			wantErr:       false, // url.Parse accepts empty string
		},
		{
			name:          "IP address without scheme",
			rawURL:        "127.0.0.1:8080",
			defaultScheme: "http://",
			wantScheme:    "http",
			wantHost:      "127.0.0.1:8080",
			wantErr:       false,
		},
		{
			name:          "IP address with scheme",
			rawURL:        "https://127.0.0.1:8443",
			defaultScheme: "http://",
			wantScheme:    "https",
			wantHost:      "127.0.0.1:8443",
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseURL(tt.rawURL, tt.defaultScheme)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if tt.wantScheme != "" && got.Scheme != tt.wantScheme {
				t.Errorf("ParseURL() scheme = %v, want %v", got.Scheme, tt.wantScheme)
			}
			if tt.wantHost != "" && got.Host != tt.wantHost {
				t.Errorf("ParseURL() host = %v, want %v", got.Host, tt.wantHost)
			}
		})
	}
}
