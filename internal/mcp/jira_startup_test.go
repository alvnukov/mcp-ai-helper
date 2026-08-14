package mcp

import (
	"context"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// jira_* tools are registered from the startup config, so a construction
// error has to survive to the first call. It used to be swallowed in
// buildJiraClient, which left every tool answering "not configured or
// connection failed" — no way to tell a missing api key from a broken URL.
func TestJiraToolsAnswerWithTheStartupError(t *testing.T) {
	enabled := true
	cfg := &config.Config{Integrations: config.IntegrationsConfig{
		Jira: &config.JiraConfig{URL: "https://jira.example.com", Enabled: &enabled},
	}}

	srv := New(cfg)
	tool, ok := srv.ListTools()["jira_search"]
	if !ok {
		t.Fatal("jira_search is not registered")
	}
	var req basemcp.CallToolRequest
	req.Params.Name = "jira_search"
	req.Params.Arguments = map[string]interface{}{"jql": "project = X"}
	result, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("jira_search should refuse a config with no api key: %s", resultText(t, result))
	}
	if got := resultText(t, result); !strings.Contains(got, "api key is required") {
		t.Fatalf("jira_search hides the startup cause: %s", got)
	}
}
