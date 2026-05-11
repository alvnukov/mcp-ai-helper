package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func TestLakeSmokeToolRegistered(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	srv := New(cfg)
	if _, ok := srv.ListTools()["lake_smoke"]; !ok {
		t.Fatal("lake_smoke tool is not registered")
	}
}

func TestRunLakeSmokeMissingWorkspaceReturnsBlocker(t *testing.T) {
	result, err := runLakeSmoke(context.Background(), lakeSmokeRequest{RepoPath: "../lake/testdata/missing", Mode: "build"}, nil)
	if err != nil {
		t.Fatalf("runLakeSmoke returned error: %v", err)
	}
	if result.ExitCode != -1 || !strings.Contains(result.Blocker, "missing lean-toolchain") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunLakeSmokeCheckRequiresFile(t *testing.T) {
	result, err := runLakeSmoke(context.Background(), lakeSmokeRequest{RepoPath: "../lake/testdata/valid", Mode: "check"}, nil)
	if err != nil {
		t.Fatalf("runLakeSmoke returned error: %v", err)
	}
	if !strings.Contains(result.Blocker, "file is required") {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestRunLakeInitTreatsLakefileTomlAsExistingWorkspace(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "lean-toolchain"), []byte("leanprover/lean4:v4.29.1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "lakefile.toml"), []byte("name = \"bootstrap\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := runLakeInit(context.Background(), lakeInitRequest{RepoPath: repo}, nil)
	if err != nil {
		t.Fatalf("runLakeInit returned error: %v", err)
	}
	if !result.AlreadyExisted {
		t.Fatalf("expected existing workspace result: %+v", result)
	}
	if _, err := os.Stat(filepath.Join(repo, "lakefile.lean")); !os.IsNotExist(err) {
		t.Fatalf("lake_init should not create lakefile.lean when lakefile.toml exists: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "Bootstrap.lean")); !os.IsNotExist(err) {
		t.Fatalf("lake_init should not create Bootstrap.lean when workspace exists: %v", err)
	}
}
