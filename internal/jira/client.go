// Package jira wraps the go-jira client with a focused interface for mcp-ai-helper tools.
package jira

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	gojira "github.com/andygrunwald/go-jira"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// Client wraps the go-jira client with our domain methods.
type Client struct {
	jc *gojira.Client
}

// NewClient creates a Jira client from config.
func NewClient(cfg config.JiraConfig) (*Client, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("jira: url is required")
	}
	apiKey := cfg.ResolvedAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("jira: api key is required — set api_key or api_key_env")
	}
	var httpClient *http.Client
	if cfg.Username != "" {
		tp := gojira.BasicAuthTransport{
			Username: cfg.Username,
			Password: apiKey,
		}
		httpClient = tp.Client()
	} else {
		tp := gojira.BearerAuthTransport{
			Token: apiKey,
		}
		httpClient = tp.Client()
	}
	jc, err := gojira.NewClient(httpClient, cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("jira: connect to %s: %w", cfg.URL, err)
	}
	return &Client{jc: jc}, nil
}

// --- Issue operations ---

func closeJiraResponse(resp *gojira.Response) error {
	if resp == nil || resp.Body == nil {
		return nil
	}
	if err := resp.Body.Close(); err != nil {
		return fmt.Errorf("jira response body close: %w", err)
	}
	return nil
}

func joinJiraResponseError(err error, resp *gojira.Response) error {
	return errors.Join(err, closeJiraResponse(resp))
}

// SearchIssues searches issues by JQL.
func (c *Client) SearchIssues(jql string, maxResults int) ([]gojira.Issue, error) {
	return c.SearchIssuesContext(context.Background(), jql, maxResults)
}

// SearchIssuesContext searches issues by JQL and propagates cancellation to Jira.
func (c *Client) SearchIssuesContext(ctx context.Context, jql string, maxResults int) ([]gojira.Issue, error) {
	issues, resp, err := c.jc.Issue.SearchWithContext(ctx, jql, &gojira.SearchOptions{
		MaxResults: maxResults,
	})
	if err = joinJiraResponseError(err, resp); err != nil {
		return nil, fmt.Errorf("jira search: %w", err)
	}
	return issues, nil
}

// GetIssue returns a single issue by key.
func (c *Client) GetIssue(key string) (*gojira.Issue, error) {
	return c.GetIssueContext(context.Background(), key)
}

// GetIssueContext returns a single issue by key and propagates cancellation to Jira.
func (c *Client) GetIssueContext(ctx context.Context, key string) (*gojira.Issue, error) {
	issue, resp, err := c.jc.Issue.GetWithContext(ctx, key, nil)
	if err = joinJiraResponseError(err, resp); err != nil {
		return nil, fmt.Errorf("jira get %s: %w", key, err)
	}
	return issue, nil
}

// UpdateIssue updates issue fields.
func (c *Client) UpdateIssue(key string, fields map[string]interface{}) error {
	return c.UpdateIssueContext(context.Background(), key, fields)
}

// UpdateIssueContext updates issue fields and propagates cancellation to Jira.
func (c *Client) UpdateIssueContext(ctx context.Context, key string, fields map[string]interface{}) error {
	unknowns := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		unknowns[k] = v
	}
	issue := &gojira.Issue{
		Key: key,
		Fields: &gojira.IssueFields{
			Unknowns: unknowns,
		},
	}
	_, resp, err := c.jc.Issue.UpdateWithContext(ctx, issue)
	if err = joinJiraResponseError(err, resp); err != nil {
		return fmt.Errorf("jira update %s: %w", key, err)
	}
	return nil
}

// SetIssueProperty sets an entity property on an issue.
func (c *Client) SetIssueProperty(issueKey, propertyKey string, value interface{}) error {
	return c.SetIssuePropertyContext(context.Background(), issueKey, propertyKey, value)
}

