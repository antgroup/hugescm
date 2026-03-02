package systemproxy

import (
	"net"
	"net/url"
	"strings"
)

func newDialer(proxyURL *url.URL, forward *net.Dialer, noProxy string) Dialer {
	p, err := NewDialerFromURL(proxyURL, forward)
	if err != nil {
		return forward
	}
	perHost := NewPerHost(p, forward)
	perHost.AddFromString(noProxy)
	return perHost
}

func newDialerForHosts(proxyURL *url.URL, forward *net.Dialer, hosts []string, bypassSimpleHostnames bool) Dialer {
	pd, err := NewDialerFromURL(proxyURL, forward)
	if err != nil {
		return forward
	}
	p := NewPerHost(pd, forward)
	p.SetBypassSimpleHostnames(bypassSimpleHostnames)
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if strings.Contains(host, "/") {
			// We assume that it's a CIDR address like 127.0.0.0/8
			if _, net, err := net.ParseCIDR(host); err == nil {
				p.AddNetwork(net)
			}
			continue
		}
		if ip := net.ParseIP(host); ip != nil {
			p.AddIP(ip)
			continue
		}
		if strings.HasPrefix(host, "*.") {
			p.AddZone(host[1:])
			continue
		}
		p.AddHost(host)
	}
	return p
}
