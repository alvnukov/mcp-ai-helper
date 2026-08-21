package setup

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEveryKnownClientResolvesAllOfItsPaths(t *testing.T) {
	for _, spec := range clients {
		for _, global := range []bool{false, true} {
			if _, err := configPath(spec, global); err != nil {
				t.Errorf("no server config path for %s (global=%v): %v", spec.id, global, err)
			}
			if _, err := spec.guidance.resolve(global); err != nil {
				t.Errorf("no instructions path for %s (global=%v): %v", spec.id, global, err)
			}
			if spec.skills != nil {
				if _, err := skillsPath(spec, global); err != nil {
					t.Errorf("no skills path for %s (global=%v): %v", spec.id, global, err)
				}
			}
		}
	}
}

func TestClaudeAndCodexDoNotShareAnInstructionsFile(t *testing.T) {
	// Claude Code reads CLAUDE.md and not AGENTS.md, so pointing both at one
	// file would leave one of them with no instructions at all.
	claude, err := client("claude")
	if err != nil {
		t.Fatalf("claude: %v", err)
	}
	codex, err := client("codex")
	if err != nil {
		t.Fatalf("codex: %v", err)
	}
	if claude.guidance.project == codex.guidance.project {
		t.Fatalf("both clients read %s for instructions", claude.guidance.project)
	}
}

func TestAnUnknownClientNamesTheOnesThatExist(t *testing.T) {
	_, err := client("cursor")
	if err == nil {
		t.Fatal("an unknown client must be refused")
	}
	for _, known := range clients {
		if !strings.Contains(err.Error(), known.id) {
			t.Errorf("%q should mention %s", err, known.id)
		}
	}
}

func TestAMissingFileReadsAsAnEmptyOne(t *testing.T) {
	text, err := readConfig(filepath.Join(t.TempDir(), "nothing-here.json"))
	if err != nil || text != "" {
		t.Fatalf("missing file: got (%q, %v), want (\"\", nil)", text, err)
	}
}

func TestAFileThatCannotBeReadIsNotMistakenForAnEmptyOne(t *testing.T) {
	// A directory standing where the file should be is a read failure that is
	// not NotExist, and unlike a permission bit it behaves the same whichever
	// user runs the tests.
	_, err := readConfig(t.TempDir())
	if err == nil {
		t.Fatal("a directory is not readable as a config")
	}
	if !strings.Contains(err.Error(), "read ") {
		t.Fatalf("the failure must name the file it could not read: %v", err)
	}
}

func TestWritingTheSameTextTwiceTouchesTheFileOnce(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	hello, goodbye := "hello\n", "goodbye\n"

	if result, err := apply(path, &hello, false); err != nil || result != outcomeDone {
		t.Fatalf("first write: got (%v, %v), want (done, nil)", result, err)
	}
	first, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	if result, err := apply(path, &hello, false); err != nil || result != outcomeCurrent {
		t.Fatalf("a second identical write must be skipped: got (%v, %v)", result, err)
	}
	second, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if !second.ModTime().Equal(first.ModTime()) {
		t.Fatal("the file must not even be touched")
	}

	if result, err := apply(path, &goodbye, false); err != nil || result != outcomeDone {
		t.Fatalf("different content must still be written: got (%v, %v)", result, err)
	}
	if data, err := os.ReadFile(path); err != nil || string(data) != goodbye {
		t.Fatalf("read back: got (%q, %v), want %q", data, err, goodbye)
	}
}

func TestADryRunReportsWithoutWriting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	hello := "hello\n"
	if result, err := apply(path, &hello, true); err != nil || result != outcomeWould {
		t.Fatalf("dry run: got (%v, %v), want (would, nil)", result, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("a dry run must not create the file")
	}
}

func TestAFileLeftHoldingNothingIsTakenAway(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{\n  \"mcpServers\": {}\n}\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	empty := ""
	if result, err := apply(path, &empty, false); err != nil || result != outcomeDone {
		t.Fatalf("emptying: got (%v, %v), want (done, nil)", result, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("an emptied config is a husk, not a configuration")
	}
	// And asking again is quiet, because absent already reads as empty.
	if result, err := apply(path, &empty, false); err != nil || result != outcomeCurrent {
		t.Fatalf("re-asking: got (%v, %v), want (current, nil)", result, err)
	}
}

func TestNothingToWriteIsNotAWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if result, err := apply(path, nil, false); err != nil || result != outcomeNothing {
		t.Fatalf("nothing: got (%v, %v), want (nothing, nil)", result, err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("nothing to write must not create the file")
	}
}

func TestLaunchArgsCarryAPinnedConfigForward(t *testing.T) {
	if args := launchArgs(""); args != nil {
		t.Errorf("an unpinned config must leave the flag off, got %v", args)
	}
	if args := launchArgs("  "); args != nil {
		t.Errorf("a blank config must leave the flag off, got %v", args)
	}
	args := launchArgs("/etc/helper.yaml")
	if len(args) != 2 || args[0] != "--config" || args[1] != "/etc/helper.yaml" {
		t.Errorf("pinned config: got %v", args)
	}
}

type errorWriter struct{}

func (errorWriter) Write([]byte) (int, error) {
	return 0, io.ErrClosedPipe
}

func TestRunReturnsWriterError(t *testing.T) {
	sandbox(t)
	err := Run(Options{Clients: []string{"claude"}, DryRun: true}, errorWriter{})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Run error = %v, want wrapped io.ErrClosedPipe", err)
	}
}

func TestRemoveReturnsWriterError(t *testing.T) {
	sandbox(t)
	err := Remove(Options{Clients: []string{"claude"}}, errorWriter{})
	if !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Remove error = %v, want wrapped io.ErrClosedPipe", err)
	}
}