// SetIssuePropertyContext sets an entity property and propagates cancellation to Jira.
func (c *Client) SetIssuePropertyContext(ctx context.Context, issueKey, propertyKey string, value interface{}) error {
	path := fmt.Sprintf("rest/api/2/issue/%s/properties/%s", issueKey, propertyKey)
	req, err := c.jc.NewRequestWithContext(ctx, http.MethodPut, path, value)
	if err != nil {
		return fmt.Errorf("jira set property %s %s: %w", issueKey, propertyKey, err)
	}
	resp, err := c.jc.Do(req, nil)
	if err != nil {
		return fmt.Errorf("jira set property %s %s: %w", issueKey, propertyKey, joinJiraResponseError(err, resp))
	}
	var responseErr error
	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			responseErr = fmt.Errorf("jira set property %s %s: HTTP %d (body: %s, read error: %w)", issueKey, propertyKey, resp.StatusCode, strings.TrimSpace(string(body)), readErr)
		} else {
			responseErr = fmt.Errorf("jira set property %s %s: HTTP %d: %s", issueKey, propertyKey, resp.StatusCode, strings.TrimSpace(string(body)))
		}
	}
	return errors.Join(responseErr, closeJiraResponse(resp))
}

// GetIssueProperty reads an entity property from an issue.
func (c *Client) GetIssueProperty(issueKey, propertyKey string, v interface{}) error {
	return c.GetIssuePropertyContext(context.Background(), issueKey, propertyKey, v)
}

// GetIssuePropertyContext reads an entity property and propagates cancellation to Jira.
func (c *Client) GetIssuePropertyContext(ctx context.Context, issueKey, propertyKey string, v interface{}) error {
	path := fmt.Sprintf("rest/api/2/issue/%s/properties/%s", issueKey, propertyKey)
	req, err := c.jc.NewRequestWithContext(ctx, http.MethodGet, path, nil)
	if err != nil {
		return fmt.Errorf("jira get property %s %s: %w", issueKey, propertyKey, err)
	}
	resp, err := c.jc.Do(req, v)
	if err != nil {
		return fmt.Errorf("jira get property %s %s: %w", issueKey, propertyKey, joinJiraResponseError(err, resp))
	}
	var responseErr error
	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			responseErr = fmt.Errorf("jira get property %s %s: HTTP %d (body: %s, read error: %w)", issueKey, propertyKey, resp.StatusCode, strings.TrimSpace(string(body)), readErr)
		} else {
			responseErr = fmt.Errorf("jira get property %s %s: HTTP %d: %s", issueKey, propertyKey, resp.StatusCode, strings.TrimSpace(string(body)))
		}
	}
	return errors.Join(responseErr, closeJiraResponse(resp))
}

// GetTransitions returns available transitions for an issue.
func (c *Client) GetTransitions(key string) ([]gojira.Transition, error) {
	return c.GetTransitionsContext(context.Background(), key)
}

// GetTransitionsContext returns available transitions and propagates cancellation to Jira.
func (c *Client) GetTransitionsContext(ctx context.Context, key string) ([]gojira.Transition, error) {
	transitions, resp, err := c.jc.Issue.GetTransitionsWithContext(ctx, key)
	if err = joinJiraResponseError(err, resp); err != nil {
		return nil, fmt.Errorf("jira transitions %s: %w", key, err)
	}
	return transitions, nil
}

// DoTransition performs a transition by name.
func (c *Client) DoTransition(key, transitionName string) error {
	return c.DoTransitionContext(context.Background(), key, transitionName)
}

// DoTransitionContext performs a transition by name and propagates cancellation to Jira.
func (c *Client) DoTransitionContext(ctx context.Context, key, transitionName string) error {
	transitions, err := c.GetTransitionsContext(ctx, key)
	if err != nil {
		return err
	}
	var transitionID string
	for _, transition := range transitions {
		if transition.Name == transitionName {
			transitionID = transition.ID
			break
		}
	}
	if transitionID == "" {
		return fmt.Errorf("jira transition %s: %q not found", key, transitionName)
	}
	resp, err := c.jc.Issue.DoTransitionWithContext(ctx, key, transitionID)
	if err = joinJiraResponseError(err, resp); err != nil {
		return fmt.Errorf("jira transition %s to %q: %w", key, transitionName, err)
	}
	return nil
}

