package mcp

import (
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

func TestNewForRepositoryScopesIntegrationTools(t *testing.T) {
	allowedRepository := t.TempDir()
	cfg := integrationScopeTestConfig(allowedRepository)

	allowedTools := NewForRepository(cfg, allowedRepository).ListTools()
	for _, name := range []string{"jira_read", "confluence"} {
		if _, ok := allowedTools[name]; !ok {
			t.Errorf("%s should be registered for an allowed repository", name)
		}
	}

	deniedTools := NewForRepository(cfg, t.TempDir()).ListTools()
	unknownTools := NewForRepository(cfg, "").ListTools()
	for _, name := range []string{"jira_read", "confluence"} {
		if _, ok := deniedTools[name]; ok {
			t.Errorf("%s should be hidden for a denied repository", name)
		}
		if _, ok := unknownTools[name]; ok {
			t.Errorf("%s should be hidden without repository context", name)
		}
	}
}

func TestNewKeepsUnscopedIntegrationTools(t *testing.T) {
	cfg := integrationScopeTestConfig("")
	tools := New(cfg).ListTools()
	for _, name := range []string{"jira_read", "confluence"} {
		if _, ok := tools[name]; !ok {
			t.Errorf("%s should keep legacy unscoped registration", name)
		}
	}
}

func integrationScopeTestConfig(allowedRepository string) *config.Config {
	jiraConfig := &config.JiraConfig{
		URL:    "https://jira.invalid",
		APIKey: "test-token",
	}
	confluenceConfig := &config.ConfluenceConfig{
		URL:    "https://confluence.invalid/rest/api",
		APIKey: "test-token",
	}
	if allowedRepository != "" {
		jiraConfig.AllowedRepositories = []string{allowedRepository}
		confluenceConfig.AllowedRepositories = []string{allowedRepository}
	}
	return &config.Config{
		AssistantGuidance: config.DefaultAssistantGuidance(),
		Integrations: config.IntegrationsConfig{
			Jira:       jiraConfig,
			Confluence: confluenceConfig,
		},
	}
}
