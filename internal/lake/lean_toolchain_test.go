package lake

import (
	"os/exec"
	"testing"
)

// requiresLeanToolchain guards the tests that shell out to a real lake; see the
// twin in internal/mcp for why they are opt-out rather than opt-in.
func requiresLeanToolchain(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping Lean toolchain test in short mode")
	}
	if _, err := exec.LookPath("lake"); err != nil {
		t.Skip("skipping Lean toolchain test: lake is not on PATH")
	}
}
