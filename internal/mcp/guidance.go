package mcp

import (
	"context"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/zol/mcp-ai-helper/internal/config"
)

const guidanceURI = "mcp-ai-helper://guidance"

func registerGuidance(srv *server.MCPServer, cfg *config.Config) {
	guidanceText := cfg.AssistantGuidance
	if guidanceText == "" {
		guidanceText = config.DefaultAssistantGuidance()
	}
	srv.AddTool(basemcp.NewTool("assistant_guidance",
		basemcp.WithDescription("Return mandatory operating guidance for using mcp-ai-helper efficiently and safely."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return structured(map[string]string{"guidance": guidanceText})
	})
	srv.AddTool(basemcp.NewTool("server_setup_guidance",
		basemcp.WithDescription("Return recommendations for configuring mcp-ai-helper and its repo-local config file."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return structured(config.SetupGuidance(cfg.SourcePath))
	})
	srv.AddResource(basemcp.Resource{
		URI:         guidanceURI,
		Name:        "mcp-ai-helper operating guidance",
		Description: "Mandatory guidance for workflow-first, token-efficient, evidence-linked helper usage.",
		MIMEType:    "text/plain",
	}, func(_ context.Context, _ basemcp.ReadResourceRequest) ([]basemcp.ResourceContents, error) {
		return []basemcp.ResourceContents{
			basemcp.TextResourceContents{URI: guidanceURI, MIMEType: "text/plain", Text: guidanceText},
		}, nil
	})
	srv.AddPrompt(basemcp.Prompt{
		Name:        "mcp-ai-helper-guidance",
		Description: "Instructions a calling LLM should follow before using mcp-ai-helper tools.",
	}, func(_ context.Context, _ basemcp.GetPromptRequest) (*basemcp.GetPromptResult, error) {
		return basemcp.NewGetPromptResult(
			"mcp-ai-helper operating guidance",
			[]basemcp.PromptMessage{
				basemcp.NewPromptMessage(basemcp.RoleUser, basemcp.NewTextContent(guidanceText)),
			},
		), nil
	})
}
