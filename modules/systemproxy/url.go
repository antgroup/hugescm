package systemproxy

import (
	"net/url"
	"strings"
)

// ParseURL parses a URL string with an optional default scheme.
// If rawURL already contains "://", it's parsed as-is.
// Otherwise, defaultScheme is prepended before parsing.
func ParseURL(rawURL string, defaultScheme string) (*url.URL, error) {
	if strings.Contains(rawURL, "://") {
		return url.Parse(rawURL)
	}
	// Ensure defaultScheme ends with "://"
	if !strings.HasSuffix(defaultScheme, "://") {
		defaultScheme += "://"
	}
	return url.Parse(defaultScheme + rawURL)
}
