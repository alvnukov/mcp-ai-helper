package mcp

import "testing"

// The handshake version used to be a hardcoded 0.1.0 that drifted from every
// release tag. A release build sets buildVersion through ldflags; anything
// else must fall back to build info instead of a constant.
func TestServerVersion(t *testing.T) {
	saved := buildVersion
	defer func() { buildVersion = saved }()

	buildVersion = "9.9.9"
	if got := serverVersion(); got != "9.9.9" {
		t.Fatalf("release override ignored: serverVersion() = %q", got)
	}

	buildVersion = ""
	if got := serverVersion(); got == "" || got == "0.1.0" {
		t.Fatalf("fallback must come from build info, got %q", got)
	}
}
