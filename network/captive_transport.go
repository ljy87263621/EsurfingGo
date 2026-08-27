package network

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	captiveHost        = "connect.rom.miui.com"
	defaultDoHEndpoint = "https://dns.google/resolve"
)

type portalDetectionContextKey struct{}

func withPortalDetection(ctx context.Context) context.Context {
	return context.WithValue(ctx, portalDetectionContextKey{}, true)
}

func isPortalDetection(ctx context.Context) bool {
	value, _ := ctx.Value(portalDetectionContextKey{}).(bool)
	return value
}

func isClashFakeIP(ip net.IP) bool {
	value := ip.To4()
	if value == nil {
		return false
	}
	return value[0] == 198 && (value[1] == 18 || value[1] == 19)
}

type hostResolver interface {
	Lookup(context.Context, string) ([]net.IP, error)
}

type doHResolver struct {
	client   *http.Client
	endpoint string
}

func newDoHResolver(client *http.Client, endpoint string) *doHResolver {
	return &doHResolver{client: client, endpoint: endpoint}
}

func (r *doHResolver) Lookup(ctx context.Context, host string) ([]net.IP, error) {
	endpoint, err := url.Parse(r.endpoint)
	if err != nil {
		return nil, fmt.Errorf("parse DoH endpoint: %w", err)
	}
	query := endpoint.Query()
	query.Set("name", host)
	query.Set("type", "A")
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create DoH request: %w", err)
	}
	req.Header.Set("Accept", "application/dns-json")
	resp, err := r.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("DoH request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("DoH returned HTTP %d", resp.StatusCode)
	}

	var payload struct {
		Status int `json:"Status"`
		Answer []struct {
			Type int    `json:"type"`
			Data string `json:"data"`
		} `json:"Answer"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 64*1024)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode DoH response: %w", err)
	}
	if payload.Status != 0 {
		return nil, fmt.Errorf("DoH DNS status %d", payload.Status)
	}

	ips := make([]net.IP, 0, len(payload.Answer))
	for _, answer := range payload.Answer {
		if answer.Type != 1 {
			continue
		}
		ip := net.ParseIP(strings.TrimSpace(answer.Data)).To4()
		if ip == nil || isClashFakeIP(ip) {
			continue
		}
		ips = append(ips, ip)
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("DoH returned no usable IPv4 address for %s", host)
	}
	return ips, nil
}

func newDefaultDoHResolver(baseTransport ...http.RoundTripper) *doHResolver {
	if len(baseTransport) > 0 && baseTransport[0] != nil {
		return newDoHResolver(&http.Client{
			// A bound authentication transport must also carry the DoH lookup.
			// Otherwise portal detection can resolve through Clash while the
			// subsequent authentication requests use the physical adapter.
			Transport: baseTransport[0],
			Timeout:   6 * time.Second,
		}, defaultDoHEndpoint)
	}

	proxyURL, err := systemProxyURL()
	if err != nil {
		// The environment proxy remains a useful fallback when the Windows
		// proxy registry cannot be read.
		proxyURL = nil
	}

	transport := &http.Transport{Proxy: http.ProxyFromEnvironment}
	if proxyURL != nil {
		transport.Proxy = http.ProxyURL(proxyURL)
	}
	return newDoHResolver(&http.Client{
		Transport: transport,
		Timeout:   6 * time.Second,
	}, defaultDoHEndpoint)
}

type captivePortalTransport struct {
	base     http.RoundTripper
	resolver hostResolver
}

func (t *captivePortalTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if !isPortalDetection(req.Context()) || !strings.EqualFold(req.URL.Hostname(), captiveHost) {
		return t.base.RoundTrip(req)
	}

	ips, err := t.resolver.Lookup(req.Context(), req.URL.Hostname())
	if err != nil {
		log.Printf("[CaptiveTransport] Real DNS lookup failed for %s: %v; falling back to normal resolution", req.URL.Hostname(), err)
		return t.base.RoundTrip(req)
	}

	var lastErr error
	for _, ip := range ips {
		resolvedReq := req.Clone(req.Context())
		resolvedURL := *req.URL
		port := req.URL.Port()
		if port == "" {
			port = "80"
			if strings.EqualFold(req.URL.Scheme, "https") {
				port = "443"
			}
		}
		resolvedURL.Host = net.JoinHostPort(ip.String(), port)
		resolvedReq.URL = &resolvedURL
		resolvedReq.Host = req.Host
		if resolvedReq.Host == "" {
			resolvedReq.Host = req.URL.Host
		}

		resp, err := t.base.RoundTrip(resolvedReq)
		if err == nil {
			return resp, nil
		}
		if resp != nil && resp.Body != nil {
			_ = resp.Body.Close()
		}
		lastErr = err
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no usable address returned for %s", req.URL.Hostname())
	}
	return nil, fmt.Errorf("connect to resolved %s: %w", req.URL.Hostname(), lastErr)
}
