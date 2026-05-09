package lake

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/config"
)

func TestRepositoryRegistryInvalidFixturesFailThroughLake(t *testing.T) {
	repoRoot := filepath.Clean("../..")
	runner := CommandRunner{
		Commands: command.NewRunner(config.CommandPolicy{
			AllowedCWDs:           []string{repoRoot},
			DefaultTimeoutSeconds: 20,
			MaxOutputBytes:        20000,
			MaxLines:              80,
		}),
		TimeoutSeconds: 20,
	}

	for _, fixture := range []string{
		"testdata/lean/InvalidDuplicateRegistry.lean",
		"testdata/lean/InvalidDanglingRegistry.lean",
		"testdata/lean/InvalidSelfRegistry.lean",
	} {
		t.Run(fixture, func(t *testing.T) {
			result, err := CheckFile(context.Background(), repoRoot, fixture, runner)
			if err != nil {
				t.Fatalf("CheckFile returned error: %v", err)
			}
			if result.ExitCode == 0 {
				t.Fatalf("expected invalid registry fixture to fail, got %+v", result)
			}
			joinedDiagnostics := strings.Join(result.Diagnostics, "\n")
			if !strings.Contains(strings.ToLower(joinedDiagnostics), "error") {
				t.Fatalf("expected Lean error diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}
