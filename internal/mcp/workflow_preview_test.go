package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// The workflow preview was built before the repo runner resolved, so a
// repository whose repo-local config denies "run action=workflow" still
// received step and commit previews — a read-only bypass of the deny.
func TestWorkflowPreviewRespectsRepoDeny(t *testing.T) {
	repo := t.TempDir()
	denyConfig := "permissions:\n  tools:\n    deny: [\"run action=workflow\"]\n"
	if err := os.WriteFile(filepath.Join(repo, ".mcp-ai-helper.yaml"), []byte(denyConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	deps := &Server{cfg: &config.Config{}}
	var req basemcp.CallToolRequest
	req.Params.Name = "run"
	req.Params.Arguments = map[string]interface{}{
		"repo_path": repo,
		"preview":   true,
	}
	result, err := runActionWorkflow(context.Background(), req, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("preview bypassed the repo deny: %s", resultText(t, result))
	}
	if got := resultText(t, result); !strings.Contains(got, "denied") {
		t.Fatalf("expected a deny error, got: %s", got)
	}
}
