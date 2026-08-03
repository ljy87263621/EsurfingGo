package network

import (
	"fmt"
	"net/url"
	"strings"
)

func parseConfiguredProxy(raw string) (*url.URL, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("proxy is empty")
	}

	for _, part := range strings.Split(raw, ";") {
		part = strings.TrimSpace(part)
		key, value, found := strings.Cut(part, "=")
		if found {
			key = strings.ToLower(strings.TrimSpace(key))
			if key != "http" && key != "https" && key != "all" {
				continue
			}
			raw = strings.TrimSpace(value)
			break
		}
	}
	if raw == "" {
		return nil, fmt.Errorf("proxy address is empty")
	}
	if !strings.Contains(raw, "://") {
		raw = "http://" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("parse proxy: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("proxy has no host")
	}
	return parsed, nil
}
