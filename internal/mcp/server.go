package mcp

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/confluence"
	"github.com/zol/mcp-ai-helper/internal/jira"
	"github.com/zol/mcp-ai-helper/internal/pipeline"
	"github.com/zol/mcp-ai-helper/internal/project"
	"github.com/zol/mcp-ai-helper/internal/provider"
	"github.com/zol/mcp-ai-helper/internal/security"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

// Server holds mutable server state protected by a read-write mutex.
type Server struct {
	mu                  sync.RWMutex
	cfg                 *config.Config
	chat                provider.ChatClient
	commands            *command.Runner
	pipelines           *pipeline.Runner
	taskStore           *tasks.Store
	taskBackend         taskBackend
	secretMask          *security.Mask
	jiraClient          *jira.Client
	confluenceClient    *confluence.Client
	confluenceClientErr error
	taskUI              *taskUIServer
}

func buildJiraClient(cfg *config.Config) *jira.Client {
	if cfg.Integrations.Jira == nil || !cfg.Integrations.Jira.IsEnabled() {
		return nil
	}
	jc, err := jira.NewClient(*cfg.Integrations.Jira)
	if err != nil {
		return nil
	}
	return jc
}

func buildConfluenceClient(cfg *config.Config) (*confluence.Client, error) {
	if cfg.Integrations.Confluence == nil || !cfg.Integrations.Confluence.IsEnabled() {
		return nil, nil
	}
	cc, err := confluence.NewClient(confluence.Config{
		URL:       cfg.Integrations.Confluence.URL,
		Username:  cfg.Integrations.Confluence.Username,
		APIKey:    cfg.Integrations.Confluence.APIKey,
		APIKeyEnv: cfg.Integrations.Confluence.APIKeyEnv,
	})
	if err != nil {
		return nil, err
	}
	return cc, nil
}

func (s *Server) getConfluenceClient() (*confluence.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.confluenceClient == nil {
		if s.confluenceClientErr != nil {
			return nil, s.confluenceClientErr
		}
		return nil, fmt.Errorf("confluence: not configured")
	}
	return s.confluenceClient, nil
}

func (s *Server) getJiraClient() (*jira.Client, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.jiraClient == nil {
		return nil, fmt.Errorf("jira: not configured or connection failed")
	}
	return s.jiraClient, nil
}

func (s *Server) loadDeps() (*config.Config, provider.ChatClient, *command.Runner, *pipeline.Runner, *tasks.Store) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg, s.chat, s.commands, s.pipelines, s.taskStore
}

func (s *Server) loadTaskBackend() taskBackend {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.taskBackend
}

func (s *Server) loadTaskBackendForRepo(repoPath string) (taskBackend, error) {
	cfg, _, cmds, _, store := s.loadDeps()
	repoCfg, err := config.LoadRepoConfig(repoPath)
	if err != nil {
		return nil, err
	}
	if repoCfg == nil || repoCfg.TaskRegistry == nil {
		return s.loadTaskBackend(), nil
	}
	merged, err := config.MergeRepoConfig(cfg, repoCfg, repoPath)
	if err != nil {
		return nil, err
	}
	return buildTaskBackend(merged, cmds, store), nil
}

func buildTaskBackend(cfg *config.Config, cmds *command.Runner, store *tasks.Store) taskBackend {
	switch cfg.TaskRegistry.Backend {
	case "obsidian":
		return newObsidianTaskBackend(cfg.TaskRegistry.Obsidian.Path)
	default:
		return newLakeTaskBackend(cmds, store)
	}
}

func buildDeps(cfg *config.Config) (provider.ChatClient, *command.Runner, *pipeline.Runner, *tasks.Store, taskBackend) {
	chat := provider.NewClient(cfg.Providers)
	commandPolicy := cfg.CommandPolicy
	commandPolicy.ProtectedConfigPath = cfg.SourcePath
	cmds := command.NewRunnerWithMask(commandPolicy, cfg.SecretMask())
	projectStore, err := project.NewStore(cfg.CommandPolicy.LogDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-ai-helper: project store from %q: %v; falling back to .mcp-ai-helper\n", cfg.CommandPolicy.LogDir, err)
		projectStore, _ = project.NewStore(".mcp-ai-helper")
	}
	store := tasks.NewStore(projectStore)
	backend := buildTaskBackend(cfg, cmds, store)
	pipes := pipeline.NewRunnerWithTaskBackend(cfg, chat, workflowTaskBackend{backend: backend})
	return chat, cmds, pipes, store, backend
}

