package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

type configAllowRepositoryResult struct {
	Added           []string `json:"added"`
	Reloaded        bool     `json:"reloaded"`
	RestartRequired bool     `json:"restart_required"`
}

func TestConfigAllowRepositoryToolAddsStartupRepository(t *testing.T) {
	repoPath := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	initial := "layers:\n" +
		"  models:\n" +
		"    enabled: false\n" +
		"integrations:\n" +
		"  jira:\n" +
		"    url: https://jira.example.test\n" +
		"    api_key: jira-token-123\n" +
		"  confluence:\n" +
		"    url: https://confluence.example.test/rest/api\n" +
		"    api_key: confluence-token-123\n"
	if _, err := writeValidatedConfig(configPath, initial); err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	srv := NewForRepository(cfg, repoPath)
	tools := srv.ListTools()
	tool, ok := tools["config_allow_repository"]
	if !ok {
		t.Fatal("config_allow_repository is not registered for a fail-closed repository")
	}
	if _, ok := tools["config_read"]; ok {
		t.Fatal("test requires the regular config suite to be hidden with models and integrations off")
	}
	if _, ok := tools["jira_search"]; ok {
		t.Fatal("Jira tools should be hidden before the repository is allowed")
	}

	call := func() string {
		t.Helper()
		var req basemcp.CallToolRequest
		req.Params.Name = "config_allow_repository"
		req.Params.Arguments = map[string]any{"integration": "both", "reload": false}
		result, callErr := tool.Handler(context.Background(), req)
		if callErr != nil {
			t.Fatalf("call config_allow_repository: %v", callErr)
		}
		if result.IsError {
			t.Fatalf("config_allow_repository failed: %s", resultText(t, result))
		}
		return resultText(t, result)
	}

	first := call()
	var firstResult configAllowRepositoryResult
	if err := json.Unmarshal([]byte(first), &firstResult); err != nil {
		t.Fatalf("decode first result %q: %v", first, err)
	}
	if len(firstResult.Added) != 2 || firstResult.Added[0] != "jira" || firstResult.Added[1] != "confluence" {
		t.Fatalf("first added = %#v, want jira and confluence", firstResult.Added)
	}
	if firstResult.Reloaded || !firstResult.RestartRequired {
		t.Fatalf("first result flags = %#v", firstResult)
	}

	wantRepository, err := normalizeConfigRepositoryPath(repoPath)
	if err != nil {
		t.Fatalf("normalize expected repository: %v", err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load updated config: %v", err)
	}
	for name, allowed := range map[string][]string{
		"jira":       loaded.Integrations.Jira.AllowedRepositories,
		"confluence": loaded.Integrations.Confluence.AllowedRepositories,
	} {
		if len(allowed) != 1 || allowed[0] != wantRepository {
			t.Fatalf("%s allowed_repositories = %#v, want [%q]", name, allowed, wantRepository)
		}
	}
	if loaded.Integrations.Jira.APIKey != "jira-token-123" ||
		loaded.Integrations.Confluence.APIKey != "confluence-token-123" {
		t.Fatal("config_allow_repository did not preserve integration tokens")
	}

	second := call()
	var secondResult configAllowRepositoryResult
	if err := json.Unmarshal([]byte(second), &secondResult); err != nil {
		t.Fatalf("decode idempotent result %q: %v", second, err)
	}
	if len(secondResult.Added) != 0 {
		t.Fatalf("idempotent added = %#v, want empty", secondResult.Added)
	}
	loaded, err = config.Load(configPath)
	if err != nil {
		t.Fatalf("reload updated config: %v", err)
	}
	if len(loaded.Integrations.Jira.AllowedRepositories) != 1 ||
		len(loaded.Integrations.Confluence.AllowedRepositories) != 1 {
		t.Fatal("idempotent call duplicated repository entries")
	}
}

func TestAllowConfigRepositoryRejectsUnsafeTargets(t *testing.T) {
	repoPath := t.TempDir()
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	if _, err := writeValidatedConfig(configPath, "integrations:\n  jira:\n    url: https://jira.example.test\n    api_key: jira-token-123\n"); err != nil {
		t.Fatalf("write config: %v", err)
	}
	if _, _, _, err := allowConfigRepository(configPath, "both", repoPath); err == nil ||
		!strings.Contains(err.Error(), "confluence integration is not configured") {
		t.Fatalf("missing integration error = %v", err)
	}
	loaded, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("load unchanged config: %v", err)
	}
	if len(loaded.Integrations.Jira.AllowedRepositories) != 0 {
		t.Fatal("failed both operation partially mutated Jira allowlist")
	}
	if _, _, _, err := allowConfigRepository(configPath, "github", repoPath); err == nil ||
		!strings.Contains(err.Error(), "must be jira, confluence, or both") {
		t.Fatalf("unsupported integration error = %v", err)
	}
	if _, _, _, err := allowConfigRepository(configPath, "jira", ""); err == nil ||
		!strings.Contains(err.Error(), "startup repository context is unavailable") {
		t.Fatalf("missing repository error = %v", err)
	}

	repoConfigPath := filepath.Join(t.TempDir(), ".mcp-ai-helper.yaml")
	if err := os.WriteFile(repoConfigPath, []byte("integrations: {}\n"), 0o600); err != nil {
		t.Fatalf("write repo config: %v", err)
	}
	if _, _, _, err := allowConfigRepository(repoConfigPath, "jira", repoPath); err == nil ||
		!strings.Contains(err.Error(), "user-editable only") {
		t.Fatalf("repo-local config error = %v", err)
	}
}