// AssignIssue assigns an issue to a user.
func (c *Client) AssignIssue(key, username string) error {
	return c.AssignIssueContext(context.Background(), key, username)
}

// AssignIssueContext assigns an issue and propagates cancellation to Jira.
func (c *Client) AssignIssueContext(ctx context.Context, key, username string) error {
	resp, err := c.jc.Issue.UpdateAssigneeWithContext(ctx, key, &gojira.User{Name: username})
	if err = joinJiraResponseError(err, resp); err != nil {
		return fmt.Errorf("jira assign %s to %s: %w", key, username, err)
	}
	return nil
}

// UnassignIssue removes the assignee.
func (c *Client) UnassignIssue(key string) error {
	return c.UnassignIssueContext(context.Background(), key)
}

// UnassignIssueContext removes the assignee and propagates cancellation to Jira.
func (c *Client) UnassignIssueContext(ctx context.Context, key string) error {
	resp, err := c.jc.Issue.UpdateAssigneeWithContext(ctx, key, nil)
	if err = joinJiraResponseError(err, resp); err != nil {
		return fmt.Errorf("jira unassign %s: %w", key, err)
	}
	return nil
}

// --- Worklog operations ---

// GetWorklogs returns worklogs for an issue, optionally filtered by date range.
func (c *Client) GetWorklogs(key string, since, until time.Time) ([]gojira.WorklogRecord, error) {
	return c.GetWorklogsContext(context.Background(), key, since, until)
}

// GetWorklogsContext returns worklogs and propagates cancellation to every page request.
func (c *Client) GetWorklogsContext(ctx context.Context, key string, since, until time.Time) ([]gojira.WorklogRecord, error) {
	var all []gojira.WorklogRecord
	startAt := 0
	const pageSize = 50
	for {
		worklogs, resp, err := c.jc.Issue.GetWorklogsWithContext(ctx, key, func(r *http.Request) error {
			q := r.URL.Query()
			q.Set("startAt", strconv.Itoa(startAt))
			q.Set("maxResults", strconv.Itoa(pageSize))
			r.URL.RawQuery = q.Encode()
			return nil
		})
		if err = joinJiraResponseError(err, resp); err != nil {
			return nil, fmt.Errorf("jira worklogs %s: %w", key, err)
		}
		all = append(all, worklogs.Worklogs...)
		if len(worklogs.Worklogs) < pageSize || len(all) >= worklogs.Total {
			break
		}
		startAt += pageSize
	}
	if since.IsZero() && until.IsZero() {
		return all, nil
	}
	var filtered []gojira.WorklogRecord
	for _, record := range all {
		if record.Started == nil {
			continue
		}
		started := time.Time(*record.Started)
		if !since.IsZero() && started.Before(since) {
			continue
		}
		if !until.IsZero() && started.After(until) {
			continue
		}
		filtered = append(filtered, record)
	}
	return filtered, nil
}

// WorklogEntry pairs a worklog record with its issue key.
type WorklogEntry struct {
	IssueKey string
	Record   gojira.WorklogRecord
}

// GetWorklogsByUser searches worklogs by user in a date range.
func (c *Client) GetWorklogsByUser(username string, since, until time.Time) ([]WorklogEntry, error) {
	return c.GetWorklogsByUserContext(context.Background(), username, since, until)
}

