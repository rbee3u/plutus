package config

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

type proxyFile struct {
	Mode *string `toml:"mode"`
	URL  *string `toml:"url"`
}

func (p proxyFile) policy() Proxy {
	proxy := Proxy{Mode: defaultProxyMode}
	if p.Mode != nil {
		proxy.Mode = strings.TrimSpace(*p.Mode)
	}
	if p.URL != nil {
		proxy.URL = strings.TrimSpace(*p.URL)
	}
	return proxy
}

// LoadRecordingProxy reads only recording's non-secret TOML proxy policy.
// Only Path and ReadFile are used. The default config/record.toml is optional
// and relative to cwd; an explicitly selected file is required.
func LoadRecordingProxy(opts Options) (Proxy, error) {
	var parsed struct {
		Proxy proxyFile `toml:"proxy"`
	}
	if err := readTOML(&opts, defaultRecordConfigFileName, &parsed); err != nil {
		return Proxy{}, err
	}
	proxy := parsed.Proxy.policy()
	if err := validateProxy(&proxy, "proxy"); err != nil {
		return Proxy{}, err
	}
	return proxy, nil
}

func validateProxy(proxy *Proxy, field string) error {
	switch proxy.Mode {
	case "direct":
		if proxy.URL != "" {
			return fmt.Errorf("%s.url must be empty unless mode is proxy", field)
		}
		return nil
	case "proxy":
		if proxy.URL == "" {
			return fmt.Errorf("%s.url is required when mode is proxy", field)
		}
	case "environment":
		return fmt.Errorf("%s.mode no longer supports environment; set direct or configure proxy with a URL in TOML", field)
	default:
		return fmt.Errorf("%s.mode must be direct or proxy", field)
	}
	// Parsing errors can contain the full input, including credentials. Never return them.
	u, err := url.Parse(proxy.URL)
	if err != nil || u.Opaque != "" {
		return fmt.Errorf("%s.url must be an absolute proxy URL", field)
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return fmt.Errorf("%s.url scheme must be http, https, socks5, or socks5h", field)
	}
	if u.User != nil {
		return fmt.Errorf("%s.url must not contain username or password", field)
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || host == "" || strings.ContainsAny(host, " \t\r\n") {
		return fmt.Errorf("%s.url must contain a host and explicit port", field)
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return fmt.Errorf("%s.url port must be between 1 and 65535", field)
	}
	if (u.Path != "" && u.Path != "/") || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(proxy.URL, "#") {
		return fmt.Errorf("%s.url must not contain a non-root path, query, or fragment", field)
	}
	return nil
}