// sandbox points HOME and the working directory at a temp tree, so a test can
// run the real commands without touching the machine's own client configs.
func sandbox(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("HOME", root)
	project := filepath.Join(root, "project")
	if err := os.MkdirAll(project, 0o700); err != nil {
		t.Fatalf("create project: %v", err)
	}
	t.Chdir(project)
	return project
}

func TestSetupThenRemoveLeavesTheProjectAsItWas(t *testing.T) {
	project := sandbox(t)
	if err := os.WriteFile(filepath.Join(project, "CLAUDE.md"), []byte("# My rules\n"), 0o600); err != nil {
		t.Fatalf("seed CLAUDE.md: %v", err)
	}
	opts := Options{Clients: []string{"claude"}}

	var out bytes.Buffer
	if err := Run(opts, &out); err != nil {
		t.Fatalf("setup: %v", err)
	}
	paths := []string{".mcp.json"}
	for _, skill := range skills {
		for _, installed := range skill.files() {
			paths = append(paths, filepath.Join(".claude", "skills", skill.name, installed.path))
		}
	}
	for _, path := range paths {
		if _, err := os.Stat(filepath.Join(project, path)); err != nil {
			t.Errorf("setup should have written %s: %v", path, err)
		}
	}
	guidance, err := os.ReadFile(filepath.Join(project, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("read CLAUDE.md: %v", err)
	}
	if !strings.Contains(string(guidance), blockStart) {
		t.Error("setup should have added the instructions block")
	}

	// A second setup is quiet, which is the property the whole design exists for.
	out.Reset()
	if err := Run(opts, &out); err != nil {
		t.Fatalf("second setup: %v", err)
	}
	if strings.Count(out.String(), "already up to date") != 3 {
		t.Errorf("a re-run must report every part as current:\n%s", out.String())
	}

	out.Reset()
	if err := Remove(opts, &out); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, ".mcp.json")); !os.IsNotExist(err) {
		t.Error("a config left holding only the helper must be taken away")
	}
	if _, err := os.Stat(filepath.Join(project, ".claude")); !os.IsNotExist(err) {
		t.Error("the skills directory the helper created must go with it")
	}
	if data, err := os.ReadFile(filepath.Join(project, "CLAUDE.md")); err != nil || string(data) != "# My rules\n" {
		t.Errorf("CLAUDE.md after remove: got (%q, %v), want %q", data, err, "# My rules\n")
	}
}

func TestRemoveLeavesSkillsSomebodyElseInstalled(t *testing.T) {
	project := sandbox(t)
	opts := Options{Clients: []string{"claude"}}
	if err := Run(opts, &bytes.Buffer{}); err != nil {
		t.Fatalf("setup: %v", err)
	}

	theirs := filepath.Join(project, ".claude", "skills", "somebody-elses", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(theirs), 0o700); err != nil {
		t.Fatalf("create their skill: %v", err)
	}
	if err := os.WriteFile(theirs, []byte("---\nname: somebody-elses\n---\n"), 0o600); err != nil {
		t.Fatalf("write their skill: %v", err)
	}

	if err := Remove(opts, &bytes.Buffer{}); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if _, err := os.Stat(theirs); err != nil {
		t.Errorf("a skill the helper did not install must survive its uninstall: %v", err)
	}
}

