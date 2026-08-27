package main

import (
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAuthorizationDoesNotReportUnknownAlgorithmForEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newSessionErrorTestClient(server.URL)
	logs := captureSessionErrorLogs(func() {
		for i := 0; i < 6; i++ {
			client.authorization()
		}
	})

	if strings.Contains(logs, "Unable to find algorithm implementation") {
		t.Fatalf("empty session response was reported as unknown algorithm: %s", logs)
	}
	if !strings.Contains(logs, "empty response") {
		t.Fatalf("empty session response was not explained in logs: %s", logs)
	}
}

func TestAuthorizationReportsUnsupportedAlgorithmOnlyAfterParsingIt(t *testing.T) {
	const unsupportedAlgoID = "11111111-1111-1111-1111-111111111111"
	zsm := append([]byte{0, 0, 0, 0, '$'}, []byte(unsupportedAlgoID)...)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zsm)
	}))
	defer server.Close()

	client := newSessionErrorTestClient(server.URL)
	logs := captureSessionErrorLogs(func() {
		for i := 0; i < 6; i++ {
			client.authorization()
		}
	})

	if !strings.Contains(strings.ToLower(logs), "unsupported session algorithm") {
		t.Fatalf("unsupported algorithm was not identified in logs: %s", logs)
	}
	if !strings.Contains(logs, unsupportedAlgoID) {
		t.Fatalf("unsupported algorithm ID was not included in logs: %s", logs)
	}
}

func newSessionErrorTestClient(url string) *Client {
	states := NewStates()
	states.RefreshStates()
	states.SetTicketURL(url)
	return &Client{
		options:    Options{LoginUser: "user", LoginPassword: "password", SMSCode: "provided"},
		states:     states,
		session:    NewSession(),
		httpClient: &http.Client{},
	}
}

func captureSessionErrorLogs(fn func()) string {
	var logs strings.Builder
	oldWriter := log.Writer()
	log.SetOutput(io.MultiWriter(oldWriter, &logs))
	defer log.SetOutput(oldWriter)
	fn()
	return logs.String()
}
