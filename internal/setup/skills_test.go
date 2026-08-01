package setup

import (
	"regexp"
	"strings"
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// TestEverySkillSatisfiesTheAgentSkillsFrontmatterContract checks the tighter of
// the two clients' bounds, so a skill that passes here loads in both.
func TestEverySkillSatisfiesTheAgentSkillsFrontmatterContract(t *testing.T) {
	validName := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	for _, s := range skills {
		if len(s.name) == 0 || len(s.name) > 64 || !validName.MatchString(s.name) {
			t.Errorf("%q is not a valid skill name", s.name)
		}

		rest, ok := strings.CutPrefix(s.body, "---\n")
		if !ok {
			t.Fatalf("%s has no frontmatter", s.name)
		}
		front, _, ok := strings.Cut(rest, "\n---\n")
		if !ok {
			t.Fatalf("%s has no frontmatter terminator", s.name)
		}

		named := frontmatterField(front, "name: ")
		if named != s.name {
			t.Errorf("%s: the frontmatter name must repeat the directory name, got %q", s.name, named)
		}

		description := frontmatterField(front, "description: ")
		if len(description) == 0 || len(description) > 1024 {
			t.Errorf("%s: a description is capped at 1024 characters, this one is %d", s.name, len(description))
		}
	}
}

func frontmatterField(front string, prefix string) string {
	for _, line := range strings.Split(front, "\n") {
		if value, ok := strings.CutPrefix(line, prefix); ok {
			return value
		}
	}
	return ""
}

// TestEverySkillOnDiskIsInstalled catches the failure the embedded layout makes
// possible: a skill added under skills/, reviewed, merged, and never installed
// because nothing named it in skillNames.
func TestEverySkillOnDiskIsInstalled(t *testing.T) {
	dirs, err := embeddedSkillDirs()
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range dirs {
		if !contains(skillNames, dir) {
			t.Errorf("skills/%s is embedded but missing from skillNames, so it is never installed", dir)
		}
	}
	if len(dirs) != len(skillNames) {
		t.Errorf("skills/ holds %d directories, skillNames lists %d", len(dirs), len(skillNames))
	}
}

func TestTheSkillsHaveDistinctNames(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range skills {
		if seen[s.name] {
			t.Errorf("two skills both called %s", s.name)
		}
		seen[s.name] = true
	}
}

// toolActions pins what the action-dispatch tools actually accept. It is the
// enum in internal/mcp, restated here so that guidance promising an action the
// server does not have fails a test rather than sending a model into an error it
// cannot recover from.
var toolActions = map[string][]string{
	"file":    {"read", "read_many", "list", "search", "snapshot"},
	"edit":    {"replace", "write"},
	"command": {"run", "cleanup", "abort", "list", "get", "filter", "health"},
	"git": {"status", "diff", "commit", "log", "log_diff", "stash_list", "branch_list",
		"remote_list", "tag_list", "blame", "prepare_task_worktree"},
	"task": {"current", "get", "list", "search", "upsert", "set_status", "batch_upsert", "delete"},
	"run":  {"pipeline", "workflow", "schema"},
}

// workflowSteps pins the step types run action=workflow dispatches on.
var workflowSteps = []string{
	"command", "guarded_replace", "task_batch_upsert", "task_transition",
	"git_commit_owned", "git_prepare_task_worktree",
}

var actionMention = regexp.MustCompile(`([a-z_]+) action=([a-z_]+)`)

func TestNoInstalledTextNamesAnActionTheToolsDoNotHave(t *testing.T) {
	texts := map[string]string{
		"instructions block": blockBody,
		// The guidance texts belong here for the same reason the skills do, and
		// more urgently: they are written into a user's config file on first run
		// and read back from there at run time, so a stale action name in them
		// keeps reaching models long after this repository moved on.
		"default assistant guidance": config.DefaultAssistantGuidance(),
	}
	for key, value := range config.SetupGuidance("") {
		texts["server setup guidance: "+key] = value
	}
	for _, s := range skills {
		texts[s.name] = s.body
	}

	for where, text := range texts {
		for _, match := range actionMention.FindAllStringSubmatch(text, -1) {
			tool, action := match[1], match[2]
			known, ok := toolActions[tool]
			if !ok {
				t.Errorf("%s mentions tool %q, which is not one of %v", where, tool, keysOf(toolActions))
				continue
			}
			if !contains(known, action) {
				t.Errorf("%s mentions %s action=%s, which is not one of %v", where, tool, action, known)
			}
		}
	}
}

func TestTheTasksSkillNamesOnlyRealWorkflowSteps(t *testing.T) {
	// The skill lists the workflow step types by name, which is exactly the
	// place a stale name would send a model into an unrecoverable step error.
	body := skills[0].body
	if skills[0].name != "mcp-ai-helper-tasks" {
		t.Fatalf("expected the tasks skill first, got %s", skills[0].name)
	}
	for _, step := range workflowSteps {
		if !strings.Contains(body, step) {
			t.Errorf("the tasks skill should name the %q workflow step", step)
		}
	}
	// And the reverse, over the section that presents the steps: every
	// snake_case identifier there is claimed to be a step, so every one of them
	// has to be. Elsewhere in the skill the same shape is a tool name.
	closing := section(body, "## Closing a task")
	if closing == "" {
		t.Fatal("the tasks skill should have a 'Closing a task' section")
	}
	for _, match := range regexp.MustCompile("`([a-z]+_[a-z_]+)`").FindAllStringSubmatch(closing, -1) {
		if !contains(workflowSteps, match[1]) && !contains([]string{"owned_files", "current_task_id"}, match[1]) {
			t.Errorf("the closing section names %q, which is neither a workflow step %v nor one of its arguments", match[1], workflowSteps)
		}
	}
}

// section returns the body of a markdown heading, up to the next one at any
// level.
func section(text string, heading string) string {
	_, rest, ok := strings.Cut(text, heading)
	if !ok {
		return ""
	}
	if before, _, ok := strings.Cut(rest, "\n## "); ok {
		return before
	}
	return rest
}

func TestTheInstructionsBlockStaysShortEnoughToLoadEverySession(t *testing.T) {
	// The block is paid for on every session in every repo the helper serves, so
	// growth here is growth everywhere. The skills are where detail belongs.
	if lines := strings.Count(blockBody, "\n") + 1; lines > 45 {
		t.Errorf("the instructions block is %d lines; move detail into a skill", lines)
	}
}

func TestEverySkillHasCodexMetadata(t *testing.T) {
	for _, skill := range skills {
		for _, field := range []string{"display_name: \"", "short_description: \"", "default_prompt: \""} {
			if !strings.Contains(skill.agent, field) {
				t.Errorf("%s metadata lacks %s", skill.name, field)
			}
		}
		if !strings.Contains(skill.agent, "$"+skill.name) {
			t.Errorf("%s default prompt must invoke the skill by name", skill.name)
		}
	}
}

func TestSkillsCoverTheFailureProneHelperWorkflows(t *testing.T) {
	required := map[string][]string{
		"mcp-ai-helper-tasks": {
			"tool_manifest", "task action=current", "task action=get", "git action=status",
			"run action=schema", "run action=workflow", "task_transition", "git_commit_owned",
			"surface_mismatch", "command_id",
		},
		"mcp-ai-helper-edits": {
			"file action=read", "file action=snapshot", "edit action=replace", "edit action=write",
			"expected_hash", "run action=schema", "git action=commit", "owned_files", "surface_mismatch",
		},
		"mcp-ai-helper-commands": {
			"command action=run", "command action=get", "command action=filter", "command action=abort",
			"command_id", "running", "timeout", "truncation", "omission", "surface mismatch",
		},
		"mcp-ai-helper-web": {
			"web_search", "web_fetch", "fetched_doc_find", "fetched_doc_read",
			"doc_id", "offsets", "completeness", "insufficient_evidence", "tool_manifest",
		},
		"mcp-ai-helper-surface": {
			"tool_manifest", "assistant_guidance", "task_advanced", "git_advanced",
			"config_read", "config_schema", "config_option_set", "feature_enable",
			"config_reload", "restart", "task_registry_init", "surface_mismatch",
		},
	}
	for name, fragments := range required {
		body := ""
		for _, skill := range skills {
			if skill.name == name {
				body = skill.body
				break
			}
		}
		if body == "" {
			t.Fatalf("missing required skill %s", name)
		}
		for _, fragment := range fragments {
			if !strings.Contains(body, fragment) {
				t.Errorf("%s lacks required guidance %q", name, fragment)
			}
		}
	}
}

func keysOf(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		out = append(out, key)
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, item := range haystack {
		if item == needle {
			return true
		}
	}
	return false
}
