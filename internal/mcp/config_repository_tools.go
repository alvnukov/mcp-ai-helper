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
	"gopkg.in/yaml.v3"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

type configAllowRepositoryRequest struct {
	Integration string `json:"integration"`
	Reload      *bool  `json:"reload"`
}

func registerConfigAllowRepositoryTool(
	srv *server.MCPServer,
	deps *Server,
	reload configReloadFunc,
	repoPath string,
) {
	srv.AddTool(basemcp.NewTool("config_allow_repository",
		setsLocal,
		basemcp.WithDescription("Allow this helper's startup repository to use Jira, Confluence, or both. Appends only the server-known repository path to the global allowed_repositories list; arbitrary paths and repo-local config are not accepted. A process restart is required before newly allowed integration tools appear."),
		basemcp.WithString("integration",
			basemcp.Required(),
			basemcp.Description("Integration to allow for the current startup repository."),
			basemcp.Enum("jira", "confluence", "both"),
		),
		basemcp.WithBoolean("reload", basemcp.Description("Reload runtime config after writing. Defaults to true; tool visibility still requires a process restart.")),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args configAllowRepositoryRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		cfg, _, _, _, _ := deps.loadDeps()
		path := effectiveConfigPath("", cfg.SourcePath)
		loaded, added, repository, err := allowConfigRepository(path, args.Integration, repoPath)
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
		return structured(map[string]any{
			"status":           "ok",
			"integration":      strings.ToLower(strings.TrimSpace(args.Integration)),
			"repository":       repository,
			"added":            added,
			"reloaded":         reloadNow,
			"restart_required": true,
			"config_path":      path,
			"config":           loaded,
		})
	})
}

func allowConfigRepository(
	path string,
	integration string,
	repoPath string,
) (*config.Config, []string, string, error) {
	if config.IsRepoConfigPath(path) {
		return nil, nil, "", errors.New("repo config (.mcp-ai-helper.yaml) is user-editable only; repository access must be added to the global helper config")
	}
	selection := strings.ToLower(strings.TrimSpace(integration))
	switch selection {
	case "jira", "confluence", "both":
	default:
		return nil, nil, "", fmt.Errorf("integration must be jira, confluence, or both, got %q", integration)
	}
	repository, err := normalizeConfigRepositoryPath(repoPath)
	if err != nil {
		return nil, nil, "", err
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, nil, "", err
	}

	type target struct {
		name    string
		allowed *[]string
	}
	targets := make([]target, 0, 2)
	if selection == "jira" || selection == "both" {
		if cfg.Integrations.Jira == nil {
			return nil, nil, "", errors.New("Jira integration is not configured; add its credentials before allowing repositories")
		}
		targets = append(targets, target{name: "jira", allowed: &cfg.Integrations.Jira.AllowedRepositories})
	}
	if selection == "confluence" || selection == "both" {
		if cfg.Integrations.Confluence == nil {
			return nil, nil, "", errors.New("Confluence integration is not configured; add its credentials before allowing repositories")
		}
		targets = append(targets, target{name: "confluence", allowed: &cfg.Integrations.Confluence.AllowedRepositories})
	}

	added := make([]string, 0, len(targets))
	for _, item := range targets {
		if repositoryCovered(*item.allowed, repository) {
			continue
		}
		*item.allowed = append(*item.allowed, repository)
		added = append(added, item.name)
	}
	if len(added) == 0 {
		return cfg, added, repository, nil
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, nil, "", fmt.Errorf("marshal config: %w", err)
	}
	loaded, err := writeValidatedConfig(path, string(data))
	if err != nil {
		return nil, nil, "", err
	}
	return loaded, added, repository, nil
}

func normalizeConfigRepositoryPath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("startup repository context is unavailable; start the helper from a local project")
	}
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("startup repository path must be absolute: %q", path)
	}
	path = filepath.Clean(path)
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		path = resolved
	}
	return path, nil
}

func repositoryCovered(allowedRepositories []string, repository string) bool {
	for _, allowedRepository := range allowedRepositories {
		root, err := normalizeConfigRepositoryPath(allowedRepository)
		if err != nil {
			continue
		}
		relative, err := filepath.Rel(root, repository)
		if err != nil {
			continue
		}
		if relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(os.PathSeparator))) {
			return true
		}
	}
	return false
}
