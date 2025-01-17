//go:build darwin

package systemproxy

import (
	"context"
	"errors"
	"net"
	"net/url"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	"golang.org/x/net/http/httpproxy"
)

type MacProxySettings struct {
	ExceptionsList         []string
	ExcludeSimpleHostnames bool
	FTPPassive             bool
	// HTTP
	HTTPEnable bool
	HTTPPort   string
	HTTPProxy  string
	HTTPUser   string
	// HTTPS
	HTTPSEnable bool
	HTTPSPort   string
	HTTPSProxy  string
	HTTPSUser   string
	// SOCKS
	SOCKSEnable bool
	SOCKSPort   string
	SOCKSProxy  string
	SOCKSUser   string
	//
	ProxyAutoConfigEnable    bool
	ProxyAutoDiscoveryEnable bool
	ProxyAutoConfigURLString string
}

func joinHostPort(u, p string) string {
	if len(p) != 0 {
		return net.JoinHostPort(u, p)
	}
	return u
}

func joinProxyURL(defaultScheme, host, port, user string) *url.URL {
	u := &url.URL{
		Scheme: defaultScheme,
		Host:   joinHostPort(host, port),
	}
	if len(user) != 0 {
		u.User = url.User(user)
	}
	return u
}

type section map[string]any

type arrayItem struct {
	i string
	v string
}

func (se section) boolean(name string) bool {
	v, ok := se[name]
	if !ok {
		return false
	}
	s, ok := v.(string)
	if !ok {
		return false
	}
	return s == "1"
}

func (se section) string(name string) string {
	v, ok := se[name]
	if !ok {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

func (se section) array(name string) []string {
	o, ok := se[name]
	if !ok {
		return nil
	}
	sub, ok := o.(section)
	if !ok {
		return nil
	}
	items := make([]*arrayItem, 0, len(sub))
	for k, v := range sub {
		s, ok := v.(string)
		if !ok {
			continue
		}
		items = append(items, &arrayItem{i: k, v: s})
	}
	slices.SortFunc(items, func(a, b *arrayItem) int {
		return strings.Compare(a.i, b.i)
	})
	arr := make([]string, 0, len(items))
	for _, i := range items {
		arr = append(arr, i.v)
	}
	return arr
}

func parseOut(out string) section {
	lines := strings.Split(out, "\n")
	var cur section
	stack := make([]section, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		lastField := fields[len(fields)-1]
		firstField := fields[0]
		if lastField == "}" {
			if len(stack) == 0 {
				break
			}
			cur = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			continue
		}
		if lastField == "{" {
			newObj := make(section)
			if cur != nil {
				stack = append(stack, cur)
				cur[firstField] = newObj
			}
			cur = newObj
			continue
		}
		if len(fields) == 3 && fields[1] == ":" {
			cur[firstField] = lastField
		}
	}
	return cur
}

func findSystemProxy() (*MacProxySettings, error) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), time.Second)
	defer cancelCtx()
	cmd := exec.CommandContext(ctx, "scutil", "--proxy")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	se := parseOut(string(out))
	if se == nil {
		return nil, errors.New("no scutil proxy settings")
	}
	return &MacProxySettings{
		ExceptionsList:           se.array("ExceptionsList"),
		ExcludeSimpleHostnames:   se.boolean("ExcludeSimpleHostnames"),
		FTPPassive:               se.boolean("FTPPassive"),
		HTTPEnable:               se.boolean("HTTPEnable"),
		HTTPPort:                 se.string("HTTPPort"),
		HTTPProxy:                se.string("HTTPProxy"),
		HTTPUser:                 se.string("HTTPSUser"),
		HTTPSEnable:              se.boolean("HTTPSEnable"),
		HTTPSPort:                se.string("HTTPSPort"),
		HTTPSProxy:               se.string("HTTPSProxy"),
		HTTPSUser:                se.string("HTTPSUser"),
		SOCKSEnable:              se.boolean("SOCKSEnable"),
		SOCKSPort:                se.string("SOCKSPort"),
		SOCKSProxy:               se.string("SOCKSProxy"),
		SOCKSUser:                se.string("SOCKSUser"),
		ProxyAutoConfigEnable:    se.boolean("ProxyAutoConfigEnable"),
		ProxyAutoDiscoveryEnable: se.boolean("ProxyAutoDiscoveryEnable"),
		ProxyAutoConfigURLString: se.string("ProxyAutoConfigURLString"),
	}, nil
}

// SOCKS5 support
func newSystemDialer(forward *net.Dialer) Dialer {
	systemProxy, err := findSystemProxy()
	if err != nil {
		return forward
	}
	if systemProxy.SOCKSEnable && len(systemProxy.SOCKSProxy) != 0 {
		proxyURL := joinProxyURL("socks5", systemProxy.SOCKSProxy, systemProxy.SOCKSPort, systemProxy.SOCKSUser)
		return newDialerForHosts(proxyURL, forward, systemProxy.ExceptionsList)
	}
	return forward
}

func NewSystemDialer(forward *net.Dialer) Dialer {
	allProxy := getEnvAny("ALL_PROXY", "all_proxy") // follow ALL_PROXY
	noProxy := getEnvAny("NO_PROXY", "no_proxy")
	if len(allProxy) == 0 {
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
	systemProxy, err := findSystemProxy()
	if err != nil {
		return cfg
	}
	if len(cfg.NoProxy) == 0 {
		cfg.NoProxy = strings.Join(systemProxy.ExceptionsList, ",")
	}
	if systemProxy.SOCKSEnable && len(systemProxy.SOCKSProxy) != 0 {
		proxyURL := joinProxyURL("socks5", systemProxy.SOCKSProxy, systemProxy.SOCKSPort, systemProxy.SOCKSUser).String()
		if len(cfg.HTTPProxy) == 0 {
			cfg.HTTPProxy = proxyURL
		}
		if len(cfg.HTTPSProxy) == 0 {
			cfg.HTTPSProxy = proxyURL
		}
		return cfg
	}
	if len(cfg.HTTPProxy) == 0 && systemProxy.HTTPEnable && len(systemProxy.HTTPProxy) != 0 {
		proxyURL := joinProxyURL("http", systemProxy.HTTPProxy, systemProxy.HTTPPort, systemProxy.HTTPUser).String()
		cfg.HTTPProxy = proxyURL
	}
	if len(cfg.HTTPSProxy) == 0 && systemProxy.HTTPSEnable && len(systemProxy.HTTPSProxy) != 0 {
		proxyURL := joinProxyURL("https", systemProxy.HTTPSProxy, systemProxy.HTTPSPort, systemProxy.HTTPSUser).String()
		cfg.HTTPSProxy = proxyURL
	}
	return cfg
}
