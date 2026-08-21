package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// registeredTools reports the action enum each registered tool advertises, which
// is what a client sees and therefore what the dispatcher has to honour.
func registeredTools(t *testing.T) map[string][]string {
	t.Helper()
	srv := New(&config.Config{AssistantGuidance: config.DefaultAssistantGuidance()})
	advertised := map[string][]string{}
	for name, tool := range srv.ListTools() {
		schemaBytes, err := json.Marshal(tool.Tool.InputSchema)
		if err != nil {
			t.Fatalf("marshal %s schema: %v", name, err)
		}
		var schema struct {
			Properties struct {
				Action struct {
					Enum []string `json:"enum"`
				} `json:"action"`
			} `json:"properties"`
		}
		if err := json.Unmarshal(schemaBytes, &schema); err != nil {
			t.Fatalf("decode %s schema: %v", name, err)
		}
		if len(schema.Properties.Action.Enum) > 0 {
			advertised[name] = schema.Properties.Action.Enum
		}
	}
	return advertised
}

func callWithAction(t *testing.T, handler func(context.Context, basemcp.CallToolRequest) (*basemcp.CallToolResult, error), arguments any) *basemcp.CallToolResult {
	t.Helper()
	var req basemcp.CallToolRequest
	req.Params.Arguments = arguments
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned a transport error: %v", err)
	}
	return result
}

func resultText(t *testing.T, result *basemcp.CallToolResult) string {
	t.Helper()
	if result == nil {
		t.Fatal("handler returned no result")
		return ""
	}
	var text strings.Builder
	for _, content := range result.Content {
		if item, ok := content.(basemcp.TextContent); ok {
			text.WriteString(item.Text)
		}
	}
	return text.String()
}

func testActions() actions {
	return actions{
		"read": ignoringContext(func(basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			return basemcp.NewToolResultText("read ran"), nil
		}),
		"write": ignoringContext(func(basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
			return basemcp.NewToolResultText("write ran"), nil
		}),
	}
}

func TestDispatchRoutesToTheNamedAction(t *testing.T) {
	got := resultText(t, callWithAction(t, dispatch(nil, "file", testActions()), map[string]any{"action": "write"}))
	if got != "write ran" {
		t.Errorf("got %q, want the write handler to run", got)
	}
}

// A model that guesses an action wrong can only recover if the refusal says what
// would have worked, so that is part of the contract rather than a nicety.
func TestAnUnknownActionIsAnsweredWithTheOnesThatExist(t *testing.T) {
	result := callWithAction(t, dispatch(nil, "file", testActions()), map[string]any{"action": "delete"})
	if !result.IsError {
		t.Fatal("an unknown action must be an error")
	}
	got := resultText(t, result)
	for _, want := range []string{"file", "delete", "read", "write"} {
		if !strings.Contains(got, want) {
			t.Errorf("the message %q should mention %q", got, want)
		}
	}
}

func TestAMissingActionSaysSoRatherThanReportingAnEmptyOne(t *testing.T) {
	for name, arguments := range map[string]any{
		"absent":     map[string]any{"repo_path": "."},
		"empty":      map[string]any{"action": ""},
		"wrong type": map[string]any{"action": 7},
		"no map":     nil,
	} {
		result := callWithAction(t, dispatch(nil, "task", testActions()), arguments)
		if !result.IsError {
			t.Errorf("%s action: expected an error", name)
			continue
		}
		if got := resultText(t, result); !strings.Contains(got, "action is required") {
			t.Errorf("%s action: got %q", name, got)
		}
	}
}

func TestTheAdvertisedActionsAreTheDispatchedActions(t *testing.T) {
	// actionEnum and dispatch read the same map, so this holds by construction —
	// the test is here to keep a future refactor from separating them again.
	handlers := testActions()
	names := handlers.names()
	if len(names) != len(handlers) {
		t.Fatalf("names() dropped an action: %v", names)
	}
	for _, name := range names {
		if _, ok := handlers[name]; !ok {
			t.Errorf("advertised action %q has no handler", name)
		}
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("names() must be stable and sorted, got %v", names)
		}
	}
}

// registerAll is what New does; running it here is the cheapest way to catch a
// tool whose schema and dispatcher disagree, or a duplicate tool name.
func TestEveryMergedToolRegistersWithAnActionEnumThatMatchesItsHandlers(t *testing.T) {
	merged := map[string]actions{
		"file":    {"read": nil, "read_many": nil, "list": nil, "search": nil, "snapshot": nil},
		"edit":    {"replace": nil, "write": nil},
		"command": {"run": nil, "cleanup": nil, "abort": nil, "list": nil, "get": nil, "filter": nil, "health": nil},
		"task":    {"current": nil, "get": nil, "list": nil, "search": nil, "upsert": nil, "set_status": nil, "batch_upsert": nil, "delete": nil},
		"run":     {"pipeline": nil, "workflow": nil, "workflow_status": nil, "schema": nil},
		"note":    {"add": nil, "list": nil, "read": nil, "search": nil, "update": nil, "delete": nil},
	}
	for _, name := range gitAdvancedActions {
		if merged["git"] == nil {
			merged["git"] = actions{"status": nil, "diff": nil, "commit": nil}
		}
		merged["git"][name] = nil
	}

	tools := registeredTools(t)
	for tool, want := range merged {
		got, ok := tools[tool]
		if !ok {
			t.Errorf("tool %q is not registered", tool)
			continue
		}
		if len(got) != len(want) {
			t.Errorf("tool %q advertises %v, want the %d actions %v", tool, got, len(want), want.names())
			continue
		}
		for _, action := range got {
			if _, ok := want[action]; !ok {
				t.Errorf("tool %q advertises action %q, which is not part of its documented surface", tool, action)
			}
		}
	}
}
