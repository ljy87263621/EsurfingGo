package main

import (
	"testing"
)

func TestPreviewForLogOnlyReportsLength(t *testing.T) {
	preview := previewForLog("sensitive-token-material")

	if preview != "24 bytes" {
		t.Fatalf("preview = %q, want byte length only", preview)
	}
}
