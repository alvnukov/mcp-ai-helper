package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIntegrationRepositoryScope(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "nested", "..", "project")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}

	jira := &JiraConfig{AllowedRepositories: []string{root}}
	confluence := &ConfluenceConfig{AllowedRepositories: []string{root}}

	cases := []struct {
		name    string
		enabled bool
		want    bool
	}{
		{name: "jira allowed", enabled: jira.IsEnabledForRepository(nested), want: true},
		{name: "confluence allowed", enabled: confluence.IsEnabledForRepository(nested), want: true},
		{name: "jira unknown denied", enabled: jira.IsEnabledForRepository(""), want: false},
		{name: "confluence unknown denied", enabled: confluence.IsEnabledForRepository(""), want: false},
	}
	for _, tc := range cases {
		if tc.enabled != tc.want {
			t.Errorf("%s = %v, want %v", tc.name, tc.enabled, tc.want)
		}
	}
}

func TestIntegrationRepositoryScopeEmptyAllowlistDeniesEverywhere(t *testing.T) {
	repository := t.TempDir()
	if (&JiraConfig{}).IsEnabledForRepository(repository) {
		t.Fatal("Jira should be disabled without an explicit allowed repository")
	}
	if (&ConfluenceConfig{}).IsEnabledForRepository(repository) {
		t.Fatal("Confluence should be disabled without an explicit allowed repository")
	}
}

func TestConfigValidateRejectsRelativeIntegrationRepository(t *testing.T) {
	cfg := defaultConfig()
	cfg.Integrations.Jira = &JiraConfig{AllowedRepositories: []string{"relative/repo"}}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "integrations.jira.allowed_repositories") {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestSchemaDocumentsIntegrationRepositoryScopes(t *testing.T) {
	fields, ok := Schema()["fields"].([]FieldDoc)
	if !ok {
		t.Fatalf("schema fields have type %T, want []FieldDoc", Schema()["fields"])
	}
	wanted := map[string]bool{
		"integrations.jira.allowed_repositories":       false,
		"integrations.confluence.allowed_repositories": false,
	}
	for _, field := range fields {
		if _, exists := wanted[field.Path]; exists {
			wanted[field.Path] = true
		}
	}
	for field, found := range wanted {
		if !found {
			t.Errorf("config schema is missing %s", field)
		}
	}
}
