package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/server"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
	"github.com/alvnukov/mcp-ai-helper/internal/confluence"
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

func TestRegisterConfluenceTools(_ *testing.T) {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"id":"123","type":"page","title":"Test","space":{"key":"VEGA"}}`)); err != nil {
			t.Errorf("write test response: %v", err)
		}
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

func confluenceDeps(readOnly *bool) *Server {
	return &Server{cfg: &config.Config{Integrations: config.IntegrationsConfig{
		Confluence: &config.ConfluenceConfig{AllowedSpaces: []string{"VEGA"}, ReadOnly: readOnly},
	}}}
}

// read_only is the setting an operator uses to say the helper may read this
// wiki and nothing more. It had a CanMutate() and no caller until conf_edit;
// this is the caller.
func TestConfWritesAreRefusedWhenTheIntegrationIsReadOnly(t *testing.T) {
	readOnly := true
	err := confWritesAllowed(confluenceDeps(&readOnly))
	if err == nil {
		t.Fatal("read_only must refuse an edit")
	}
	if !strings.Contains(err.Error(), "read_only") {
		t.Errorf("refusal = %q; it has to name the setting standing in the way, or the caller cannot tell its user what to change", err)
	}
}

func TestConfWritesAreAllowedWhenReadOnlyIsUnset(t *testing.T) {
	if err := confWritesAllowed(confluenceDeps(nil)); err != nil {
		t.Fatalf("an integration without read_only may be edited: %v", err)
	}
	writable := false
	if err := confWritesAllowed(confluenceDeps(&writable)); err != nil {
		t.Fatalf("read_only: false may be edited: %v", err)
	}
}

func TestConfWritesAreRefusedWithoutAnIntegration(t *testing.T) {
	if err := confWritesAllowed(&Server{cfg: &config.Config{}}); err == nil {
		t.Fatal("an unconfigured integration must refuse an edit")
	}
}

// conf_edit binds its arguments with json.Unmarshal, which drops a key the
// struct has no field for without saying anything: the model would be told its
// edit succeeded while half of what it asked for was discarded. So the schema
// and the binder have to name the same arguments, checked in both directions —
// an advertised field nothing binds is a silent loss, and a bound field nothing
// advertises is one no caller knows to send.
func TestConfEditSchemaAndBinderNameTheSameArguments(t *testing.T) {
	t.Parallel()

	bound := map[string]bool{}
	requestType := reflect.TypeOf(confEditRequest{})
	for i := range requestType.NumField() {
		name, _, _ := strings.Cut(requestType.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			bound[name] = true
		}
	}

	tool, ok := New(allLayersConfig()).ListTools()["conf_edit"]
	if !ok {
		t.Fatal("conf_edit is not registered")
	}
	schemaBytes, err := json.Marshal(tool.Tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal conf_edit schema: %v", err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("decode conf_edit schema: %v", err)
	}
	if len(schema.Properties) == 0 {
		t.Fatal("conf_edit advertises no arguments; nothing was checked")
	}

	for name := range schema.Properties {
		// The dispatcher reads action off the raw arguments before any struct
		// binds it, so it is the one argument advertised without being bound.
		if name == "action" || bound[name] {
			continue
		}
		t.Errorf("conf_edit advertises %q, which confEditRequest does not bind: a caller that sends it gets no error and no effect", name)
	}
	for name := range bound {
		if _, advertised := schema.Properties[name]; !advertised {
			t.Errorf("confEditRequest binds %q but the schema does not name it: no caller can discover it", name)
		}
	}
}
