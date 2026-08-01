package main

import "testing"

func TestVersionLine(t *testing.T) {
	original := version
	t.Cleanup(func() { version = original })
	version = "1.0.0"
	if got, want := versionLine(), "mcp-ai-helper 1.0.0"; got != want {
		t.Fatalf("versionLine() = %q, want %q", got, want)
	}
}
