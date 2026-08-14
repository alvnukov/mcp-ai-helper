package webfetch

import (
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// The URL policy checks the hostname string, but the connection is made to
// whatever the resolver returned: a DNS name rebinding inward, or an exotic
// loopback encoding, slides past the hostname check. The publicness verdict
// has to be re-checked at dial time, on the actual address.
func TestWebDialGuardRejectsPrivateResolvedAddresses(t *testing.T) {
	guard := webDialGuard(config.WebPolicy{})
	for _, address := range []string{
		"127.0.0.1:8080",
		"10.0.0.5:80",
		"192.168.1.4:80",
		"169.254.169.254:80",
		"[::1]:80",
		"[fe80::1]:80",
		"2130706433:80",
	} {
		if err := guard("tcp", address, nil); err == nil {
			t.Fatalf("dial guard accepted non-public address %s", address)
		}
	}
	if err := guard("tcp", "93.184.216.34:443", nil); err != nil {
		t.Fatalf("dial guard rejected a public address: %v", err)
	}

	// allowed_hosts is explicit user trust; it must keep working for
	// loopback test and intranet setups.
	trusting := webDialGuard(config.WebPolicy{AllowedHosts: []string{"127.0.0.1"}})
	if err := trusting("tcp", "127.0.0.1:8080", nil); err != nil {
		t.Fatalf("dial guard ignored the allowed_hosts trust: %v", err)
	}
}
