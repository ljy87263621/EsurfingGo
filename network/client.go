package network

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	neturl "net/url"
	"strings"
	"time"
)

const (
	// Use the maintained Linux desktop channel. The native Windows channel
	// currently negotiates an unsupported AutoZSM algorithm on this portal.
	userAgent     = "CCTP/Linux64/1003"
	requestAccept = "text/html,text/xml,application/xhtml+xml,application/x-javascript,*/*"
)

// EmptyResponseError indicates that the server completed the HTTP request but
// returned no protocol payload. The URL is sanitized before it is stored.
type EmptyResponseError struct {
	StatusCode int
	FinalURL   string
}

func (e *EmptyResponseError) Error() string {
	return fmt.Sprintf("empty response body (status=%d, finalURL=%s)", e.StatusCode, e.FinalURL)
}

// StateProvider provides the global state values needed for requests.
type StateProvider interface {
	GetClientID() string
	GetAlgoID() string
	GetSchoolID() string
	GetDomain() string
	GetArea() string
	SetArea(string)
	SetSchoolID(string)
	SetDomain(string)
}

// redirectInterceptor is an http.RoundTripper that handles redirects with custom headers.
type redirectInterceptor struct {
	inner    http.RoundTripper
	state    StateProvider
	maxRedir int
}

func (r *redirectInterceptor) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.inner.RoundTrip(req)
	if err != nil {
		log.Printf("[Redirect] RoundTrip error for %s %s: %v", req.Method, safeURLForLog(req.URL.String()), err)
		return resp, err
	}

	redirectCount := 0
	for isRedirect(resp.StatusCode) && redirectCount < r.maxRedir {
		redirectCount++

		// Extract routing headers (check both CDC-* and plain variants)
		area := getCDCHeader(resp, "Area")
		if area != "" {
			log.Printf("[Redirect] Header area=%s", area)
			r.state.SetArea(area)
		}
		schoolID := getCDCHeader(resp, "SchoolId")
		if schoolID != "" {
			log.Printf("[Redirect] Header schoolid=%s", schoolID)
			r.state.SetSchoolID(schoolID)
		}
		domain := getCDCHeader(resp, "Domain")
		if domain != "" {
			log.Printf("[Redirect] Header domain=%s", domain)
			r.state.SetDomain(domain)
		}

		location := resp.Header.Get("Location")
		log.Printf("[Redirect] #%d %d -> %s", redirectCount, resp.StatusCode, safeURLForLog(location))
		if location == "" {
			log.Println("[Redirect] Empty Location header, stopping redirect chain")
			break
		}

		nextURL, err := req.URL.Parse(location)
		if err != nil {
			resp.Body.Close()
			return nil, err
		}

		// Close old response body
		resp.Body.Close()

		var newBody io.ReadCloser
		if req.Body != nil {
			newBody, err = cloneRequestBody(req)
			if err != nil {
				return nil, err
			}
		}

		newReq, err := http.NewRequestWithContext(req.Context(), req.Method, nextURL.String(), newBody)
		if err != nil {
			if newBody != nil {
				newBody.Close()
			}
			return nil, err
		}
		newReq.GetBody = req.GetBody
		newReq.ContentLength = req.ContentLength
		// Copy headers
		for k, v := range req.Header {
			newReq.Header[k] = v
		}
		// Add routing headers if not present
		if r.state.GetSchoolID() != "" && newReq.Header.Get("CDC-SchoolId") == "" {
			newReq.Header.Set("CDC-SchoolId", r.state.GetSchoolID())
		}
		if r.state.GetDomain() != "" && newReq.Header.Get("CDC-Domain") == "" {
			newReq.Header.Set("CDC-Domain", r.state.GetDomain())
		}
		if r.state.GetArea() != "" && newReq.Header.Get("CDC-Area") == "" {
			newReq.Header.Set("CDC-Area", r.state.GetArea())
		}

		resp, err = r.inner.RoundTrip(newReq)
		if err != nil {
			return nil, err
		}
		req = newReq
	}
	return resp, nil
}

// cloneRequestBody creates a fresh body reader for redirected requests.
func cloneRequestBody(req *http.Request) (io.ReadCloser, error) {
	if req.Body == nil {
		return nil, nil
	}
	if req.GetBody != nil {
		return req.GetBody()
	}

	buf, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(buf))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(buf)), nil
	}
	req.ContentLength = int64(len(buf))
	return req.GetBody()
}

func isRedirect(code int) bool {
	return code == 301 || code == 302 || code == 303 || code == 307 || code == 308
}

