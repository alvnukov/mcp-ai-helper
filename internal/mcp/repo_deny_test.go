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

// The repo-local tools.deny policy was only consulted by the command, lake,
// pipeline, and web runners; file, edit, git, and task ignored it, so a
// repository that denied "edit" still got its files rewritten. The deny
// check belongs in the shared dispatcher, ahead of every action.
func TestRepoDenyCoversAllLocalTools(t *testing.T) {
	repo := t.TempDir()
	denyConfig := "permissions:\n  tools:\n    deny: [file, edit, git, task, command, run]\n"
	if err := os.WriteFile(filepath.Join(repo, ".mcp-ai-helper.yaml"), []byte(denyConfig), 0o600); err != nil {
		t.Fatal(err)
	}

	srv := New(&config.Config{})
	for _, tool := range []string{"file", "edit", "git", "task", "command", "run"} {
		t.Run(tool, func(t *testing.T) {
			handler, ok := srv.ListTools()[tool]
			if !ok {
				t.Fatalf("%s is not registered", tool)
			}
			var req basemcp.CallToolRequest
			req.Params.Name = tool
			req.Params.Arguments = map[string]interface{}{"repo_path": repo, "action": "list"}
			result, err := handler.Handler(context.Background(), req)
			if err != nil {
				t.Fatal(err)
			}
			if !result.IsError {
				t.Fatalf("%s ran despite the repo-local deny: %s", tool, resultText(t, result))
			}
			if got := resultText(t, result); !strings.Contains(got, "denied") {
				t.Fatalf("%s failed for another reason: %s", tool, got)
			}
		})
	}
}
