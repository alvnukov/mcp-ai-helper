package mcp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/config"
)

type configReloadFunc func(path string) (*config.Config, error)

type configPathRequest struct {
	Path string `json:"path"`
}

type configReplaceRequest struct {
	Path       string `json:"path"`
	ConfigYAML string `json:"config_yaml"`
	Reload     *bool  `json:"reload"`
}

func registerConfigTools(srv *server.MCPServer, deps *Server, reload configReloadFunc) {
	srv.AddTool(basemcp.NewTool("config_schema",
		basemcp.WithDescription("Return machine-readable documentation for every mcp-ai-helper config field and the safe model-driven setup workflow."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return structured(config.Schema())
	})

	srv.AddTool(basemcp.NewTool("config_read",
		basemcp.WithDescription("Return the active sanitized config, or validate/read another config path without exposing literal api_key values."),
		basemcp.WithString("path", basemcp.Description("Optional config path. Empty means active in-memory config.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args configPathRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		if strings.TrimSpace(args.Path) == "" {
			cfg, _, _, _, _ := deps.loadDeps()
			return structured(map[string]any{"config": cfg, "config_path": cfg.SourcePath, "source": "memory"})
		}
		loaded, err := config.Load(args.Path)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"config": loaded, "config_path": loaded.SourcePath, "source": "file"})
	})

	srv.AddTool(basemcp.NewTool("config_replace",
		basemcp.WithDescription("Validate and atomically replace the complete YAML config. Reloads the running helper by default, without restarting Codex."),
		basemcp.WithString("config_yaml", basemcp.Required(), basemcp.Description("Complete YAML config document.")),
		basemcp.WithString("path", basemcp.Description("Optional config path. Empty means the active config path.")),
		basemcp.WithBoolean("reload", basemcp.Description("Reload runtime after writing. Defaults to true.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args configReplaceRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		cfg, _, _, _, _ := deps.loadDeps()
		path := effectiveConfigPath(args.Path, cfg.SourcePath)
		loaded, err := writeValidatedConfig(path, args.ConfigYAML)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		reloadNow := args.Reload == nil || *args.Reload
		if reloadNow {
			loaded, err = reload(path)
			if err != nil {
				return basemcp.NewToolResultError(err.Error()), nil
			}
		}
		return structured(map[string]any{"status": "ok", "reloaded": reloadNow, "config_path": path, "config": loaded})
	})

	srv.AddTool(basemcp.NewTool("config_reload",
		basemcp.WithDescription("Reload the running helper from config YAML without restarting Codex. Tool visibility still changes only on process restart."),
		basemcp.WithString("path", basemcp.Description("Optional config path. Empty means the active config path.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args configPathRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		cfg, _, _, _, _ := deps.loadDeps()
		path := effectiveConfigPath(args.Path, cfg.SourcePath)
		loaded, err := reload(path)
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(map[string]any{"status": "ok", "config_path": path, "config": loaded})
	})
}

func effectiveConfigPath(requested string, active string) string {
	if strings.TrimSpace(requested) != "" {
		return requested
	}
	if strings.TrimSpace(active) != "" {
		return active
	}
	return config.DefaultConfigPath()
}

func writeValidatedConfig(path string, yamlText string) (*config.Config, error) {
	if strings.TrimSpace(yamlText) == "" {
		return nil, errors.New("config_yaml is required")
	}
	if strings.TrimSpace(path) == "" {
		path = config.DefaultConfigPath()
	}
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create config directory: %w", err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("open config directory root: %w", err)
	}
	defer func() { _ = root.Close() }()
	tmp, err := os.CreateTemp(dir, ".config-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("create temp config: %w", err)
	}
	tmpPath := tmp.Name()
	tmpBase := filepath.Base(tmpPath)
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("chmod temp config: %w", err)
	}
	if _, err := tmp.WriteString(yamlText); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write temp config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close temp config: %w", err)
	}
	loaded, err := config.Load(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		src, readErr := root.ReadFile(tmpBase)
		if readErr != nil {
			return nil, fmt.Errorf("read temp config for copy: %w", readErr)
		}
		if writeErr := root.WriteFile(base, src, 0o600); writeErr != nil {
			return nil, fmt.Errorf("write config via copy: %w", writeErr)
		}
	}
	loaded.SourcePath = path
	return loaded, nil
}
