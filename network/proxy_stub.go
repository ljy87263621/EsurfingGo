//go:build !windows

package network

import (
	"net/url"
	"os"
)

func systemProxyURL() (*url.URL, error) {
	for _, name := range []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"} {
		if raw := os.Getenv(name); raw != "" {
			return parseConfiguredProxy(raw)
		}
	}
	return nil, nil
}
