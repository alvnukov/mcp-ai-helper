package mcp

import (
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// server_setup_guidance promises layers.<name>.enabled toggles logs, tasks,
// guidance, models, commands, and workflows — but registration gated the
// entire everyday surface behind layers.models alone, and layers.commands /
// workflows / guidance / tasks were never read. Each layer has to gate its
// own tools and nothing else's.
func TestLayerGatesMatchDocumentedSurface(t *testing.T) {
	cases := []struct {
		layer string
		set   func(cfg *config.Config, off *bool)
		tools []string
	}{
		{"guidance", func(c *config.Config, off *bool) { c.Layers.Guidance = config.LayerConfig{Enabled: off} }, []string{"assistant_guidance"}},
		{"models", func(c *config.Config, off *bool) { c.Layers.Models = config.LayerConfig{Enabled: off} }, []string{"list_models", "config_read"}},
		{"commands", func(c *config.Config, off *bool) { c.Layers.Commands = config.LayerConfig{Enabled: off} }, []string{"command"}},
		{"workflows", func(c *config.Config, off *bool) { c.Layers.Workflows = config.LayerConfig{Enabled: off} }, []string{"run"}},
		{"tasks", func(c *config.Config, off *bool) { c.Layers.Tasks = config.LayerConfig{Enabled: off} }, []string{"task"}},
	}
	for _, tc := range cases {
		t.Run(tc.layer+"=false removes its tools", func(t *testing.T) {
			off := false
			cfg := &config.Config{}
			tc.set(cfg, &off)
			srv := New(cfg)
			for _, tool := range tc.tools {
				if _, ok := srv.ListTools()[tool]; ok {
					t.Errorf("%s registered despite layers.%s.enabled=false", tool, tc.layer)
				}
			}
		})
	}

	t.Run("models=false keeps the everyday surface", func(t *testing.T) {
		off := false
		cfg := &config.Config{}
		cfg.Layers.Models = config.LayerConfig{Enabled: &off}
		srv := New(cfg)
		for _, tool := range []string{"command", "run", "task", "assistant_guidance", "file", "edit", "git"} {
			if _, ok := srv.ListTools()[tool]; !ok {
				t.Errorf("%s missing with layers.models=false; models only gates model and config tools", tool)
			}
		}
	})

	t.Run("default config keeps the everyday surface", func(t *testing.T) {
		srv := New(&config.Config{})
		for _, tool := range []string{"command", "run", "task", "assistant_guidance", "list_models", "config_read", "file", "edit", "git"} {
			if _, ok := srv.ListTools()[tool]; !ok {
				t.Errorf("%s missing from the default surface", tool)
			}
		}
	})
}
