package mcp

import (
	"context"
	"fmt"
	"time"

	gojira "github.com/andygrunwald/go-jira"
	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/jira"
)

// --- Request types ---

type jiraSearchRequest struct {
	JQL        string `json:"jql"`
	MaxResults int    `json:"max_results"`
}

type jiraReadRequest struct {
	IssueKey string `json:"issue_key"`
}

type jiraUpdateRequest struct {
	IssueKey     string                 `json:"issue_key"`
	Summary      *string                `json:"summary"`
	Description  *string                `json:"description"`
	Priority     *string                `json:"priority"`
	Labels       []string               `json:"labels"`
	Components   []string               `json:"components"`
	FixVersions  []string               `json:"fix_versions"`
	CustomFields map[string]interface{} `json:"custom_fields"`
}

type jiraTransitionRequest struct {
	IssueKey       string `json:"issue_key"`
	TransitionName string `json:"transition_name"`
}

type jiraAssignRequest struct {
	IssueKey string `json:"issue_key"`
	Username string `json:"username"`
	Unassign bool   `json:"unassign"`
}

type jiraWorklogListRequest struct {
	IssueKey string `json:"issue_key"`
	Since    string `json:"since"`
	Until    string `json:"until"`
	Username string `json:"username"`
}

type jiraWorklogReportRequest struct {
	Username string `json:"username"`
	Since    string `json:"since"`
	Until    string `json:"until"`
}

type jiraWorklogAddRequest struct {
	IssueKey  string `json:"issue_key"`
	TimeSpent string `json:"time_spent"`
	Comment   string `json:"comment"`
	Started   string `json:"started"`
}

type jiraWorklogUpdateRequest struct {
	IssueKey  string  `json:"issue_key"`
	WorklogID string  `json:"worklog_id"`
	TimeSpent *string `json:"time_spent"`
	Comment   *string `json:"comment"`
}

type jiraWorklogDeleteRequest struct {
	IssueKey  string `json:"issue_key"`
	WorklogID string `json:"worklog_id"`
}

// --- Registration ---

func checkJiraMutate(deps *Server, issueKey string) bool {
	cfg, _, _, _, _ := deps.loadDeps()
	if cfg.Integrations.Jira == nil {
		return false
	}
	if !cfg.Integrations.Jira.CanMutate() {
		return false
	}
	if !cfg.Integrations.Jira.IsProjectAllowed(issueKey) {
		return false
	}
	return true
}

func safeError(deps *Server, err error) *basemcp.CallToolResult {
	return basemcp.NewToolResultError(deps.sanitize(err.Error()))
}