// NewHTTPClient creates a configured HTTP client with redirect handling.
// An optional baseTransport can be provided to override the default transport
// (e.g., a transport bound to a specific network interface).
func NewHTTPClient(state StateProvider, baseTransport ...http.RoundTripper) *http.Client {
	inner := http.RoundTripper(http.DefaultTransport)
	if len(baseTransport) > 0 && baseTransport[0] != nil {
		inner = baseTransport[0]
	}
	inner = &captivePortalTransport{
		base:     inner,
		resolver: newDefaultDoHResolver(baseTransport...),
	}
	transport := &redirectInterceptor{
		inner:    inner,
		state:    state,
		maxRedir: 5,
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Printf("[HTTPClient] Failed to create cookie jar: %v", err)
	}
	return &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
		Jar:       jar,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Post sends a POST request with authentication headers.
func Post(client *http.Client, url, data string, state StateProvider, extraHeaders map[string]string) (string, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(data))
	if err != nil {
		log.Printf("[Post] Failed to create request for %s: %v", safeURLForLog(url), err)
		return "", fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", requestAccept)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	// MD5 checksum
	hash := md5.Sum([]byte(data))
	req.Header.Set("CDC-Checksum", hex.EncodeToString(hash[:]))
	req.Header.Set("Client-ID", state.GetClientID())
	// The initial ticket.cgi negotiation uses an all-zero algorithm ID. Newer
	// portals reject that placeholder when it is sent as a header; omit it
	// until the server returns the negotiated algorithm.
	if algoID := strings.TrimSpace(state.GetAlgoID()); algoID != "" && !isZeroAlgoID(algoID) {
		req.Header.Set("Algo-ID", algoID)
	}

	if v := state.GetSchoolID(); v != "" {
		req.Header.Set("CDC-SchoolId", v)
	}
	if v := state.GetDomain(); v != "" {
		req.Header.Set("CDC-Domain", v)
	}
	if v := state.GetArea(); v != "" {
		req.Header.Set("CDC-Area", v)
	}

	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[Post] Request to %s failed: %v", safeURLForLog(url), err)
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[Post] Failed to read body from %s: %v", safeURLForLog(url), err)
		return "", fmt.Errorf("read body: %w", err)
	}
	if len(body) == 0 {
		finalURL := safeURLForLog(url)
		if resp.Request != nil && resp.Request.URL != nil {
			finalURL = safeURLForLog(resp.Request.URL.String())
		}
		log.Printf("[Post] Empty response body from %s (status=%d, finalURL=%s)", safeURLForLog(url), resp.StatusCode, safeURLForLog(finalURL))
		return "", &EmptyResponseError{StatusCode: resp.StatusCode, FinalURL: finalURL}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[Post] %s returned %d: body=%d bytes", safeURLForLog(url), resp.StatusCode, len(body))
	}

	return string(body), nil
}

// PostRaw sends a POST request and returns raw bytes (for binary responses like ZSM).
func PostRaw(client *http.Client, url, data string, state StateProvider) ([]byte, error) {
	req, err := http.NewRequest("POST", url, strings.NewReader(data))
	if err != nil {
		log.Printf("[PostRaw] Failed to create request for %s: %v", safeURLForLog(url), err)
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", requestAccept)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	hash := md5.Sum([]byte(data))
	req.Header.Set("CDC-Checksum", hex.EncodeToString(hash[:]))
	req.Header.Set("Client-ID", state.GetClientID())
	if algoID := strings.TrimSpace(state.GetAlgoID()); algoID != "" && !isZeroAlgoID(algoID) {
		req.Header.Set("Algo-ID", algoID)
	}

	if v := state.GetSchoolID(); v != "" {
		req.Header.Set("CDC-SchoolId", v)
	}
	if v := state.GetDomain(); v != "" {
		req.Header.Set("CDC-Domain", v)
	}
	if v := state.GetArea(); v != "" {
		req.Header.Set("CDC-Area", v)
	}

	resp, err := client.Do(req)
	if err != nil {
		log.Printf("[PostRaw] Request to %s failed: %v", safeURLForLog(url), err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[PostRaw] %s returned status %d", safeURLForLog(url), resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[PostRaw] Failed to read body from %s: %v", safeURLForLog(url), err)
		return nil, fmt.Errorf("read body: %w", err)
	}
	if len(body) == 0 {
		finalURL := safeURLForLog(url)
		if resp.Request != nil && resp.Request.URL != nil {
			finalURL = safeURLForLog(resp.Request.URL.String())
		}
		log.Printf("[PostRaw] Empty response body from %s (status=%d, finalURL=%s)", safeURLForLog(url), resp.StatusCode, safeURLForLog(finalURL))
		return nil, &EmptyResponseError{StatusCode: resp.StatusCode, FinalURL: finalURL}
	}

	return body, nil
}

func safeURLForLog(rawURL string) string {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return "<invalid URL>"
	}
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func isZeroAlgoID(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "00000000-0000-0000-0000-000000000000")
}