func (s *Server) commandRunnerForRepo(repoPath string, toolName string) (*command.Runner, error) {
	cfg, _, cmds, _, _ := s.loadDeps()
	repoCfg, err := config.LoadRepoConfig(repoPath)
	if err != nil {
		return nil, err
	}
	if repoCfg == nil {
		return cmds, nil
	}
	if repoCfg.ToolDenied(toolName) {
		return nil, fmt.Errorf("tool %q is denied by repo-local config", toolName)
	}
	merged, err := config.MergeRepoConfig(cfg, repoCfg, repoPath)
	if err != nil {
		return nil, err
	}
	return command.NewRunnerWithMask(merged.CommandPolicy, merged.SecretMask()), nil
}

func (s *Server) pipelineRunnerForRepo(repoPath string, toolName string) (*pipeline.Runner, error) {
	cfg, chat, _, pipes, store := s.loadDeps()
	repoCfg, err := config.LoadRepoConfig(repoPath)
	if err != nil {
		return nil, err
	}
	if repoCfg == nil {
		return pipes, nil
	}
	if repoCfg.ToolDenied(toolName) {
		return nil, fmt.Errorf("tool %q is denied by repo-local config", toolName)
	}
	merged, err := config.MergeRepoConfig(cfg, repoCfg, repoPath)
	if err != nil {
		return nil, err
	}
	cmds := command.NewRunnerWithMask(merged.CommandPolicy, merged.SecretMask())
	backend := buildTaskBackend(merged, cmds, store)
	return pipeline.NewRunnerWithTaskBackend(merged, chat, workflowTaskBackend{backend: backend}), nil
}

// New constructs an MCP server with all configured helper tools.
func New(cfg *config.Config) *server.MCPServer {
	srv := server.NewMCPServer(
		"mcp-ai-helper",
		"0.1.0",
		server.WithToolCapabilities(false),
		server.WithResourceCapabilities(false, false),
		server.WithPromptCapabilities(false),
	)

	deps := &Server{cfg: cfg, secretMask: buildSecretMask(cfg), jiraClient: buildJiraClient(cfg)}
	deps.chat, deps.commands, deps.pipelines, deps.taskStore, deps.taskBackend = buildDeps(cfg)

	registerLanguageTools(srv)
	registerFileTools(srv)
	registerGitTools(srv)

	if cfg.LayerEnabled("models") {
		registerGuidance(srv, deps)

		reloadConfig := func(path string) (*config.Config, error) {
			if strings.TrimSpace(path) == "" {
				deps.mu.RLock()
				path = deps.cfg.SourcePath
				deps.mu.RUnlock()
			}
			next, err := config.Load(path)
			if err != nil {
				return nil, err
			}
			chat, cmds, pipes, store, backend := buildDeps(next)
			deps.mu.Lock()
			deps.cfg = next
			deps.chat = chat
			deps.commands = cmds
			deps.pipelines = pipes
			deps.taskStore = store
			deps.taskBackend = backend
			deps.secretMask = buildSecretMask(next)
			deps.jiraClient = buildJiraClient(next)
			deps.confluenceClient, deps.confluenceClientErr = buildConfluenceClient(next)
			deps.mu.Unlock()
			return next, nil
		}
		registerConfigTools(srv, deps, reloadConfig)
		registerFeatureTools(srv, deps)
		registerModelTools(srv, deps)
		registerCommandTools(srv, deps)
		registerLakeTools(srv, deps)
		registerPipelineTools(srv, deps)

		if cfg.LayerEnabled("issues") {
			registerIssueTools(srv, deps)
		}
		registerTaskTools(srv, deps)
		registerTaskUITools(srv, deps)
		registerPlanningTools(srv, deps)
	}

	if cfg.Integrations.Jira != nil && cfg.Integrations.Jira.IsEnabled() {
		registerJiraTools(srv, deps)
	}
	if cfg.Integrations.Confluence != nil && cfg.Integrations.Confluence.IsEnabled() {
		registerConfluenceTools(srv, deps)
	}

	return srv
}

// sanitize masks known secrets in a string before it reaches the LLM.
func (s *Server) sanitize(msg string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.secretMask != nil {
		return s.secretMask.Apply(msg)
	}
	return msg
}

func buildSecretMask(cfg *config.Config) *security.Mask {
	return cfg.SecretMask()
}
