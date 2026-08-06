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

	properties, _ := confEditSchema(t)

	for name := range properties {
		// The dispatcher reads action off the raw arguments before any struct
		// binds it, so it is the one argument advertised without being bound.
		if name == "action" || bound[name] {
			continue
		}
		t.Errorf("conf_edit advertises %q, which confEditRequest does not bind: a caller that sends it gets no error and no effect", name)
	}
	for name := range bound {
		if _, advertised := properties[name]; !advertised {
			t.Errorf("confEditRequest binds %q but the schema does not name it: no caller can discover it", name)
		}
	}
}

// confEditSchema decodes what conf_edit advertises — its arguments and its
// action enum — exactly as a client receives them.
func confEditSchema(t *testing.T) (map[string]json.RawMessage, []string) {
	t.Helper()

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
	var action struct {
		Enum []string `json:"enum"`
	}
	raw, ok := schema.Properties["action"]
	if !ok {
		t.Fatal("conf_edit does not advertise an action argument")
	}
	if err := json.Unmarshal(raw, &action); err != nil {
		t.Fatalf("decode conf_edit action enum: %v", err)
	}
	return schema.Properties, action.Enum
}

// An editing tool that cannot show the page it is about to edit is not a whole
// tool: the caller needs the current body to build a span edit and the current
// version to guard it, and would otherwise have to know which other tool to
// call in the middle of an edit.
func TestConfEditAdvertisesTheWholeEditingLoop(t *testing.T) {
	t.Parallel()

	_, advertised := confEditSchema(t)
	got := map[string]bool{}
	for _, action := range advertised {
		got[action] = true
	}
	for _, want := range []string{"read", "update", "create", "delete"} {
		if !got[want] {
			t.Errorf("conf_edit advertises %v, which is missing %q", advertised, want)
		}
	}
	if len(advertised) != 4 {
		t.Errorf("conf_edit advertises %v, want exactly the four editing actions", advertised)
	}
}

// Reading is not writing. A read_only integration still answers a read, and the
// caller meets read_only at update — where the refusal can name it — rather than
// at a read that would have to pretend the page was unreachable.
func TestConfEditReadsAPageEvenWhenTheIntegrationIsReadOnly(t *testing.T) {
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
	deps := confluenceDeps(&readOnly)
	deps.confluenceClient = client

	var req basemcp.CallToolRequest
	req.Params.Arguments = map[string]any{"action": "read", "page_id": "123"}
	result, err := confEditRead(context.Background(), req, deps)
	if err != nil {
		t.Fatal(err)
	}
	if result.IsError {
		t.Fatalf("read was refused: %s", resultText(t, result))
	}
	// Decoded rather than matched as a substring: encoding/json escapes the
	// angle brackets of storage format, so the body a caller gets back is not
	// the text it would search for.
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

// A page outside allowed_spaces is out of reach for the edit tool's read too,
// not only for the read tool's.
func TestConfEditReadRefusesASpaceOutsideTheAllowlist(t *testing.T) {
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
	deps := confluenceDeps(nil)
	deps.confluenceClient = client

	var req basemcp.CallToolRequest
	req.Params.Arguments = map[string]any{"action": "read", "page_id": "123"}
	result, err := confEditRead(context.Background(), req, deps)
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
