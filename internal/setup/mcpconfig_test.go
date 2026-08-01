package setup

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

func parseJSON(t *testing.T, text string) map[string]any {
	t.Helper()
	var root map[string]any
	if err := json.Unmarshal([]byte(text), &root); err != nil {
		t.Fatalf("not valid JSON: %v\n%s", err, text)
	}
	return root
}

func dig(t *testing.T, root map[string]any, path ...string) any {
	t.Helper()
	var current any = root
	for _, key := range path {
		table, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%v is not a map on the way to %v", current, path)
		}
		current = table[key]
	}
	return current
}

func TestANewClaudeConfigGetsTheHelperCommand(t *testing.T) {
	written, err := mergeJSON("", "mcpServers", claudeEntry("/usr/bin/mcp-ai-helper", []string{"--config", "/etc/helper.yaml"}))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	root := parseJSON(t, written)
	if got := dig(t, root, "mcpServers", serverName, "command"); got != "/usr/bin/mcp-ai-helper" {
		t.Errorf("command: got %v", got)
	}
	args, ok := dig(t, root, "mcpServers", serverName, "args").([]any)
	if !ok || len(args) != 2 || args[0] != "--config" {
		t.Errorf("args: got %v", args)
	}
}

func TestAnEntryWithoutArgsKeepsThemAnEmptyArray(t *testing.T) {
	written, err := mergeJSON("", "mcpServers", claudeEntry("mcp-ai-helper", nil))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	args, ok := dig(t, parseJSON(t, written), "mcpServers", serverName, "args").([]any)
	if !ok || len(args) != 0 {
		t.Errorf("args must be [] rather than null, got %v", dig(t, parseJSON(t, written), "mcpServers", serverName, "args"))
	}
}

func TestOtherServersInTheConfigSurvive(t *testing.T) {
	existing := `{"mcpServers":{"other":{"command":"other-server"}},"theme":"dark"}`
	written, err := mergeJSON(existing, "mcpServers", claudeEntry("mcp-ai-helper", nil))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	root := parseJSON(t, written)
	if got := dig(t, root, "mcpServers", "other", "command"); got != "other-server" {
		t.Errorf("another server was lost: %v", got)
	}
	if got := dig(t, root, "theme"); got != "dark" {
		t.Errorf("unrelated keys must be preserved, got %v", got)
	}
	if _, ok := dig(t, root, "mcpServers", serverName).(map[string]any); !ok {
		t.Error("the helper should have been added")
	}
}

func TestALargeNumberSurvivesTheRoundTrip(t *testing.T) {
	// Decoding into float64 would quietly round this, rewriting a value the
	// helper only ever passed through.
	existing := `{"sessionId":9007199254740993,"mcpServers":{}}`
	written, err := mergeJSON(existing, "mcpServers", claudeEntry("mcp-ai-helper", nil))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !strings.Contains(written, "9007199254740993") {
		t.Errorf("a large integer must survive verbatim:\n%s", written)
	}
}

func TestReRunningSetupReplacesOnlyTheHelperEntry(t *testing.T) {
	first, err := mergeJSON("", "mcpServers", claudeEntry("old", nil))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	second, err := mergeJSON(first, "mcpServers", claudeEntry("new", nil))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	root := parseJSON(t, second)
	if got := dig(t, root, "mcpServers", serverName, "command"); got != "new" {
		t.Errorf("command: got %v", got)
	}
	servers, _ := dig(t, root, "mcpServers").(map[string]any)
	if len(servers) != 1 {
		t.Errorf("a re-run must not duplicate the entry, got %v", servers)
	}
}

