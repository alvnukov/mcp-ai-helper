package mcp

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/command"
	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/pipeline"
	"github.com/zol/mcp-ai-helper/internal/project"
	"github.com/zol/mcp-ai-helper/internal/provider"
	"github.com/zol/mcp-ai-helper/internal/tasks"
)

// Server holds mutable server state protected by a read-write mutex.
type Server struct {
	mu        sync.RWMutex
	cfg       *config.Config
	chat      provider.ChatClient
	commands  *command.Runner
	pipelines *pipeline.Runner
	taskStore *tasks.Store
}

func (s *Server) loadDeps() (*config.Config, provider.ChatClient, *command.Runner, *pipeline.Runner, *tasks.Store) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg, s.chat, s.commands, s.pipelines, s.taskStore
}

func buildDeps(cfg *config.Config) (provider.ChatClient, *command.Runner, *pipeline.Runner, *tasks.Store) {
	chat := provider.NewClient(cfg.Providers)
	cmds := command.NewRunner(cfg.CommandPolicy)
	projectStore, err := project.NewStore(cfg.CommandPolicy.LogDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mcp-ai-helper: project store from %q: %v; falling back to .mcp-ai-helper\n", cfg.CommandPolicy.LogDir, err)
		projectStore, _ = project.NewStore(".mcp-ai-helper")
	}
	store := tasks.NewStore(projectStore)
	pipes := pipeline.NewRunnerWithTaskBackend(cfg, chat, leanTaskBackend{commands: cmds, store: store})
	return chat, cmds, pipes, store
}

type leanTaskBackend struct {
	commands *command.Runner
	store    *tasks.Store
}

func (b leanTaskBackend) Get(ctx context.Context, repoPath string, id string) (tasks.Task, error) {
	task, _, err := readTask(ctx, repoPath, id, b.commands, b.store)
	return task, err
}

func (b leanTaskBackend) List(ctx context.Context, repoPath string) ([]tasks.Task, error) {
	items, _, err := readCurrentTasks(ctx, repoPath, b.commands, b.store)
	return items, err
}

func (b leanTaskBackend) SetStatus(ctx context.Context, req tasks.StatusRequest) (tasks.Task, error) {
	result, err := setTaskStatus(ctx, req, b.commands, b.store)
	return result.Task, err
}

func (b leanTaskBackend) BatchUpsert(ctx context.Context, req tasks.BatchUpsertRequest) (tasks.BatchUpsertResult, error) {
	result, err := batchUpsertTasks(ctx, req, b.commands, b.store)
	return tasks.BatchUpsertResult{Upserted: result.Upserted, Closed: result.Closed}, err
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

	deps := &Server{cfg: cfg}
	deps.chat, deps.commands, deps.pipelines, deps.taskStore = buildDeps(cfg)

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
			chat, cmds, pipes, store := buildDeps(next)
			deps.mu.Lock()
			deps.cfg = next
			deps.chat = chat
			deps.commands = cmds
			deps.pipelines = pipes
			deps.taskStore = store
			deps.mu.Unlock()
			return next, nil
		}
		registerConfigTools(srv, deps, reloadConfig)
		registerModelTools(srv, deps)
		registerCommandTools(srv, deps)
		registerLakeTools(srv, deps)
		registerPipelineTools(srv, deps)

		if cfg.LayerEnabled("issues") {
			registerIssueTools(srv, deps)
		}
		registerTaskTools(srv, deps)
		registerPlanningTools(srv, deps)
	}

	return srv
}
