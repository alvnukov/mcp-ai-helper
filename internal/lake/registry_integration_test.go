package lake

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/config"
)

// A registry that duplicates an id, points at an artifact that is not there, or
// refers to itself has to be rejected by Lean rather than by anything on the Go
// side — that is the whole reason the registry lives in Lean. This drives the
// real toolchain over each fixture, because a canned runner result would prove
// only that CheckFile passes a struct through.
func TestRepositoryRegistryInvalidFixturesFailThroughLake(t *testing.T) {
	repoRoot := prepareLakeTestRepo(t)
	runner := CommandRunner{
		Commands: command.NewRunner(config.CommandPolicy{
			AllowedCWDs:           []string{repoRoot},
			DefaultTimeoutSeconds: 60,
			MaxOutputBytes:        200000,
			MaxLines:              80,
		}),
		TimeoutSeconds: 60,
	}

	for _, fixture := range []string{
		"InvalidDuplicateRegistry.lean",
		"InvalidDanglingRegistry.lean",
		"InvalidSelfRegistry.lean",
	} {
		t.Run(fixture, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("testdata/lean", fixture))
			if err != nil {
				t.Fatalf("read fixture: %v", err)
			}
			if err := os.WriteFile(filepath.Join(repoRoot, fixture), source, 0o600); err != nil {
				t.Fatalf("write fixture: %v", err)
			}

			result, err := CheckFile(context.Background(), repoRoot, fixture, runner)
			if err != nil {
				t.Fatalf("CheckFile returned error: %v", err)
			}
			if result.ExitCode == 0 {
				t.Fatalf("Lean accepted an invalid registry: %+v", result)
			}
			if diagnostics := strings.ToLower(strings.Join(result.Diagnostics, "\n")); !strings.Contains(diagnostics, "error") {
				t.Fatalf("expected a Lean error diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}
