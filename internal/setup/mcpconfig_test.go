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
	written, err := mergeClaudeJSON("", "/usr/bin/mcp-ai-helper", "/etc/helper.yaml")
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
	written, err := mergeClaudeJSON("", "mcp-ai-helper", "")
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
	written, err := mergeClaudeJSON(existing, "mcp-ai-helper", "")
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
	// And not by reformatting the file around it: the neighbour's bytes and
	// the tail of the file stand unchanged; only a new member went in.
	if !strings.Contains(written, `"other":{"command":"other-server"}`) ||
		!strings.HasSuffix(strings.TrimRight(written, "\n"), `,"theme":"dark"}`) {
		t.Errorf("the file's own bytes must survive the merge:\n%s", written)
	}
}

// TestSetupDoesNotRewriteTheConfigItRegistersIn is the failure the surgical
// editor exists for: a config OpenCode accepts — JSONC, hand-ordered, with
// one-line entries and flags the user added by hand — must come back from a
// registration byte-identical everywhere outside the helper's own stanza.
func TestSetupDoesNotRewriteTheConfigItRegistersIn(t *testing.T) {
	existing := `{
  "$schema": "https://opencode.ai/config.json",
  "model": "zai/glm-5.3",
  // my tuning, do not touch
  "mcp": {
    "happ": {"type": "local", "command": ["/opt/homebrew/bin/happ", "mcp"]},
    "mcp-ai-helper": {
      "type": "local",
      "command": [
        "/old/bin/mcp-ai-helper",
        "--repo",
        "/Users/zol/src/mcp-ai-helper"
      ],
      "enabled": true,
      "timeout": 600000
    }
  },
  "tools": {"bash": false}
}
`
	written, err := mergeOpenCodeJSON(existing, "/new/bin/mcp-ai-helper", "")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}

	// What the user owns byte-for-byte: the comment, the order, the
	// one-liners, their settings.
	for _, want := range []string{
		"// my tuning, do not touch",
		`"happ": {"type": "local", "command": ["/opt/homebrew/bin/happ", "mcp"]}`,
		`"timeout": 600000`,
		`"tools": {"bash": false}`,
	} {
		if !strings.Contains(written, want) {
			t.Errorf("the merge lost %q:\n%s", want, written)
		}
	}
	if strings.Index(written, `"$schema"`) > strings.Index(written, `"model"`) ||
		strings.Index(written, `"model"`) > strings.Index(written, `"mcp"`) {
		t.Errorf("the merge must not reorder the file:\n%s", written)
	}
	// What the helper owns: the binary is re-pinned, and the user's --repo
	// survives after it. The result is JSONC now, so the helper's own
	// scanner — not a strict parser — is what reads it back.
	command, registered, err := registeredJSONCommand(written, "mcp")
	if err != nil || !registered || command != "/new/bin/mcp-ai-helper" {
		t.Errorf("the re-pinned binary should read back: got (%q, %v, %v)", command, registered, err)
	}
	if !strings.Contains(written, `"command": ["/new/bin/mcp-ai-helper","--repo","/Users/zol/src/mcp-ai-helper"],`) {
		t.Errorf("the user's --repo must survive after the binary:\n%s", written)
	}

	// And a second registration is a fixed point: apply() compares against
	// the file, and this equality is what keeps a re-run a no-op.
	again, err := mergeOpenCodeJSON(written, "/new/bin/mcp-ai-helper", "")
	if err != nil {
		t.Fatalf("re-merge: %v", err)
	}
	if again != written {
		t.Errorf("merge is not a fixed point:\n%s\n---\n%s", written, again)
	}
}

// TestTheConfigPairIsTheHelpersToManage pins the --config flag's lifecycle:
// set when setup names a config, replaced when it names another, gone when it
// names none — and never duplicated.
func TestTheConfigPairIsTheHelpersToManage(t *testing.T) {
	pinned := `{"mcp":{"` + serverName + `":{"command":["/bin/h","--config","/old.yaml","--repo","/r"]}}}`
	written, err := mergeOpenCodeJSON(pinned, "/bin/h", "/new.yaml")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	argv, _ := dig(t, parseJSON(t, written), "mcp", serverName, "command").([]any)
	if len(argv) != 5 || argv[0] != "/bin/h" || argv[1] != "--config" || argv[2] != "/new.yaml" || argv[3] != "--repo" || argv[4] != "/r" {
		t.Errorf("a new config path must replace the old pair in place: got %v", argv)
	}

	written, err = mergeOpenCodeJSON(pinned, "/bin/h", "")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	argv, _ = dig(t, parseJSON(t, written), "mcp", serverName, "command").([]any)
	if len(argv) != 3 || argv[0] != "/bin/h" || argv[1] != "--repo" {
		t.Errorf("an unpinned setup must drop the pair but keep user flags: got %v", argv)
	}
}

// TestAJSONCConfigIsEditedNotRefused covers the case that used to die with
// "existing config is not valid JSON": OpenCode documents JSONC, so comments
// and trailing commas are a legal config, and setup has to live with them.
func TestAJSONCConfigIsEditedNotRefused(t *testing.T) {
	existing := "{\n  // providers live here\n  \"mcp\": {\n    \"happ\": {},\n  },\n}\n"
	written, err := mergeOpenCodeJSON(existing, "/bin/h", "")
	if err != nil {
		t.Fatalf("a JSONC config must merge, got: %v", err)
	}
	if !strings.Contains(written, "// providers live here") {
		t.Errorf("the comment must survive:\n%s", written)
	}
	// The result is JSONC now, so strict JSON parsing is the wrong judge;
	// reading it back through the helper's own scanner is the honest one.
	command, registered, err := registeredJSONCommand(written, "mcp")
	if err != nil || !registered || command != "/bin/h" {
		t.Errorf("the helper should have been added and readable: got (%q, %v, %v)", command, registered, err)
	}
}

