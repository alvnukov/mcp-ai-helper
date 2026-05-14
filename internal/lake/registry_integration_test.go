package lake

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryRegistryInvalidFixturesFailThroughLake(t *testing.T) {
	repoRoot := prepareLakeTestRepo(t)
	fixtures := []string{
		"InvalidDuplicateRegistry.lean",
		"InvalidDanglingRegistry.lean",
		"InvalidSelfRegistry.lean",
	}
	for _, name := range fixtures {
		src, err := os.ReadFile(filepath.Join("testdata/lean", name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(repoRoot, name), src, 0o600); err != nil {
			t.Fatalf("write fixture %s: %v", name, err)
		}
	}
	errDiag := FilterDiagnostics("error: duplicate definition in registry\nerror: dangling reference")
	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			runner := &fakeRunner{result: CommandResult{WorkspaceDetected: true, WorkspaceDir: repoRoot, ExitCode: 1, Diagnostics: errDiag}}
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
