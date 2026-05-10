package mcp

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/confluence"
)

func TestCheckConfSpace_Allowed(t *testing.T) {
	deps := &Server{cfg: &config.Config{Integrations: config.IntegrationsConfig{
		Confluence: &config.ConfluenceConfig{AllowedSpaces: []string{"VEGA"}},
	}}}
	if !checkConfSpace(deps, "VEGA") {
		t.Fatal("VEGA should be allowed")
	}
}

func TestCheckConfSpace_Denied(t *testing.T) {
	deps := &Server{cfg: &config.Config{Integrations: config.IntegrationsConfig{
		Confluence: &config.ConfluenceConfig{AllowedSpaces: []string{"VEGA"}},
	}}}
	if checkConfSpace(deps, "OTHER") {
		t.Fatal("OTHER should be denied")
	}
}

func TestCheckConfSpace_EmptyAllowlist(t *testing.T) {
	deps := &Server{cfg: &config.Config{Integrations: config.IntegrationsConfig{
		Confluence: &config.ConfluenceConfig{},
	}}}
	if !checkConfSpace(deps, "ANYTHING") {
		t.Fatal("empty allowlist should allow all")
	}
}

func TestCheckConfSpace_NotConfigured(t *testing.T) {
	deps := &Server{cfg: &config.Config{}}
	if checkConfSpace(deps, "ANYTHING") {
		t.Fatal("nil Confluence config should deny all")
	}
}

func TestRegisterConfluenceTools(t *testing.T) {
	// verify tools register without panic
	srv := server.NewMCPServer("test", "1.0")
	deps := &Server{
		cfg: &config.Config{Integrations: config.IntegrationsConfig{
			Confluence: &config.ConfluenceConfig{
				URL:     "https://example.com/wiki/rest/api",
				APIKey:  "test",
				Enabled: func() *bool { b := true; return &b }(),
			},
		}},
	}
	deps.confluenceClient, _ = confluence.NewClient(confluence.Config{
		URL:    "https://example.com/wiki/rest/api",
		APIKey: "test",
	})
	registerConfluenceTools(srv, deps)
	// if we got here without panic, registration succeeded
}

func TestCheckConfSpace_Integration(t *testing.T) {
	// simulate conf_read flow: get page, check space
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"123","type":"page","title":"Test","space":{"key":"VEGA"}}`))
	}))
	defer srv.Close()

	c, err := confluence.NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	page, err := c.GetContentByID("123")
	if err != nil {
		t.Fatal(err)
	}
	if page.Space != "VEGA" {
		t.Fatalf("expected space VEGA, got %s", page.Space)
	}

	// verify scoping check
	deps := &Server{cfg: &config.Config{Integrations: config.IntegrationsConfig{
		Confluence: &config.ConfluenceConfig{AllowedSpaces: []string{"VEGA"}},
	}}}
	if !checkConfSpace(deps, page.Space) {
		t.Fatal("page from VEGA space should be allowed")
	}
}