func TestALargeNumberSurvivesTheRoundTrip(t *testing.T) {
	// A splice cannot round anything through float64; this is a promise the
	// surgical editor makes about every value it does not own.
	existing := `{"sessionId":9007199254740993,"mcpServers":{}}`
	written, err := mergeClaudeJSON(existing, "mcp-ai-helper", "")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if !strings.Contains(written, "9007199254740993") {
		t.Errorf("a large integer must survive verbatim:\n%s", written)
	}
}

func TestReRunningSetupReplacesOnlyTheHelperEntry(t *testing.T) {
	first, err := mergeClaudeJSON("", "old", "")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	second, err := mergeClaudeJSON(first, "new", "")
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
	// The caller compares against the file to decide whether to write, so
	// this equality is what makes a repeated setup a no-op.
	for _, merge := range []struct {
		name string
		call func(string) (string, error)
	}{
		{"claude", func(in string) (string, error) { return mergeClaudeJSON(in, "/usr/bin/mcp-ai-helper", "") }},
		{"opencode", func(in string) (string, error) { return mergeOpenCodeJSON(in, "/usr/bin/mcp-ai-helper", "") }},
	} {
		once, err := merge.call("")
		if err != nil {
			t.Fatalf("%s merge: %v", merge.name, err)
		}
		twice, err := merge.call(once)
		if err != nil {
			t.Fatalf("%s re-merge: %v", merge.name, err)
		}
		if once != twice {
			t.Errorf("%s merge is not a fixed point:\n%s\n---\n%s", merge.name, once, twice)
		}
	}

	once, err := mergeCodexTOML("model = \"o3\"\n", "/usr/bin/mcp-ai-helper", nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	twice, err := mergeCodexTOML(once, "/usr/bin/mcp-ai-helper", nil)
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	if once != twice {
		t.Errorf("TOML merge is not a fixed point:\n%s\n---\n%s", once, twice)
	}
}

func TestABrokenExistingConfigIsRefusedRatherThanOverwritten(t *testing.T) {
	if _, err := mergeClaudeJSON("{not json", "x", ""); err == nil {
		t.Error("broken JSON must be refused")
	}
	if _, err := mergeClaudeJSON(`["a list"]`, "x", ""); err == nil {
		t.Error("a JSON document that is not an object must be refused")
	}
	if _, err := mergeOpenCodeJSON(`{"mcp": 7}`, "x", ""); err == nil {
		t.Error("a section that is not a map must be refused")
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
	// Surgical means surgical: this fixture's remaining bytes are exactly
	// the ones it had before the helper ever arrived.
	if want := `{"mcpServers":{"other":{"command":"o"}},"theme":"dark"}`; *written != want {
		t.Errorf("remove must restore the file byte for byte:\n%s\nwant:\n%s", *written, want)
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
	// Each of these is a way for the helper to be absent, and none of them is
	// a reason to rewrite — let alone reformat — somebody else's config.
	for _, existing := range []string{
		"",
		`{"theme":"dark"}`,
		`{"mcpServers":{"other":{}}}`,
		"{\n  // only comments and somebody else's keys\n  \"theme\": \"dark\",\n}\n",
	} {
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
	// Byte-exact: install, remove, and the file is the file it was. A
	// canonicalising editor cannot pass this, which is the point.
	original := "{\n  \"theme\": \"dark\",\n  \"mcpServers\": {\"other\": {\"command\": \"o\"}}\n}\n"
	installed, err := mergeClaudeJSON(original, "/usr/bin/mcp-ai-helper", "")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	uninstalled, err := withoutJSON(installed, "mcpServers")
	if err != nil || uninstalled == nil {
		t.Fatalf("remove: got (%v, %v)", uninstalled, err)
	}
	if *uninstalled != original {
		t.Errorf("a round trip must leave the file exactly as it was:\n%q\nwant:\n%q", *uninstalled, original)
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
	written, err := mergeOpenCodeJSON("", "/usr/bin/mcp-ai-helper", "/etc/helper.yaml")
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

// TestAnEntryThatIsNotAnObjectIsReplaced covers the degenerate case: somebody
// once wrote a number where the entry belongs, and setup replaces it wholesale
// rather than trying to merge into it.
func TestAnEntryThatIsNotAnObjectIsReplaced(t *testing.T) {
	existing := `{"mcp":{"` + serverName + `":7,"happ":{}}}`
	written, err := mergeOpenCodeJSON(existing, "/bin/h", "")
	if err != nil {
		t.Fatalf("merge: %v", err)
	}
	root := parseJSON(t, written)
	if _, ok := dig(t, root, "mcp", serverName).(map[string]any); !ok {
		t.Errorf("the entry should have been replaced with a real one:\n%s", written)
	}
	if _, ok := dig(t, root, "mcp", "happ").(map[string]any); !ok {
		t.Errorf("the neighbour must survive:\n%s", written)
	}
}
