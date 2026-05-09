package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/provider"
)

func registerModelTools(srv *server.MCPServer, deps *Server) {
	srv.AddTool(basemcp.NewTool("health",
		basemcp.WithDescription("Return server health status."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return structured(map[string]string{"status": "ok"})
	})

	srv.AddTool(basemcp.NewTool("list_models",
		basemcp.WithDescription("List configured model profiles and routing policy."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		cfg, _, _, _, _ := deps.loadDeps()
		return structured(map[string]any{"models": cfg.Models, "routing": cfg.Routing})
	})

	srv.AddTool(basemcp.NewTool("query_model",
		basemcp.WithDescription("Send a bounded prompt to a configured OpenAI-compatible model."),
		basemcp.WithString("model_id", basemcp.Description("Configured model id.")),
		basemcp.WithString("prompt", basemcp.Description("User prompt.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args struct {
			ModelID string `json:"model_id"`
			Prompt  string `json:"prompt"`
		}
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		cfg, chat, _, _, _ := deps.loadDeps()
		model, ok := cfg.Models[args.ModelID]
		if !ok {
			return basemcp.NewToolResultError("unknown model_id"), nil
		}
		resp, err := chat.Complete(ctx, provider.ChatRequest{
			ProviderID:      model.Provider,
			ModelID:         args.ModelID,
			Model:           model.Model,
			SystemPrompt:    model.Prompt(),
			UserPrompt:      args.Prompt,
			MaxOutputTokens: model.MaxOutputTokens,
			Temperature:     model.Temperature,
		})
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(resp)
	})
}
