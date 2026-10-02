package config

import (
	"errors"
	"net"
	"net/url"
	"strconv"
	"strings"
)

func validateProxy(proxy *Proxy) error {
	switch proxy.Mode {
	case "environment", "direct":
		if proxy.URL != "" {
			return errors.New("collection.proxy.url must be empty unless mode is proxy")
		}
		return nil
	case "proxy":
		if proxy.URL == "" {
			return errors.New("collection.proxy.url is required when mode is proxy")
		}
	default:
		return errors.New("collection.proxy.mode must be environment, direct, or proxy")
	}
	// Parsing errors can contain the full input, including credentials. Never return them.
	u, err := url.Parse(proxy.URL)
	if err != nil || u.Opaque != "" {
		return errors.New("collection.proxy.url must be an absolute proxy URL")
	}
	switch u.Scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return errors.New("collection.proxy.url scheme must be http, https, socks5, or socks5h")
	}
	if u.User != nil {
		return errors.New("collection.proxy.url must not contain username or password")
	}
	host, port, err := net.SplitHostPort(u.Host)
	if err != nil || host == "" || strings.ContainsAny(host, " \t\r\n") {
		return errors.New("collection.proxy.url must contain a host and explicit port")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 1 || n > 65535 {
		return errors.New("collection.proxy.url port must be between 1 and 65535")
	}
	if (u.Path != "" && u.Path != "/") || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(proxy.URL, "#") {
		return errors.New("collection.proxy.url must not contain a non-root path, query, or fragment")
	}
	return nil
}
