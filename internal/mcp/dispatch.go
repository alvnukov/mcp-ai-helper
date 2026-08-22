package mcp

import (
	"context"
	"fmt"
	"sort"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// The six merged tools — file, edit, command, git, task, run — all take an
// `action` and fan out from it. This is that fan-out, written once.
//
// Sharing it buys two things the hand-written copies could not. The action list
// in the schema now comes from the same map the dispatcher reads, so a handler
// added without its enum entry — or an enum entry with no handler — is no longer
// possible. And a wrong action is answered with the actions that would have
// worked, which is what lets a model fix the call on its next attempt instead of
// guessing at a surface it cannot see.

// actionHandler serves one action of a merged tool.
type actionHandler func(context.Context, basemcp.CallToolRequest) (*basemcp.CallToolResult, error)

// actions is the set of actions one merged tool understands.
type actions map[string]actionHandler

// names lists the actions in a stable order, for the schema enum and for error
// messages.
func (a actions) names() []string {
	names := make([]string, 0, len(a))
	for name := range a {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// dispatch builds the handler for a merged tool from its action set. The
// repo-local tools.deny policy is enforced here, ahead of every action,
// because a denial that only some tools consult is not a policy.
func dispatch(deps *Server, tool string, handlers actions) server.ToolHandlerFunc {
	return func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		argsMap, _ := req.Params.Arguments.(map[string]any)
		action, _ := argsMap["action"].(string)
		if deps != nil {
			if repoPath, _ := argsMap["repo_path"].(string); strings.TrimSpace(repoPath) != "" {
				repoCfg, err := config.LoadRepoConfig(repoPath)
				if err != nil {
					return basemcp.NewToolResultError(fmt.Sprintf("load repo config: %v", err)), nil
				}
				if repoCfg.ToolDenied(tool) {
					return basemcp.NewToolResultError(fmt.Sprintf("tool %q is denied by repo-local config", tool)), nil
				}
				if _, known := handlers[action]; known {
					if err := deps.inspectGenericAccess(tool, action, argsMap, repoCfg); err != nil {
						return basemcp.NewToolResultError(err.Error()), nil
					}
				}
			}
		}
		if handler, ok := handlers[action]; ok {
			return handler(ctx, req)
		}
		return basemcp.NewToolResultError(unknownActionMessage(tool, action, handlers.names())), nil
	}
}

func unknownActionMessage(tool string, action string, known []string) string {
	if strings.TrimSpace(action) == "" {
		return tool + ": action is required; expected one of: " + strings.Join(known, ", ")
	}
	return tool + ": unknown action: " + action + "; expected one of: " + strings.Join(known, ", ")
}

// actionEnum renders the action set as the schema option for the tool's `action`
// argument, so the advertised list and the dispatched list are the same list.
func actionEnum(handlers actions) basemcp.PropertyOption {
	return basemcp.Enum(handlers.names()...)
}

// ignoringContext adapts a handler that has no use for the request context,
// which most of the read-only actions do not.
func ignoringContext(handler func(basemcp.CallToolRequest) (*basemcp.CallToolResult, error)) actionHandler {
	return func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return handler(req)
	}
}

// withDeps binds the server an action needs, so the action set holds handlers of
// one shape.
func withDeps(handler func(context.Context, basemcp.CallToolRequest, *Server) (*basemcp.CallToolResult, error), deps *Server) actionHandler {
	return func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		return handler(ctx, req, deps)
	}
}