// GetWorklogsByUserContext searches worklogs by user and propagates cancellation to every request.
func (c *Client) GetWorklogsByUserContext(ctx context.Context, username string, since, until time.Time) ([]WorklogEntry, error) {
	escaped := username
	escaped = strings.ReplaceAll(escaped, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	jql := fmt.Sprintf("worklogAuthor = \"%s\"", escaped)
	if !since.IsZero() {
		jql += fmt.Sprintf(" AND worklogDate >= %s", since.Format("2006-01-02"))
	}
	if !until.IsZero() {
		jql += fmt.Sprintf(" AND worklogDate <= %s", until.Format("2006-01-02"))
	}
	issues, err := c.SearchIssuesContext(ctx, jql, 100)
	if err != nil {
		return nil, fmt.Errorf("jira worklogs by user %s: %w", username, err)
	}
	var entries []WorklogEntry
	for _, issue := range issues {
		records, err := c.GetWorklogsContext(ctx, issue.Key, since, until)
		if err != nil {
			return nil, fmt.Errorf("jira worklogs by user %s issue %s: %w", username, issue.Key, err)
		}
		for _, record := range records {
			if record.Author != nil && record.Author.Name == username {
				entries = append(entries, WorklogEntry{IssueKey: issue.Key, Record: record})
			}
		}
	}
	return entries, nil
}

// AddWorklog adds a worklog entry.
func (c *Client) AddWorklog(key, timeSpent, comment string, started *time.Time) (*gojira.WorklogRecord, error) {
	return c.AddWorklogContext(context.Background(), key, timeSpent, comment, started)
}

// AddWorklogContext adds a worklog entry and propagates cancellation to Jira.
func (c *Client) AddWorklogContext(ctx context.Context, key, timeSpent, comment string, started *time.Time) (*gojira.WorklogRecord, error) {
	record := &gojira.WorklogRecord{
		Comment:   comment,
		TimeSpent: timeSpent,
	}
	if started != nil {
		record.Started = (*gojira.Time)(started)
	}
	created, resp, err := c.jc.Issue.AddWorklogRecordWithContext(ctx, key, record)
	if err = joinJiraResponseError(err, resp); err != nil {
		return nil, fmt.Errorf("jira add worklog %s: %w", key, err)
	}
	return created, nil
}

// UpdateWorklog updates a worklog entry.
func (c *Client) UpdateWorklog(key, worklogID string, timeSpent *string, comment *string) error {
	return c.UpdateWorklogContext(context.Background(), key, worklogID, timeSpent, comment)
}

// UpdateWorklogContext updates a worklog entry and propagates cancellation to Jira.
func (c *Client) UpdateWorklogContext(ctx context.Context, key, worklogID string, timeSpent *string, comment *string) error {
	body := map[string]interface{}{}
	if timeSpent != nil {
		body["timeSpent"] = *timeSpent
	}
	if comment != nil {
		body["comment"] = *comment
	}
	if len(body) == 0 {
		return nil
	}
	urlStr := fmt.Sprintf("rest/api/2/issue/%s/worklog/%s", key, worklogID)
	req, err := c.jc.NewRequestWithContext(ctx, http.MethodPut, urlStr, body)
	if err != nil {
		return fmt.Errorf("jira update worklog %s/%s: %w", key, worklogID, err)
	}
	resp, err := c.jc.Do(req, nil)
	if err = joinJiraResponseError(err, resp); err != nil {
		return fmt.Errorf("jira update worklog %s/%s: %w", key, worklogID, err)
	}
	return nil
}

// DeleteWorklog deletes a worklog entry.
func (c *Client) DeleteWorklog(key, worklogID string) error {
	return c.DeleteWorklogContext(context.Background(), key, worklogID)
}

// DeleteWorklogContext deletes a worklog entry and propagates cancellation to Jira.
func (c *Client) DeleteWorklogContext(ctx context.Context, key, worklogID string) error {
	urlStr := fmt.Sprintf("rest/api/2/issue/%s/worklog/%s", key, worklogID)
	req, err := c.jc.NewRequestWithContext(ctx, http.MethodDelete, urlStr, nil)
	if err != nil {
		return fmt.Errorf("jira delete worklog %s/%s: %w", key, worklogID, err)
	}
	resp, err := c.jc.Do(req, nil)
	if err = joinJiraResponseError(err, resp); err != nil {
		return fmt.Errorf("jira delete worklog %s/%s: %w", key, worklogID, err)
	}
	return nil
}
