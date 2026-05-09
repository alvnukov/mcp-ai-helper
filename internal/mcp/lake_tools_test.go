package mcp

import (
	"context"
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