func registerJiraTools(srv *server.MCPServer, deps *Server) {
	getClient := func() (*jira.Client, error) {
		return deps.getJiraClient()
	}

	// --- Issue tools ---

	srv.AddTool(basemcp.NewTool("jira_search",
		basemcp.WithDescription("Search Jira issues by JQL query."),
		basemcp.WithString("jql", basemcp.Required(), basemcp.Description("JQL query string.")),
		basemcp.WithNumber("max_results", basemcp.Description("Maximum results. Defaults to 20.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args jiraSearchRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		if args.MaxResults <= 0 {
			args.MaxResults = 20
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		issues, err := jc.SearchIssues(args.JQL, args.MaxResults)
		if err != nil {
			return safeError(deps, err), nil
		}
		type summary struct {
			Key      string `json:"key"`
			Summary  string `json:"summary"`
			Status   string `json:"status"`
			Priority string `json:"priority"`
			Assignee string `json:"assignee"`
		}
		items := make([]summary, 0, len(issues))
		for _, iss := range issues {
			s := summary{Key: iss.Key}
			if iss.Fields != nil {
				s.Summary = iss.Fields.Summary
				if iss.Fields.Status != nil {
					s.Status = iss.Fields.Status.Name
				}
				if iss.Fields.Priority != nil {
					s.Priority = iss.Fields.Priority.Name
				}
				if iss.Fields.Assignee != nil {
					s.Assignee = iss.Fields.Assignee.DisplayName
				}
			}
			items = append(items, s)
		}
		return structured(map[string]any{"total": len(items), "issues": items})
	})

	srv.AddTool(basemcp.NewTool("jira_read",
		basemcp.WithDescription("Read a single Jira issue by key, including available transitions."),
		basemcp.WithString("issue_key", basemcp.Required(), basemcp.Description("Jira issue key, e.g. PROJ-123.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args jiraReadRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		issue, err := jc.GetIssue(args.IssueKey)
		if err != nil {
			return safeError(deps, err), nil
		}
		transitions, _ := jc.GetTransitions(args.IssueKey)
		transitionNames := make([]string, 0, len(transitions))
		for _, t := range transitions {
			transitionNames = append(transitionNames, t.Name)
		}
		return structured(map[string]any{
			"issue":                 issue,
			"available_transitions": transitionNames,
		})
	})

	srv.AddTool(basemcp.NewTool("jira_update",
		basemcp.WithDescription("Update Jira issue fields. Only provided fields are changed."),
		basemcp.WithString("issue_key", basemcp.Required(), basemcp.Description("Jira issue key.")),
		basemcp.WithString("summary", basemcp.Description("New summary.")),
		basemcp.WithString("description", basemcp.Description("New description.")),
		basemcp.WithString("priority", basemcp.Description("Priority name, e.g. High, Medium, Low.")),
		basemcp.WithArray("labels", basemcp.Description("Replacement labels array.")),
		basemcp.WithArray("components", basemcp.Description("Component names.")),
		basemcp.WithArray("fix_versions", basemcp.Description("Fix version names.")),
		basemcp.WithObject("custom_fields", basemcp.Description("Custom field ids mapped to values.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args jiraUpdateRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		if !checkJiraMutate(deps, args.IssueKey) {
			return safeError(deps, fmt.Errorf("jira: mutation not allowed")), nil
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		fields := make(map[string]interface{})
		if args.Summary != nil {
			fields["summary"] = *args.Summary
		}
		if args.Description != nil {
			fields["description"] = *args.Description
		}
		if args.Priority != nil {
			fields["priority"] = map[string]string{"name": *args.Priority}
		}
		if args.Labels != nil {
			fields["labels"] = args.Labels
		}
		if args.Components != nil {
			comps := make([]map[string]string, len(args.Components))
			for i, c := range args.Components {
				comps[i] = map[string]string{"name": c}
			}
			fields["components"] = comps
		}
		if args.FixVersions != nil {
			vers := make([]map[string]string, len(args.FixVersions))
			for i, v := range args.FixVersions {
				vers[i] = map[string]string{"name": v}
			}
			fields["fixVersions"] = vers
		}
		if args.CustomFields != nil {
			for k, v := range args.CustomFields {
				fields[k] = v
			}
		}
		if len(fields) == 0 {
			return safeError(deps, fmt.Errorf("no fields to update")), nil
		}
		if err := jc.UpdateIssue(args.IssueKey, fields); err != nil {
			return safeError(deps, err), nil
		}
		return structured(map[string]any{"status": "ok", "updated_fields": fieldKeys(fields)})
	})

	srv.AddTool(basemcp.NewTool("jira_transition",
		basemcp.WithDescription("Transition a Jira issue to a new status by transition name."),
		basemcp.WithString("issue_key", basemcp.Required(), basemcp.Description("Jira issue key.")),
		basemcp.WithString("transition_name", basemcp.Required(), basemcp.Description("Target transition name, e.g. 'Done', 'In Progress'.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args jiraTransitionRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		if !checkJiraMutate(deps, args.IssueKey) {
			return safeError(deps, fmt.Errorf("jira: mutation not allowed")), nil
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		if err := jc.DoTransition(args.IssueKey, args.TransitionName); err != nil {
			return safeError(deps, err), nil
		}
		return structured(map[string]any{"status": "ok", "issue_key": args.IssueKey, "transition": args.TransitionName})
	})

	srv.AddTool(basemcp.NewTool("jira_assign",
		basemcp.WithDescription("Assign or unassign a Jira issue."),
		basemcp.WithString("issue_key", basemcp.Required(), basemcp.Description("Jira issue key.")),
		basemcp.WithString("username", basemcp.Description("Username to assign. Omit when unassigning.")),
		basemcp.WithBoolean("unassign", basemcp.Description("Set to true to remove assignee.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args jiraAssignRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		if !checkJiraMutate(deps, args.IssueKey) {
			return safeError(deps, fmt.Errorf("jira: mutation not allowed")), nil
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		if args.Unassign {
			if err := jc.UnassignIssue(args.IssueKey); err != nil {
				return safeError(deps, err), nil
			}
			return structured(map[string]any{"status": "ok", "issue_key": args.IssueKey, "assigned": false})
		}
		if err := jc.AssignIssue(args.IssueKey, args.Username); err != nil {
			return safeError(deps, err), nil
		}
		return structured(map[string]any{"status": "ok", "issue_key": args.IssueKey, "assigned": args.Username})
	})

	// --- Worklog tools ---

	srv.AddTool(basemcp.NewTool("jira_worklog_list",
		basemcp.WithDescription("List worklog entries for an issue, optionally filtered by date range and user."),
		basemcp.WithString("issue_key", basemcp.Required(), basemcp.Description("Jira issue key.")),
		basemcp.WithString("since", basemcp.Description("Start date in YYYY-MM-DD or RFC3339 format.")),
		basemcp.WithString("until", basemcp.Description("End date in YYYY-MM-DD or RFC3339 format.")),
		basemcp.WithString("username", basemcp.Description("Filter by author username.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args jiraWorklogListRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		since, until, err := parseDateRange(args.Since, args.Until)
		if err != nil {
			return safeError(deps, err), nil
		}
		records, err := jc.GetWorklogs(args.IssueKey, since, until)
		if err != nil {
			return safeError(deps, err), nil
		}
		if args.Username != "" {
			var filtered []gojira.WorklogRecord
			for _, r := range records {
				if r.Author != nil && r.Author.Name == args.Username {
					filtered = append(filtered, r)
				}
			}
			records = filtered
		}
		return structured(map[string]any{"issue_key": args.IssueKey, "worklogs": records, "total": len(records)})
	})

	srv.AddTool(basemcp.NewTool("jira_worklog_report",
		basemcp.WithDescription("Aggregated worklog report for a user over a date range, grouped by issue."),
		basemcp.WithString("username", basemcp.Required(), basemcp.Description("Jira username.")),
		basemcp.WithString("since", basemcp.Required(), basemcp.Description("Start date in YYYY-MM-DD format.")),
		basemcp.WithString("until", basemcp.Required(), basemcp.Description("End date in YYYY-MM-DD format.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args jiraWorklogReportRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		since, until, err := parseDateRange(args.Since, args.Until)
		if err != nil {
			return safeError(deps, err), nil
		}
		entries, err := jc.GetWorklogsByUser(args.Username, since, until)
		if err != nil {
			return safeError(deps, err), nil
		}
		type issueSummary struct {
			Key     string                 `json:"key"`
			Hours   float64                `json:"hours"`
			Entries []gojira.WorklogRecord `json:"entries"`
		}
		byIssue := make(map[string]*issueSummary)
		var totalHours float64
		for _, e := range entries {
			s, ok := byIssue[e.IssueKey]
			if !ok {
				s = &issueSummary{Key: e.IssueKey}
				byIssue[e.IssueKey] = s
			}
			s.Entries = append(s.Entries, e.Record)
			s.Hours += float64(e.Record.TimeSpentSeconds) / 3600.0
			totalHours += float64(e.Record.TimeSpentSeconds) / 3600.0
		}
		issues := make([]issueSummary, 0, len(byIssue))
		for _, s := range byIssue {
			issues = append(issues, *s)
		}
		return structured(map[string]any{
			"username":    args.Username,
			"since":       args.Since,
			"until":       args.Until,
			"total_hours": totalHours,
			"by_issue":    issues,
		})
	})

	srv.AddTool(basemcp.NewTool("jira_worklog_add",
		basemcp.WithDescription("Add a worklog entry to a Jira issue."),
		basemcp.WithString("issue_key", basemcp.Required(), basemcp.Description("Jira issue key.")),
		basemcp.WithString("time_spent", basemcp.Required(), basemcp.Description("Time spent in Jira format, e.g. '1h 30m', '2d', '4h'.")),
		basemcp.WithString("comment", basemcp.Required(), basemcp.Description("Worklog comment.")),
		basemcp.WithString("started", basemcp.Description("Start time in RFC3339 format. Defaults to now.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args jiraWorklogAddRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		if !checkJiraMutate(deps, args.IssueKey) {
			return safeError(deps, fmt.Errorf("jira: mutation not allowed")), nil
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		var started *time.Time
		if args.Started != "" {
			t, err := time.Parse(time.RFC3339, args.Started)
			if err != nil {
				return safeError(deps, fmt.Errorf("invalid started time: %w", err)), nil
			}
			started = &t
		}
		record, err := jc.AddWorklog(args.IssueKey, args.TimeSpent, args.Comment, started)
		if err != nil {
			return safeError(deps, err), nil
		}
		return structured(map[string]any{"status": "ok", "worklog": record})
	})

	srv.AddTool(basemcp.NewTool("jira_worklog_update",
		basemcp.WithDescription("Update a worklog entry's time spent or comment."),
		basemcp.WithString("issue_key", basemcp.Required(), basemcp.Description("Jira issue key.")),
		basemcp.WithString("worklog_id", basemcp.Required(), basemcp.Description("Worklog entry ID.")),
		basemcp.WithString("time_spent", basemcp.Description("New time spent in Jira format.")),
		basemcp.WithString("comment", basemcp.Description("New comment text.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args jiraWorklogUpdateRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		if !checkJiraMutate(deps, args.IssueKey) {
			return safeError(deps, fmt.Errorf("jira: mutation not allowed")), nil
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		if err := jc.UpdateWorklog(args.IssueKey, args.WorklogID, args.TimeSpent, args.Comment); err != nil {
			return safeError(deps, err), nil
		}
		return structured(map[string]any{"status": "ok", "issue_key": args.IssueKey, "worklog_id": args.WorklogID})
	})

	srv.AddTool(basemcp.NewTool("jira_worklog_delete",
		basemcp.WithDescription("Delete a worklog entry from a Jira issue."),
		basemcp.WithString("issue_key", basemcp.Required(), basemcp.Description("Jira issue key.")),
		basemcp.WithString("worklog_id", basemcp.Required(), basemcp.Description("Worklog entry ID to delete.")),
	), func(ctx context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args jiraWorklogDeleteRequest
		if err := bind(req, &args); err != nil {
			return nil, err
		}
		if !checkJiraMutate(deps, args.IssueKey) {
			return safeError(deps, fmt.Errorf("jira: mutation not allowed")), nil
		}
		jc, err := getClient()
		if err != nil {
			return safeError(deps, err), nil
		}
		if err := jc.DeleteWorklog(args.IssueKey, args.WorklogID); err != nil {
			return safeError(deps, err), nil
		}
		return structured(map[string]any{"status": "ok", "issue_key": args.IssueKey, "deleted_worklog_id": args.WorklogID})
	})
}

// --- Helpers ---

func parseDateRange(sinceStr, untilStr string) (time.Time, time.Time, error) {
	var since, until time.Time
	var err error
	formats := []string{time.DateOnly, time.RFC3339}
	if sinceStr != "" {
		for _, f := range formats {
			since, err = time.Parse(f, sinceStr)
			if err == nil {
				break
			}
		}
		if err != nil {
			return since, until, fmt.Errorf("invalid since date %q: %w", sinceStr, err)
		}
	}
	if untilStr != "" {
		for _, f := range formats {
			until, err = time.Parse(f, untilStr)
			if err == nil {
				break
			}
		}
		if err != nil {
			return since, until, fmt.Errorf("invalid until date %q: %w", untilStr, err)
		}
	}
	return since, until, nil
}

func fieldKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
