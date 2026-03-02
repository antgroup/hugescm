//go:build windows

package systemproxy

import (
	"net"
	"os"
	"strings"

	"golang.org/x/net/http/httpproxy"
	"golang.org/x/sys/windows/registry"
)

type windowsProxyConfig struct {
	ProxyServer   string
	ProxyOverride string
	ProxyEnable   uint64
	AutoConfigURL string
}

// parseProxyServer parses Windows proxy server string into a map
// Windows proxy format: "http=proxy.example.com:8080;https=proxy.example.com:8443;socks=proxy.example.com:1080"
// or just "proxy.example.com:8080" for all protocols
// Note: Keys are normalized to lowercase for case-insensitive matching
func parseProxyServer(proxyServer string) map[string]string {
	protocol := make(map[string]string)
	for s := range strings.SplitSeq(proxyServer, ";") {
		if s == "" {
			continue
		}
		pair := strings.SplitN(s, "=", 2)
		if len(pair) > 1 {
			// Normalize key to lowercase for case-insensitive matching
			protocol[strings.ToLower(pair[0])] = pair[1]
		} else {
			protocol[""] = pair[0]
		}
	}
	return protocol
}

// getProtocolAny returns the first matching protocol value from the map
// Keys are checked in order, returns empty string if none found
func getProtocolAny(protocol map[string]string, keys ...string) string {
	for _, key := range keys {
		if v, ok := protocol[key]; ok {
			return v
		}
	}
	return ""
}

