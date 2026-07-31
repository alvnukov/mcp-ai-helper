package mcp

import (
	"os/exec"
	"testing"
)

// requiresLeanToolchain guards the tests that drive a real Lean workspace.
//
// Those tests are the honest ones — the task registry lives in Lean, and nothing
// short of running lake proves a mutation round-trips. They are also the slow
// ones: every mutation resets the shared `lake serve`, so each one pays for a
// cold server start, and the end-to-end test alone runs for minutes.
//
// So they are opt-out rather than opt-in: `go test ./...` runs them and CI keeps
// the coverage, while `go test -short ./...` gives a working loop in seconds.
// A machine with no lake on PATH skips them either way, because a missing
// toolchain is an absent test, not a failing one.
func requiresLeanToolchain(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping Lean toolchain test in short mode")
	}
	if _, err := exec.LookPath("lake"); err != nil {
		t.Skip("skipping Lean toolchain test: lake is not on PATH")
	}
}

// emptyLeanRepo is a bare directory for the tests that exercise bootstrapping
// itself, and so cannot start from the prebuilt fixture. Each one pays for a
// full Lean compile, which is why they carry the same guard.
func emptyLeanRepo(t *testing.T) string {
	t.Helper()
	requiresLeanToolchain(t)
	return t.TempDir()
}
