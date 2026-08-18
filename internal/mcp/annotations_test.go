package mcp

import (
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

type toolHints struct {
	readOnly    bool
	destructive bool
	idempotent  bool
	openWorld   bool
}

// allLayersConfig turns on every optional layer and integration, so ListTools
// returns the whole surface rather than the default subset. A tool this test
// cannot see is a tool it cannot hold to anything.
func allLayersConfig() *config.Config {
	on := true
	cfg := &config.Config{AssistantGuidance: config.DefaultAssistantGuidance()}
	for _, layer := range []*config.LayerConfig{
		&cfg.Layers.GitAdvanced,
		&cfg.Layers.Web,
		&cfg.Layers.TaskAdvanced,
		&cfg.Layers.TaskUI,
		&cfg.Layers.ConfigAdvanced,
		&cfg.Layers.Lake,
	} {
		layer.Enabled = &on
	}
	cfg.Integrations.Jira = &config.JiraConfig{}
	cfg.Integrations.Confluence = &config.ConfluenceConfig{}
	return cfg
}

// mcp-go's NewTool seeds every annotation field with a non-nil default
// (ReadOnly=false, Destructive=true, Idempotent=false, OpenWorld=true), so a
// nil check can never catch a regression and a tool whose annotations merely
// happen to equal the defaults is indistinguishable from one that set none.
//
// Every registered tool therefore states all four hints, through a profile in
// annotations.go. The values below are written out here rather than read from
// those profiles on purpose: a test that asked annotate() what it produces
// would agree with any change to it. Flip a value in a profile, drop a profile
// from a registration, or add a tool without deciding what it does to the
// world, and this fails.
func TestEveryRegisteredToolPublishesAllFourAnnotations(t *testing.T) {
	t.Parallel()

	groups := []struct {
		hints toolHints
		why   string
		tools []string
	}{
		{
			toolHints{readOnly: true, destructive: false, idempotent: true, openWorld: false},
			"answers from this machine and writes nothing",
			[]string{
				"file", "assistant_guidance", "server_setup_guidance", "tool_manifest",
				"config_schema", "config_read", "feature_list", "feature_get",
				"language_profiles", "language_detect", "health", "list_models",
				"fetched_doc_read", "fetched_doc_find",
			},
		},
		{
			toolHints{readOnly: true, destructive: false, idempotent: true, openWorld: true},
			"answers from a remote system and writes nothing",
			[]string{
				"conf_search", "conf_read", "conf_spaces",
				"jira_search", "jira_read", "jira_get_property",
				"jira_worklog_list", "jira_worklog_report",
				"query_model", "web_search",
			},
		},
		{
			toolHints{readOnly: false, destructive: false, idempotent: true, openWorld: false},
			"converges local state without overwriting; the task readers auto-heal the notes they scan",
			[]string{"issue_list", "task_graph", "task_context", "lake_init", "task_ui_start"},
		},
		{
			toolHints{readOnly: false, destructive: false, idempotent: false, openWorld: false},
			"appends local state",
			[]string{"issue_add"},
		},
		{
			toolHints{readOnly: false, destructive: true, idempotent: true, openWorld: false},
			"overwrites local state with a value the caller names",
			[]string{
				"task_registry_init", "config_reload", "config_option_set", "config_option_reset",
				"feature_enable", "feature_disable", "feature_reset",
				"issue_accept", "task_export", "task_ui_stop",
			},
		},
		{
			toolHints{readOnly: false, destructive: true, idempotent: false, openWorld: false},
			"overwrites local state, and what it does depends on the state it finds",
			[]string{"edit", "git", "task"},
		},
		{
			toolHints{readOnly: false, destructive: false, idempotent: false, openWorld: true},
			"reaches a remote system and only appends",
			[]string{"web_fetch", "jira_comment_add", "jira_worklog_add"},
		},
		{
			toolHints{readOnly: false, destructive: true, idempotent: true, openWorld: true},
			"writes a remote field to a value the caller names",
			[]string{"jira_update", "jira_assign", "jira_worklog_update", "jira_worklog_delete"},
		},
		{
			toolHints{readOnly: false, destructive: true, idempotent: false, openWorld: true},
			"changes remote state depending on what it finds, or runs a command that can do anything",
			[]string{"conf_edit", "jira_transition", "command", "run", "lake_smoke"},
		},
	}

	want := make(map[string]toolHints)
	reason := make(map[string]string)
	for _, group := range groups {
		for _, name := range group.tools {
			if _, duplicate := want[name]; duplicate {
				t.Fatalf("tool %q is listed in two groups", name)
			}
			want[name] = group.hints
			reason[name] = group.why
		}
	}

	tools := New(allLayersConfig()).ListTools()
	check := func(got *bool, want bool, name, field string) {
		if got == nil {
			t.Errorf("%q: %s is nil; all four hints must be set explicitly", name, field)
			return
		}
		if *got != want {
			t.Errorf("%q: %s = %v, want %v (%s)", name, field, *got, want, reason[name])
		}
	}
	for name, expected := range want {
		entry, ok := tools[name]
		if !ok {
			t.Errorf("tool %q is expected here but is not registered", name)
			continue
		}
		a := entry.Tool.Annotations
		check(a.ReadOnlyHint, expected.readOnly, name, "ReadOnlyHint")
		check(a.DestructiveHint, expected.destructive, name, "DestructiveHint")
		check(a.IdempotentHint, expected.idempotent, name, "IdempotentHint")
		check(a.OpenWorldHint, expected.openWorld, name, "OpenWorldHint")
	}
	for name := range tools {
		if _, expected := want[name]; !expected {
			t.Errorf("tool %q is registered but not listed here: decide what it does to the world, give its registration a profile from annotations.go, and add it above", name)
		}
	}
}
