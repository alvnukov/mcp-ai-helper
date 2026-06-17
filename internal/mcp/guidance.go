package mcp

import (
	"context"
	"sort"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
	"github.com/zol/mcp-ai-helper/internal/config"
)

const guidanceURI = "mcp-ai-helper://guidance"

const toolDiscoveryGuidance = `## Tool Discovery Hints

1. Retained command output: use command_get(command_id, mode=status|result|tail|evidence) or filter_command_history(command_id) instead of rerunning commands or reading raw log files.
2. Feedback intake: use issue_add to record cross-repository feedback, issue_list to inspect open feedback issues, and issue_accept to move one issue into in_progress when taking ownership.
3. If these tool names are not visible after assistant_guidance, call tool_manifest to compare helper-registered tools with the client-visible surface, then request MCP client rediscovery/restart; do not replace them with shell/file/git fallbacks.`

func currentGuidance(cfg *config.Config) string {
	return withToolDiscoveryGuidance(config.GuidanceForConfig(cfg))
}

func withToolDiscoveryGuidance(guidance string) string {
	if strings.Contains(guidance, "tool_manifest") && strings.Contains(guidance, "command_get") && strings.Contains(guidance, "filter_command_history") && strings.Contains(guidance, "issue_add") {
		return guidance
	}
	guidance = strings.TrimSpace(guidance)
	if guidance == "" {
		return toolDiscoveryGuidance
	}
	return guidance + "\n\n" + toolDiscoveryGuidance
}

func toolManifest(srv *server.MCPServer) map[string]any {
	names := make([]string, 0, len(srv.ListTools()))
	for name := range srv.ListTools() {
		names = append(names, name)
	}
	sort.Strings(names)
	return map[string]any{"count": len(names), "tools": names}
}

func registerGuidance(srv *server.MCPServer, deps *Server) {
	guidanceText := func() string {
		cfg, _, _, _, _ := deps.loadDeps()
		return currentGuidance(cfg)
	}
	srv.AddTool(basemcp.NewTool("assistant_guidance",
		basemcp.WithDescription("Return mandatory operating guidance for using mcp-ai-helper efficiently and safely."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return structured(map[string]string{"guidance": guidanceText()})
	})
	srv.AddTool(basemcp.NewTool("server_setup_guidance",
		basemcp.WithDescription("Return recommendations for configuring mcp-ai-helper and its repo-local config file."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		cfg, _, _, _, _ := deps.loadDeps()
		return structured(config.SetupGuidanceForConfig(cfg))
	})
	srv.AddTool(basemcp.NewTool("tool_manifest",
		basemcp.WithDescription("Return a compact sorted list of helper-registered MCP tools for surface mismatch diagnostics."),
	), func(_ context.Context, _ basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return structured(toolManifest(srv))
	})
	srv.AddResource(basemcp.Resource{
		URI:         guidanceURI,
		Name:        "mcp-ai-helper operating guidance",
		Description: "Mandatory guidance for workflow-first, token-efficient, evidence-linked helper usage.",
		MIMEType:    "text/plain",
	}, func(_ context.Context, _ basemcp.ReadResourceRequest) ([]basemcp.ResourceContents, error) {
		return []basemcp.ResourceContents{
			basemcp.TextResourceContents{URI: guidanceURI, MIMEType: "text/plain", Text: guidanceText()},
		}, nil
	})
	srv.AddPrompt(basemcp.Prompt{
		Name:        "mcp-ai-helper-guidance",
		Description: "Instructions a calling LLM should follow before using mcp-ai-helper tools.",
	}, func(_ context.Context, _ basemcp.GetPromptRequest) (*basemcp.GetPromptResult, error) {
		return basemcp.NewGetPromptResult(
			"mcp-ai-helper operating guidance",
			[]basemcp.PromptMessage{
				basemcp.NewPromptMessage(basemcp.RoleUser, basemcp.NewTextContent(guidanceText())),
			},
		), nil
	})
}