func TestMergingTheSameEntryTwiceIsAFixedPoint(t *testing.T) {
	// The caller compares against the file to decide whether to write, so this
	// equality is what makes a repeated setup a no-op.
	once, err := mergeJSON("", "mcpServers", claudeEntry("/usr/bin/mcp-ai-helper", nil))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	twice, err := mergeJSON(once, "mcpServers", claudeEntry("/usr/bin/mcp-ai-helper", nil))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if once != twice {
		t.Errorf("JSON merge is not a fixed point:\n%s\n---\n%s", once, twice)
	}

	once, err = mergeCodexTOML("model = \"o3\"\n", "/usr/bin/mcp-ai-helper", nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	twice, err = mergeCodexTOML(once, "/usr/bin/mcp-ai-helper", nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if once != twice {
		t.Errorf("TOML merge is not a fixed point:\n%s\n---\n%s", once, twice)
	}
}

func TestABrokenExistingConfigIsRefusedRatherThanOverwritten(t *testing.T) {
	if _, err := mergeJSON("{not json", "mcpServers", claudeEntry("x", nil)); err == nil {
		t.Error("broken JSON must be refused")
	}
	if _, err := mergeJSON(`["a list"]`, "mcpServers", claudeEntry("x", nil)); err == nil {
		t.Error("a JSON document that is not an object must be refused")
	}
	if _, err := mergeCodexTOML("not = = toml", "x", nil); err == nil {
		t.Error("broken TOML must be refused")
	}
	if _, err := withoutJSON("{not json", "mcpServers"); err == nil {
		t.Error("broken JSON must be refused on remove too")
	}
	if _, err := withoutCodexTOML("not = = toml"); err == nil {
		t.Error("broken TOML must be refused on remove too")
	}
}

func TestRemovingTakesOutTheHelperAndNothingElse(t *testing.T) {
	existing := `{"mcpServers":{"` + serverName + `":{"command":"h"},"other":{"command":"o"}},"theme":"dark"}`
	written, err := withoutJSON(existing, "mcpServers")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if written == nil {
		t.Fatal("the helper was registered")
		return
	}
	root := parseJSON(t, *written)
	if got := dig(t, root, "mcpServers", serverName); got != nil {
		t.Errorf("the helper should be gone, got %v", got)
	}
	if got := dig(t, root, "mcpServers", "other", "command"); got != "o" {
		t.Errorf("another server was lost: %v", got)
	}
	if got := dig(t, root, "theme"); got != "dark" {
		t.Errorf("unrelated keys must be preserved, got %v", got)
	}
}

func TestASectionHoldingOnlyTheHelperGoesAwayWithIt(t *testing.T) {
	written, err := withoutJSON(`{"mcpServers":{"`+serverName+`":{}},"theme":"dark"}`, "mcpServers")
	if err != nil {
		t.Fatalf("remove: %v", err)
	}
	if written == nil {
		t.Fatal("the helper was registered")
		return
	}
	root := parseJSON(t, *written)
	if _, ok := root["mcpServers"]; ok {
		t.Errorf("an empty server list is clutter, not configuration: %s", *written)
	}
	if got := root["theme"]; got != "dark" {
		t.Errorf("theme: got %v", got)
	}
}

func TestRemovingWhatWasNeverRegisteredWritesNothing(t *testing.T) {
	// Each of these is a way for the helper to be absent, and none of them is a
	// reason to rewrite — let alone reformat — somebody else's config.
	for _, existing := range []string{"", `{"theme":"dark"}`, `{"mcpServers":{"other":{}}}`} {
		got, err := withoutJSON(existing, "mcpServers")
		if err != nil || got != nil {
			t.Errorf("withoutJSON(%q): got (%v, %v), want (nil, nil)", existing, got, err)
		}
	}
	for _, existing := range []string{"", "model = \"o3\"\n", "[mcp_servers.other]\ncommand = \"o\"\n"} {
		got, err := withoutCodexTOML(existing)
		if err != nil || got != nil {
			t.Errorf("withoutCodexTOML(%q): got (%v, %v), want (nil, nil)", existing, got, err)
		}
	}
}

func TestSetupThenRemoveLeavesTheConfigAsItWas(t *testing.T) {
	original := `{"theme":"dark","mcpServers":{"other":{"command":"o"}}}`
	installed, err := mergeJSON(original, "mcpServers", claudeEntry("/usr/bin/mcp-ai-helper", nil))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	uninstalled, err := withoutJSON(installed, "mcpServers")
	if err != nil || uninstalled == nil {
		t.Fatalf("remove: got (%v, %v)", uninstalled, err)
	}
	canonical, err := encodeJSON(parseJSON(t, original))
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if *uninstalled != canonical {
		t.Errorf("a round trip must not leave anything behind:\n%s\n---\n%s", *uninstalled, canonical)
	}

	originalTOML := "model = \"o3\"\n"
	installedTOML, err := mergeCodexTOML(originalTOML, "/usr/bin/mcp-ai-helper", nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	uninstalledTOML, err := withoutCodexTOML(installedTOML)
	if err != nil || uninstalledTOML == nil {
		t.Fatalf("remove: got (%v, %v)", uninstalledTOML, err)
	}
	if *uninstalledTOML != originalTOML {
		t.Errorf("a TOML round trip must not leave anything behind:\n%q", *uninstalledTOML)
	}
}

func TestCodexGetsTomlUnderMcpServers(t *testing.T) {
	written, err := mergeCodexTOML("model = \"o3\"\n", "/usr/bin/mcp-ai-helper", []string{"--config", "/etc/helper.yaml"})
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var root map[string]any
	if _, err := toml.Decode(written, &root); err != nil {
		t.Fatalf("not valid TOML: %v\n%s", err, written)
	}
	if got := dig(t, root, "model"); got != "o3" {
		t.Errorf("unrelated keys must be preserved, got %v", got)
	}
	if got := dig(t, root, "mcp_servers", serverName, "command"); got != "/usr/bin/mcp-ai-helper" {
		t.Errorf("command: got %v", got)
	}
	args, ok := dig(t, root, "mcp_servers", serverName, "args").([]any)
	if !ok || len(args) != 2 || args[0] != "--config" {
		t.Errorf("args: got %v", args)
	}
}

func TestCodexToolSettingsForTheHelperSurviveARerun(t *testing.T) {
	// Codex keeps per-tool approval under the server's own table. Those are the
	// user's settings, and a re-registration must not drop them.
	existing := "[mcp_servers.\"" + serverName + "\".tools.command]\napproval_mode = \"approve\"\n"
	written, err := mergeCodexTOML(existing, "/usr/bin/mcp-ai-helper", nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	var root map[string]any
	if _, err := toml.Decode(written, &root); err != nil {
		t.Fatalf("not valid TOML: %v\n%s", err, written)
	}
	if got := dig(t, root, "mcp_servers", serverName, "tools", "command", "approval_mode"); got != "approve" {
		t.Errorf("per-tool settings must survive re-registration, got %v in:\n%s", got, written)
	}
}

func TestOpencodeGetsCommandAsOneArgvArray(t *testing.T) {
	written, err := mergeJSON("", "mcp", opencodeEntry("/usr/bin/mcp-ai-helper", []string{"--config", "/etc/helper.yaml"}))
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	root := parseJSON(t, written)
	if got := dig(t, root, "mcp", serverName, "type"); got != "local" {
		t.Errorf("type: got %v", got)
	}
	if got := dig(t, root, "mcp", serverName, "enabled"); got != true {
		t.Errorf("enabled: got %v", got)
	}
	argv, ok := dig(t, root, "mcp", serverName, "command").([]any)
	if !ok || len(argv) != 3 || argv[0] != "/usr/bin/mcp-ai-helper" || argv[1] != "--config" {
		t.Errorf("command: got %v", argv)
	}
}
