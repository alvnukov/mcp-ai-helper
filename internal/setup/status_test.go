package setup

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatusReportsNothingInstalledInAFreshProject(t *testing.T) {
	sandbox(t)

	var out bytes.Buffer
	current, err := Status(Options{Clients: []string{"claude"}}, &out)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if current {
		t.Error("a project nothing was installed into cannot be current")
	}
	for _, want := range []string{"not registered", "no helper block", "missing: " + skills[0].name} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("status should report %q:\n%s", want, out.String())
		}
	}
}

func TestStatusAgreesWithSetupItJustRan(t *testing.T) {
	sandbox(t)
	opts := Options{Clients: []string{"claude"}}
	if err := Run(opts, &bytes.Buffer{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	var out bytes.Buffer
	current, err := Status(opts, &out)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if !current {
		t.Errorf("status right after a successful setup must be current:\n%s", out.String())
	}
	for _, wrong := range []string{"missing", "out of date", "not registered"} {
		if strings.Contains(out.String(), wrong) {
			t.Errorf("status should not report %q here:\n%s", wrong, out.String())
		}
	}
}

// TestStatusNoticesASkillAnOlderBuildWrote is the failure the whole command
// exists for: the file is there, it is a valid skill, and it says something this
// build no longer means.
func TestStatusNoticesASkillAnOlderBuildWrote(t *testing.T) {
	project := sandbox(t)
	opts := Options{Clients: []string{"claude"}}
	if err := Run(opts, &bytes.Buffer{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	stale := filepath.Join(project, ".claude", "skills", skills[0].name, "SKILL.md")
	if err := os.WriteFile(stale, []byte("---\nname: "+skills[0].name+"\ndescription: old\n---\n"), 0o600); err != nil {
		t.Fatalf("age the skill: %v", err)
	}

	var out bytes.Buffer
	current, err := Status(opts, &out)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if current {
		t.Error("a skill whose text this build would not write is not current")
	}
	if !strings.Contains(out.String(), "out of date: "+skills[0].name) {
		t.Errorf("status should name the stale skill:\n%s", out.String())
	}
}

func TestStatusNoticesAnInstructionsBlockThatDrifted(t *testing.T) {
	project := sandbox(t)
	opts := Options{Clients: []string{"claude"}}
	if err := Run(opts, &bytes.Buffer{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	drifted := blockStart + "\n## mcp-ai-helper\n\nSomething an older build said.\n" + blockEnd + "\n"
	if err := os.WriteFile(filepath.Join(project, "CLAUDE.md"), []byte(drifted), 0o600); err != nil {
		t.Fatalf("age the block: %v", err)
	}

	var out bytes.Buffer
	current, err := Status(opts, &out)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if current {
		t.Error("a block this build would rewrite is not current")
	}
	if !strings.Contains(out.String(), "out of date") {
		t.Errorf("status should report the block as out of date:\n%s", out.String())
	}
}

// TestStatusNoticesAnEntryPointingAtAHelperThatIsGone covers the install that
// looks perfect from inside the client: the entry is there, the client tries to
// start it, and all the model ever learns is that the tools are missing.
func TestStatusNoticesAnEntryPointingAtAHelperThatIsGone(t *testing.T) {
	project := sandbox(t)

	entry, err := mergeClaudeJSON("", filepath.Join(project, "nowhere", "mcp-ai-helper"), "")
	if err != nil {
		t.Fatalf("build entry: %v", err)
	}
	if err := os.WriteFile(filepath.Join(project, ".mcp.json"), []byte(entry), 0o600); err != nil {
		t.Fatalf("write entry: %v", err)
	}

	var out bytes.Buffer
	current, err := Status(Options{Clients: []string{"claude"}}, &out)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if current {
		t.Error("an entry naming a binary that is not there is not current")
	}
	if !strings.Contains(out.String(), "is gone") {
		t.Errorf("status should say the recorded helper is gone:\n%s", out.String())
	}
}

// TestRegisteredCommandReadsEveryClientsShape keeps the status check honest for
// the two clients this repository is not usually run from.
func TestRegisteredCommandReadsEveryClientsShape(t *testing.T) {
	for _, spec := range clients {
		var existing string
		var err error
		switch spec.format {
		case claudeJSON:
			existing, err = mergeClaudeJSON("", "/opt/mcp-ai-helper", "")
		case opencodeJSON:
			existing, err = mergeOpenCodeJSON("", "/opt/mcp-ai-helper", "")
		case codexTOML:
			existing, err = mergeCodexTOML("", "/opt/mcp-ai-helper", nil)
		}
		if err != nil {
			t.Fatalf("%s: build entry: %v", spec.id, err)
		}

		command, registered, err := registeredCommand(existing, spec.format)
		if err != nil {
			t.Fatalf("%s: read entry: %v", spec.id, err)
		}
		if !registered || command != "/opt/mcp-ai-helper" {
			t.Errorf("%s: got (%q, %v), want the command the entry was built with", spec.id, command, registered)
		}

		if _, registered, err := registeredCommand("", spec.format); err != nil || registered {
			t.Errorf("%s: an empty config registers nothing, got (%v, %v)", spec.id, registered, err)
		}
	}
}

// TestRegisteredCommandReadsAJSONCConfig keeps status honest on the configs
// OpenCode actually accepts — comments and trailing commas are legal there —
// so reading the entry must not die on them the way a strict parser does.
func TestRegisteredCommandReadsAJSONCConfig(t *testing.T) {
	existing := "{\n  // pinned by hand\n  \"mcp\": {\"" + serverName + "\": {\"command\": [\"/opt/mcp-ai-helper\", \"--repo\", \"/r\"],},}\n}\n"
	command, registered, err := registeredCommand(existing, opencodeJSON)
	if err != nil || !registered || command != "/opt/mcp-ai-helper" {
		t.Errorf("got (%q, %v, %v), want (/opt/mcp-ai-helper, true, nil)", command, registered, err)
	}
}
