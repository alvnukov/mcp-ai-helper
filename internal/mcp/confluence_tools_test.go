package mcp

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"
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
	// simulate read flow: get page, check space
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

func confDeps(readOnly *bool) *Server {
	return &Server{cfg: &config.Config{Integrations: config.IntegrationsConfig{
		Confluence: &config.ConfluenceConfig{AllowedSpaces: []string{"VEGA"}, ReadOnly: readOnly},
	}}}
}

func TestConfWritesAreRefusedWhenTheIntegrationIsReadOnly(t *testing.T) {
	readOnly := true
	err := confWritesAllowed(confDeps(&readOnly))
	if err == nil {
		t.Fatal("read_only must refuse an edit")
	}
	if !strings.Contains(err.Error(), "read_only") {
		t.Errorf("refusal = %q; it has to name the setting standing in the way, or the caller cannot tell its user what to change", err)
	}
}

func TestConfWritesAreAllowedWhenReadOnlyIsUnset(t *testing.T) {
	if err := confWritesAllowed(confDeps(nil)); err != nil {
		t.Fatalf("an integration without read_only may be edited: %v", err)
	}
	writable := false
	if err := confWritesAllowed(confDeps(&writable)); err != nil {
		t.Fatalf("read_only: false may be edited: %v", err)
	}
}

func TestConfWritesAreRefusedWithoutAnIntegration(t *testing.T) {
	if err := confWritesAllowed(&Server{cfg: &config.Config{}}); err == nil {
		t.Fatal("an unconfigured integration must refuse an edit")
	}
}

// confluenceSchema decodes what confluence advertises — its arguments and its
// action enum — exactly as a client receives them.
func confluenceSchema(t *testing.T) (map[string]json.RawMessage, []string) {
	t.Helper()

	tool, ok := New(allLayersConfig()).ListTools()["confluence"]
	if !ok {
		t.Fatal("confluence is not registered")
	}
	schemaBytes, err := json.Marshal(tool.Tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal confluence schema: %v", err)
	}
	var schema struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if err := json.Unmarshal(schemaBytes, &schema); err != nil {
		t.Fatalf("decode confluence schema: %v", err)
	}
	if len(schema.Properties) == 0 {
		t.Fatal("confluence advertises no arguments; nothing was checked")
	}
	var action struct {
		Enum []string `json:"enum"`
	}
	raw, ok := schema.Properties["action"]
	if !ok {
		t.Fatal("confluence does not advertise an action argument")
	}
	if err := json.Unmarshal(raw, &action); err != nil {
		t.Fatalf("decode confluence action enum: %v", err)
	}
	return schema.Properties, action.Enum
}

// confluenceRequest binds its arguments with json.Unmarshal, which drops a key
// the struct has no field for without saying anything: the model would be told
// its edit succeeded while half of what it asked for was discarded. So the
// schema and the binder have to name the same arguments, checked in both
// directions — an advertised field nothing binds is a silent loss, and a bound
// field nothing advertises is one no caller knows to send.
func TestConfSchemaAndBinderNameTheSameArguments(t *testing.T) {
	t.Parallel()

	bound := map[string]bool{}
	requestType := reflect.TypeOf(confRequest{})
	for i := range requestType.NumField() {
		name, _, _ := strings.Cut(requestType.Field(i).Tag.Get("json"), ",")
		if name != "" && name != "-" {
			bound[name] = true
		}
	}

	properties, _ := confluenceSchema(t)

	for name := range properties {
		// The dispatcher reads action off the raw arguments before any struct
		// binds it, so it is the one argument advertised without being bound.
		if name == "action" || bound[name] {
			continue
		}
		t.Errorf("confluence advertises %q, which confRequest does not bind: a caller that sends it gets no error and no effect", name)
	}
	for name := range bound {
		if _, advertised := properties[name]; !advertised {
			t.Errorf("confRequest binds %q but the schema does not name it: no caller can discover it", name)
		}
	}
}

func TestConfluenceAdvertisesTheWholeEditingLoop(t *testing.T) {
	t.Parallel()

	_, advertised := confluenceSchema(t)
	got := map[string]bool{}
	for _, action := range advertised {
		got[action] = true
	}
	for _, want := range []string{"search", "read", "spaces", "update", "create", "delete"} {
		if !got[want] {
			t.Errorf("confluence advertises %v, which is missing %q", advertised, want)
		}
	}
	if len(advertised) != 6 {
		t.Errorf("confluence advertises %v, want exactly the six actions", advertised)
	}
}

func TestConfluenceReadsAPageEvenWhenTheIntegrationIsReadOnly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		body := `{"id":"123","type":"page","title":"Capacity","space":{"key":"VEGA"},` +
			`"version":{"number":7},"body":{"storage":{"value":"<p>hi</p>"}}}`
		if _, err := w.Write([]byte(body)); err != nil {
			t.Errorf("write test response: %v", err)
		}
	}))
	defer srv.Close()

	client, err := confluence.NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	readOnly := true
	deps := confDeps(&readOnly)
	deps.confluenceClient = client

	var req basemcp.CallToolRequest
	req.Params.Arguments = map[string]any{"action": "read", "page_id": "123"}
	result, err := confActionRead(context.Background(), req, deps)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("read was refused: %s", resultText(t, result))
	}
	var payload struct {
		Page struct {
			Body    string
			Version int
		} `json:"page"`
	}
	got := resultText(t, result)
	if err := json.Unmarshal([]byte(got), &payload); err != nil {
		t.Fatalf("read returned %s, which does not decode: %v", got, err)
	}
	if payload.Page.Version != 7 {
		t.Errorf("version = %d, want 7: without it there is nothing to guard the edit with", payload.Page.Version)
	}
	if payload.Page.Body != "<p>hi</p>" {
		t.Errorf("body = %q, want the page's storage format: without it there is no span to edit", payload.Page.Body)
	}
}

func TestConfluenceReadRefusesASpaceOutsideTheAllowlist(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if _, err := w.Write([]byte(`{"id":"123","type":"page","title":"Elsewhere","space":{"key":"OTHER"}}`)); err != nil {
			t.Errorf("write test response: %v", err)
		}
	}))
	defer srv.Close()

	client, err := confluence.NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	deps := confDeps(nil)
	deps.confluenceClient = client

	var req basemcp.CallToolRequest
	req.Params.Arguments = map[string]any{"action": "read", "page_id": "123"}
	result, err := confActionRead(context.Background(), req, deps)
	if err != nil {
		t.Fatal(err)
	}
	if !result.IsError {
		t.Fatalf("a page in OTHER must be refused, got %s", resultText(t, result))
	}
	if got := resultText(t, result); !strings.Contains(got, "allowed_spaces") {
		t.Errorf("refusal = %q, want it to name the allowlist", got)
	}
}
