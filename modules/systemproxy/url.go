package systemproxy

import (
	"net/url"
	"strings"
)

func ParseURL(rawURL string, schemePrefix string) (*url.URL, error) {
	if strings.Contains(rawURL, "://") {
		return url.Parse(rawURL)
	}
	return url.Parse(schemePrefix + rawURL)
}