func TestRemovingWhatWasNeverInstalledChangesNothing(t *testing.T) {
	project := sandbox(t)
	var out bytes.Buffer
	if err := Remove(Options{Clients: []string{"claude"}}, &out); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if strings.Contains(out.String(), "Restart the client") {
		t.Errorf("nothing changed, so nothing needs restarting:\n%s", out.String())
	}
	entries, err := os.ReadDir(project)
	if err != nil {
		t.Fatalf("read project: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("removing from an untouched project must create nothing, got %v", entries)
	}
}

func TestADryRunOfSetupWritesNothing(t *testing.T) {
	project := sandbox(t)
	var out bytes.Buffer
	if err := Run(Options{Clients: []string{"claude", "opencode"}, DryRun: true}, &out); err != nil {
		t.Fatalf("dry run: %v", err)
	}
	entries, err := os.ReadDir(project)
	if err != nil {
		t.Fatalf("read project: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("a dry run must write nothing, got %v", entries)
	}
	if !strings.Contains(out.String(), "would be") {
		t.Errorf("a dry run must report what it would do:\n%s", out.String())
	}
	if strings.Contains(out.String(), "Restart the client") {
		t.Errorf("a dry run changed nothing, so nothing needs restarting:\n%s", out.String())
	}
}

func TestCodexUsesUserConfigAndInstallsUserWideSkills(t *testing.T) {
	sandbox(t)
	codex, err := client("codex")
	if err != nil {
		t.Fatalf("codex: %v", err)
	}
	project, err := configPath(codex, false)
	if err != nil {
		t.Fatalf("project path: %v", err)
	}
	global, err := configPath(codex, true)
	if err != nil {
		t.Fatalf("global path: %v", err)
	}
	if project != global {
		t.Errorf("Codex has no project config scope: %s != %s", project, global)
	}
	projectSkills, err := skillsPath(codex, false)
	if err != nil {
		t.Fatalf("project skills path: %v", err)
	}
	globalSkills, err := skillsPath(codex, true)
	if err != nil {
		t.Fatalf("global skills path: %v", err)
	}
	if projectSkills != globalSkills {
		t.Errorf("Codex skills are user-wide: %s != %s", projectSkills, globalSkills)
	}

	var out bytes.Buffer
	if err := Run(Options{Clients: []string{"codex"}}, &out); err != nil {
		t.Fatalf("setup: %v", err)
	}
	for _, skill := range skills {
		for _, installed := range skill.files() {
			path := filepath.Join(globalSkills, skill.name, installed.path)
			if _, err := os.Stat(path); err != nil {
				t.Errorf("setup should have installed %s: %v", path, err)
			}
		}
	}
	if strings.Contains(out.String(), "skipped") {
		t.Errorf("Codex skills must not be skipped:\n%s", out.String())
	}
}

// TestOpenCodeConfigIsEditedInPlace runs the whole command against a config
// the way a user really has one: a comment, a hand-pinned flag, a neighbour
// on one line. The file that comes back must differ only inside the helper's
// own entry.
func TestOpenCodeConfigIsEditedInPlace(t *testing.T) {
	project := sandbox(t)
	seed := `{
  "$schema": "https://opencode.ai/config.json",
  // my tuning, do not touch
  "mcp": {
    "happ": {"type": "local", "command": ["/opt/homebrew/bin/happ", "mcp"]},
    "mcp-ai-helper": {
      "type": "local",
      "command": ["/old/bin/mcp-ai-helper", "--repo", "/r"],
      "enabled": true,
      "timeout": 600000
    }
  }
}
`
	if err := os.WriteFile(filepath.Join(project, "opencode.json"), []byte(seed), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := Run(Options{Clients: []string{"opencode"}, NoInstructions: true, NoSkills: true}, &bytes.Buffer{}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	after, err := os.ReadFile(filepath.Join(project, "opencode.json"))
	if err != nil {
		t.Fatalf("read back: %v", err)
	}

	for _, want := range []string{
		"// my tuning, do not touch",
		`"happ": {"type": "local", "command": ["/opt/homebrew/bin/happ", "mcp"]}`,
		`"timeout": 600000`,
		`"--repo"`,
		`"/r"`,
	} {
		if !strings.Contains(string(after), want) {
			t.Errorf("setup rewrote more than its own entry; lost %q:\n%s", want, after)
		}
	}
	if strings.Contains(string(after), "/old/bin/mcp-ai-helper") {
		t.Errorf("the stale binary must be re-pinned:\n%s", after)
	}
}

// TestOpenCodeUsesAnExistingJsoncConfig pins the file choice: a user with an
// opencode.jsonc gets it edited where it lies, not a second config written
// beside it.
func TestOpenCodeUsesAnExistingJsoncConfig(t *testing.T) {
	project := sandbox(t)
	jsonc := filepath.Join(project, "opencode.jsonc")
	if err := os.WriteFile(jsonc, []byte("{\n  // hand written\n}\n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if err := Run(Options{Clients: []string{"opencode"}, NoInstructions: true, NoSkills: true}, &bytes.Buffer{}); err != nil {
		t.Fatalf("setup: %v", err)
	}
	if _, err := os.Stat(filepath.Join(project, "opencode.json")); !os.IsNotExist(err) {
		t.Error("a jsonc config must not gain a competing opencode.json")
	}
	after, err := os.ReadFile(jsonc)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(after), serverName) || !strings.Contains(string(after), "// hand written") {
		t.Errorf("the jsonc config should have been edited in place:\n%s", after)
	}
}
