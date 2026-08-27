package network

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestIsClashFakeIPRecognizesFakeIPRange(t *testing.T) {
	for _, test := range []struct {
		name string
		ip   string
		want bool
	}{
		{name: "clash fake ip", ip: "198.18.0.114", want: true},
		{name: "benchmark range upper half", ip: "198.19.10.20", want: true},
		{name: "public address", ip: "8.219.141.49", want: false},
		{name: "private address", ip: "10.10.8.105", want: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := isClashFakeIP(net.ParseIP(test.ip)); got != test.want {
				t.Fatalf("isClashFakeIP(%s) = %t, want %t", test.ip, got, test.want)
			}
		})
	}
}

func TestDoHResolverReturnsRealIPv4AnswersOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Query().Get("name"); got != "connect.rom.miui.com" {
			t.Fatalf("DoH query name = %q", got)
		}
		if got := r.URL.Query().Get("type"); got != "A" {
			t.Fatalf("DoH query type = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"Status":0,"Answer":[{"type":5,"data":"target.example."},{"type":1,"data":"198.18.0.114"},{"type":1,"data":"8.219.141.49"},{"type":28,"data":"2001:db8::1"}]}`)
	}))
	defer server.Close()

	resolver := newDoHResolver(server.Client(), server.URL)
	ips, err := resolver.Lookup(context.Background(), "connect.rom.miui.com")
	if err != nil {
		t.Fatalf("Lookup returned error: %v", err)
	}
	if len(ips) != 1 || !ips[0].Equal(net.ParseIP("8.219.141.49")) {
		t.Fatalf("Lookup returned %v, want [8.219.141.49]", ips)
	}
}

func TestCaptivePortalTransportUsesResolvedIPAndPreservesHost(t *testing.T) {
	var gotURL *url.URL
	var gotHost string
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL
		gotHost = req.Host
		return &http.Response{
			StatusCode: http.StatusNoContent,
			Status:     "204 No Content",
			Body:       io.NopCloser(strings.NewReader("")),
			Request:    req,
		}, nil
	})
	resolver := staticHostResolver{ips: []net.IP{net.ParseIP("8.219.141.49")}}
	transport := &captivePortalTransport{base: base, resolver: resolver}

	req, err := http.NewRequestWithContext(withPortalDetection(context.Background()), http.MethodGet, "http://connect.rom.miui.com/generate_204", nil)
	if err != nil {
		t.Fatalf("NewRequest() error: %v", err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error: %v", err)
	}
	_ = resp.Body.Close()

	if gotURL == nil || gotURL.Host != "8.219.141.49:80" {
		t.Fatalf("resolved URL host = %v, want 8.219.141.49:80", gotURL)
	}
	if gotHost != "connect.rom.miui.com" {
		t.Fatalf("request Host = %q, want connect.rom.miui.com", gotHost)
	}
}

func TestCaptivePortalTransportLeavesAuthenticationRequestsUntouched(t *testing.T) {
	var gotURL *url.URL
	base := roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		gotURL = req.URL
		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    req,
		}, nil
	})
	transport := &captivePortalTransport{
		base:     base,
		resolver: staticHostResolver{ips: []net.IP{net.ParseIP("8.219.141.49")}},
	}
	req, err := http.NewRequest(http.MethodPost, "http://auth.example/login", strings.NewReader("payload"))
	if err != nil {
		t.Fatalf("NewRequest() error: %v", err)
	}
	resp, err := transport.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip() error: %v", err)
	}
	_ = resp.Body.Close()

	if gotURL == nil || gotURL.Host != "auth.example" {
		t.Fatalf("authentication request URL host = %v, want auth.example", gotURL)
	}
}

func TestNewHTTPClientUsesBoundTransportForPortalResolver(t *testing.T) {
	state := &testState{}
	boundTransport := &recordingRoundTripper{}

	client := NewHTTPClient(state, boundTransport)
	redirect, ok := client.Transport.(*redirectInterceptor)
	if !ok {
		t.Fatalf("HTTP client transport = %T, want *redirectInterceptor", client.Transport)
	}
	captive, ok := redirect.inner.(*captivePortalTransport)
	if !ok {
		t.Fatalf("redirect transport inner = %T, want *captivePortalTransport", redirect.inner)
	}
	resolver, ok := captive.resolver.(*doHResolver)
	if !ok {
		t.Fatalf("portal resolver = %T, want *doHResolver", captive.resolver)
	}

	if resolver.client.Transport != boundTransport {
		t.Fatalf("portal resolver transport = %T, want the supplied bound transport %T", resolver.client.Transport, boundTransport)
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

type recordingRoundTripper struct{}

func (*recordingRoundTripper) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, nil
}

type staticHostResolver struct {
	ips []net.IP
}

func (r staticHostResolver) Lookup(context.Context, string) ([]net.IP, error) {
	return r.ips, nil
}