func fromWindowsProxy() (values windowsProxyConfig, err error) {
	var proxySettingsPerUser uint64 = 1 // 1 is the default value to consider current user
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, `Software\Policies\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err == nil {
		// We had used the below variable tempPrxUsrSettings, because the Golang method GetIntegerValue
		// sets the value to zero even it fails.
		tempPrxUsrSettings, _, err := k.GetIntegerValue("ProxySettingsPerUser")
		if err == nil {
			// consider the value of tempPrxUsrSettings if it is a success
			proxySettingsPerUser = tempPrxUsrSettings
		}
		_ = k.Close()
	}
	var hkey registry.Key
	if proxySettingsPerUser == 0 {
		hkey = registry.LOCAL_MACHINE
	} else {
		hkey = registry.CURRENT_USER
	}
	k, err = registry.OpenKey(hkey, `Software\Microsoft\Windows\CurrentVersion\Internet Settings`, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer k.Close() // nolint

	values.ProxyServer, _, err = k.GetStringValue("ProxyServer")
	if err != nil && err != registry.ErrNotExist {
		return
	}
	values.ProxyOverride, _, err = k.GetStringValue("ProxyOverride")
	if err != nil && err != registry.ErrNotExist {
		return
	}

	values.ProxyEnable, _, err = k.GetIntegerValue("ProxyEnable")
	if err != nil && err != registry.ErrNotExist {
		return
	}

	values.AutoConfigURL, _, err = k.GetStringValue("AutoConfigURL")
	if err != nil && err != registry.ErrNotExist {
		return
	}
	err = nil
	return
}

// parseProxyOverride parses Windows ProxyOverride string and handles <local> special tag
// Windows format: "localhost;127.0.0.1;<local>;*.example.com"
// <local> means bypass proxy for all local addresses (simple hostnames without dots)
func parseProxyOverride(proxyOverride string) (hosts []string, bypassLocal bool) {
	for item := range strings.SplitSeq(proxyOverride, ";") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		// <local> is a special tag in Windows that means bypass proxy for:
		// - Hostnames without dots (e.g., "server", "localhost")
		// - Does NOT include FQDNs or IP addresses
		if strings.EqualFold(item, "<local>") {
			bypassLocal = true
			continue
		}
		hosts = append(hosts, item)
	}
	return hosts, bypassLocal
}

func newSystemDialer(forward *net.Dialer) Dialer {
	values, err := fromWindowsProxy()
	if err != nil || values.ProxyEnable < 1 {
		// not config or disabled
		return forward
	}
	noProxy, bypassLocal := parseProxyOverride(values.ProxyOverride)
	protocol := parseProxyServer(values.ProxyServer)

	// Priority: socks proxy > default proxy
	// SOCKS proxy is preferred for general dialing as it supports more protocols (TCP, UDP, etc.)
	// Default proxy (without protocol prefix) is typically HTTP proxy
	if socksProxy := getProtocolAny(protocol, "socks"); socksProxy != "" {
		if proxyURL, err := ParseURL(socksProxy, "socks5://"); err == nil {
			return newDialerForHosts(proxyURL, forward, noProxy, bypassLocal)
		}
	}
	if defaultProxy := getProtocolAny(protocol, ""); defaultProxy != "" {
		if proxyURL, err := ParseURL(defaultProxy, "http://"); err == nil {
			return newDialerForHosts(proxyURL, forward, noProxy, bypassLocal)
		}
	}
	return forward
}

func NewSystemDialer(forward *net.Dialer) Dialer {
	allProxy := getEnvAny("ALL_PROXY", "all_proxy")
	noProxy := getEnvAny("NO_PROXY", "no_proxy")
	if allProxy == "" {
		return newSystemDialer(forward)
	}
	proxyURL, err := ParseURL(allProxy, "http://")
	if err != nil {
		return forward
	}
	return newDialer(proxyURL, forward, noProxy)
}

func systemProxyConfig() *httpproxy.Config {
	cfg := &httpproxy.Config{
		HTTPProxy:  getEnvAny("HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"),
		HTTPSProxy: getEnvAny("HTTPS_PROXY", "https_proxy", "ALL_PROXY", "all_proxy"),
		NoProxy:    getEnvAny("NO_PROXY", "no_proxy"),
		CGI:        os.Getenv("REQUEST_METHOD") != "",
	}
	if cfg.HTTPProxy != "" || cfg.HTTPSProxy != "" {
		return cfg
	}
	values, err := fromWindowsProxy()
	if err != nil || values.ProxyEnable < 1 {
		// not config or disabled
		return cfg
	}
	protocol := parseProxyServer(values.ProxyServer)
	if cfg.NoProxy == "" {
		// Parse ProxyOverride and convert to standard NoProxy format
		noProxyHosts, bypassLocal := parseProxyOverride(values.ProxyOverride)
		var noProxyParts []string
		for _, host := range noProxyHosts {
			// Convert Windows format to standard format
			noProxyParts = append(noProxyParts, host)
		}
		if bypassLocal {
			// For <local>, add common local patterns
			// Note: httpproxy.Config doesn't natively support "simple hostname" concept,
			// so we add common local addresses
			noProxyParts = append(noProxyParts, "localhost", "127.0.0.1", "::1")
		}
		cfg.NoProxy = strings.Join(noProxyParts, ",")
	}
	// Windows proxy priority: protocol-specific proxy takes precedence over SOCKS
	// HTTP requests use HTTP proxy, HTTPS requests use HTTPS proxy
	// SOCKS is only used as fallback when no protocol-specific proxy is configured
	// Reference: WinHTTP proxy configuration behavior

	// Configure HTTP proxy
	if cfg.HTTPProxy == "" {
		if httpProxy := getProtocolAny(protocol, "http"); httpProxy != "" {
			cfg.HTTPProxy = httpProxy
		} else if socksProxy := getProtocolAny(protocol, "socks"); socksProxy != "" {
			// Fallback to SOCKS if no HTTP proxy configured
			cfg.HTTPProxy = "socks5://" + socksProxy
		} else if defaultProxy := getProtocolAny(protocol, ""); defaultProxy != "" {
			cfg.HTTPProxy = defaultProxy
		}
	}

	// Configure HTTPS proxy
	if cfg.HTTPSProxy == "" {
		if httpsProxy := getProtocolAny(protocol, "https"); httpsProxy != "" {
			cfg.HTTPSProxy = httpsProxy
		} else if socksProxy := getProtocolAny(protocol, "socks"); socksProxy != "" {
			// Fallback to SOCKS if no HTTPS proxy configured
			cfg.HTTPSProxy = "socks5://" + socksProxy
		} else if defaultProxy := getProtocolAny(protocol, ""); defaultProxy != "" {
			cfg.HTTPSProxy = defaultProxy
		}
	}

	return cfg
}
