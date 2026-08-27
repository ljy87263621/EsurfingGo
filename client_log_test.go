package main

import "testing"

func TestTicketURLWithClientParams(t *testing.T) {
	got := ticketURLWithClientParams("http://portal/ticket.cgi", "202.103.138.17", "10.10.8.105")
	want := "http://portal/ticket.cgi?clientip=1&wlanacip=202.103.138.17&wlanuserip=10.10.8.105"
	if got != want {
		t.Fatalf("ticket URL = %q, want %q", got, want)
	}
}

func TestPreviewForLogOnlyReportsLength(t *testing.T) {
	preview := previewForLog("sensitive-token-material")

	if preview != "24 bytes" {
		t.Fatalf("preview = %q, want byte length only", preview)
	}
}
