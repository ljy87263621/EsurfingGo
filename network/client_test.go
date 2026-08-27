package network

import (
	"bytes"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type testState struct {
	clientID string
	algoID   string
	schoolID string
	domain   string
	area     string
}

func (s *testState) GetClientID() string { return s.clientID }
func (s *testState) GetAlgoID() string   { return s.algoID }
func (s *testState) GetSchoolID() string { return s.schoolID }
func (s *testState) GetDomain() string   { return s.domain }
func (s *testState) GetArea() string     { return s.area }
func (s *testState) SetArea(v string)    { s.area = v }
func (s *testState) SetSchoolID(v string) {
	s.schoolID = v
}
func (s *testState) SetDomain(v string) { s.domain = v }

func TestPostRedirectPreservesBody(t *testing.T) {
	state := &testState{
		clientID: "cid-test",
		algoID:   "00000000-0000-0000-0000-000000000000",
	}

	var server *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/start", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		_ = r.Body.Close()
		w.Header().Set("Location", server.URL+"/auth")
		w.WriteHeader(http.StatusFound)
	})

	var gotMethod string
	var gotBody string
	mux.HandleFunc("/auth", func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		body, _ := io.ReadAll(r.Body)
		_ = r.Body.Close()
		gotBody = string(body)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	})

	server = httptest.NewServer(mux)
	defer server.Close()

	client := NewHTTPClient(state)
	payload := "hello=world&x=1"

	respBody, err := Post(client, server.URL+"/start", payload, state, nil)
	if err != nil {
		t.Fatalf("Post returned error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Fatalf("redirected request method = %q, want %q", gotMethod, http.MethodPost)
	}
	if gotBody != payload {
		t.Fatalf("redirected request body = %q, want %q", gotBody, payload)
	}
	if respBody != payload {
		t.Fatalf("Post response body = %q, want %q", respBody, payload)
	}
}

func TestPostNonSuccessLogOmitsResponseBody(t *testing.T) {
	state := &testState{
		clientID: "cid-test",
		algoID:   "00000000-0000-0000-0000-000000000000",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("passwd=secret-pass&ticket=ticket-value"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldOutput := log.Writer()
	log.SetOutput(&buf)
	defer log.SetOutput(oldOutput)

	_, err := Post(NewHTTPClient(state), server.URL+"?token=query-secret", "payload", state, nil)
	if err != nil {
		t.Fatalf("Post returned error: %v", err)
	}

	logs := buf.String()
	for _, secret := range []string{"secret-pass", "ticket-value"} {
		if strings.Contains(logs, secret) {
			t.Fatalf("log leaked response body secret %q: %s", secret, logs)
		}
	}
	if strings.Contains(logs, "query-secret") {
		t.Fatalf("log leaked URL query secret: %s", logs)
	}
	if !strings.Contains(logs, "body=38 bytes") {
		t.Fatalf("log did not report response body length: %s", logs)
	}
}

func TestSafeURLForLogStripsQueryAndFragment(t *testing.T) {
	got := safeURLForLog("https://portal.example/auth?token=secret#frag")
	want := "https://portal.example/auth"
	if got != want {
		t.Fatalf("safeURLForLog() = %q, want %q", got, want)
	}
}

func TestPostRawEmptyResponseReturnsError(t *testing.T) {
	state := &testState{
		clientID: "cid-test",
		algoID:   "00000000-0000-0000-0000-000000000000",
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	_, err := PostRaw(NewHTTPClient(state), server.URL+"/ticket.cgi?token=secret", "payload", state)
	if err == nil {
		t.Fatal("PostRaw returned nil error for an empty response")
	}

	var emptyErr *EmptyResponseError
	if !errors.As(err, &emptyErr) {
		t.Fatalf("PostRaw error = %T %v, want EmptyResponseError", err, err)
	}
	if emptyErr.StatusCode != http.StatusOK {
		t.Fatalf("empty response status = %d, want %d", emptyErr.StatusCode, http.StatusOK)
	}
	if strings.Contains(emptyErr.FinalURL, "token=secret") {
		t.Fatalf("empty response error leaked URL query: %q", emptyErr.FinalURL)
	}
}

func TestPostRawOmitsPlaceholderAlgoID(t *testing.T) {
	state := &testState{clientID: "cid-test", algoID: "00000000-0000-0000-0000-000000000000"}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Algo-ID"); got != "" {
			t.Errorf("placeholder Algo-ID header = %q, want omitted", got)
		}
		_, _ = w.Write([]byte("zsm"))
	}))
	defer server.Close()
	if _, err := PostRaw(NewHTTPClient(state), server.URL+"/ticket.cgi", "payload", state); err != nil {
		t.Fatalf("PostRaw returned error: %v", err)
	}
}
