// Package jira wraps the go-jira client with a focused interface for mcp-ai-helper tools.
package jira

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	gojira "github.com/andygrunwald/go-jira"

	"github.com/zol/mcp-ai-helper/internal/config"
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

// SearchIssues searches issues by JQL.
func (c *Client) SearchIssues(jql string, maxResults int) ([]gojira.Issue, error) {
	issues, _, err := c.jc.Issue.Search(jql, &gojira.SearchOptions{
		MaxResults: maxResults,
	})
	if err != nil {
		return nil, fmt.Errorf("jira search: %w", err)
	}
	return issues, nil
}

// GetIssue returns a single issue by key.
func (c *Client) GetIssue(key string) (*gojira.Issue, error) {
	issue, _, err := c.jc.Issue.Get(key, nil)
	if err != nil {
		return nil, fmt.Errorf("jira get %s: %w", key, err)
	}
	return issue, nil
}

// UpdateIssue updates issue fields.
func (c *Client) UpdateIssue(key string, fields map[string]interface{}) error {
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
	_, _, err := c.jc.Issue.Update(issue)
	if err != nil {
		return fmt.Errorf("jira update %s: %w", key, err)
	}
	return nil
}

// SetIssueProperty sets an entity property on an issue.
func (c *Client) SetIssueProperty(issueKey, propertyKey string, value interface{}) error {
	path := fmt.Sprintf("rest/api/2/issue/%s/properties/%s", issueKey, propertyKey)
	req, err := c.jc.NewRequest("PUT", path, value)
	if err != nil {
		return fmt.Errorf("jira set property %s %s: %w", issueKey, propertyKey, err)
	}
	resp, err := c.jc.Do(req, nil)
	if err != nil {
		return fmt.Errorf("jira set property %s %s: %w", issueKey, propertyKey, err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("jira set property %s %s: HTTP %d: %s", issueKey, propertyKey, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// GetIssueProperty reads an entity property from an issue.
func (c *Client) GetIssueProperty(issueKey, propertyKey string, v interface{}) error {
	path := fmt.Sprintf("rest/api/2/issue/%s/properties/%s", issueKey, propertyKey)
	req, err := c.jc.NewRequest("GET", path, nil)
	if err != nil {
		return fmt.Errorf("jira get property %s %s: %w", issueKey, propertyKey, err)
	}
	resp, err := c.jc.Do(req, v)
	if err != nil {
		return fmt.Errorf("jira get property %s %s: %w", issueKey, propertyKey, err)
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return fmt.Errorf("jira get property %s %s: HTTP %d: %s", issueKey, propertyKey, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// GetTransitions returns available transitions for an issue.
func (c *Client) GetTransitions(key string) ([]gojira.Transition, error) {
	transitions, _, err := c.jc.Issue.GetTransitions(key)
	if err != nil {
		return nil, fmt.Errorf("jira transitions %s: %w", key, err)
	}
	return transitions, nil
}

// DoTransition performs a transition by name.
func (c *Client) DoTransition(key, transitionName string) error {
	transitions, err := c.GetTransitions(key)
	if err != nil {
		return err
	}
	var transitionID string
	for _, t := range transitions {
		if t.Name == transitionName {
			transitionID = t.ID
			break
		}
	}
	if transitionID == "" {
		return fmt.Errorf("jira transition %s: %q not found", key, transitionName)
	}
	_, err = c.jc.Issue.DoTransition(key, transitionID)
	if err != nil {
		return fmt.Errorf("jira transition %s to %q: %w", key, transitionName, err)
	}
	return nil
}

// AssignIssue assigns an issue to a user.
func (c *Client) AssignIssue(key, username string) error {
	_, err := c.jc.Issue.UpdateAssignee(key, &gojira.User{Name: username})
	if err != nil {
		return fmt.Errorf("jira assign %s to %s: %w", key, username, err)
	}
	return nil
}

// UnassignIssue removes the assignee.
func (c *Client) UnassignIssue(key string) error {
	_, err := c.jc.Issue.UpdateAssignee(key, nil)
	if err != nil {
		return fmt.Errorf("jira unassign %s: %w", key, err)
	}
	return nil
}

// --- Worklog operations ---

// GetWorklogs returns worklogs for an issue, optionally filtered by date range.
func (c *Client) GetWorklogs(key string, since, until time.Time) ([]gojira.WorklogRecord, error) {
	var all []gojira.WorklogRecord
	startAt := 0
	const pageSize = 50
	for {
		wl, _, err := c.jc.Issue.GetWorklogs(key, func(r *http.Request) error {
			q := r.URL.Query()
			q.Set("startAt", strconv.Itoa(startAt))
			q.Set("maxResults", strconv.Itoa(pageSize))
			r.URL.RawQuery = q.Encode()
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("jira worklogs %s: %w", key, err)
		}
		all = append(all, wl.Worklogs...)
		if len(wl.Worklogs) < pageSize || len(all) >= wl.Total {
			break
		}
		startAt += pageSize
	}
	if since.IsZero() && until.IsZero() {
		return all, nil
	}
	var filtered []gojira.WorklogRecord
	for _, r := range all {
		if r.Started == nil {
			continue
		}
		started := time.Time(*r.Started)
		if !since.IsZero() && started.Before(since) {
			continue
		}
		if !until.IsZero() && started.After(until) {
			continue
		}
		filtered = append(filtered, r)
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
	issues, err := c.SearchIssues(jql, 100)
	if err != nil {
		return nil, fmt.Errorf("jira worklogs by user %s: %w", username, err)
	}
	var entries []WorklogEntry
	for _, issue := range issues {
		records, err := c.GetWorklogs(issue.Key, since, until)
		if err != nil {
			continue
		}
		for _, r := range records {
			if r.Author != nil && r.Author.Name == username {
				entries = append(entries, WorklogEntry{IssueKey: issue.Key, Record: r})
			}
		}
	}
	return entries, nil
}

// AddWorklog adds a worklog entry.
func (c *Client) AddWorklog(key, timeSpent, comment string, started *time.Time) (*gojira.WorklogRecord, error) {
	record := &gojira.WorklogRecord{
		Comment:   comment,
		TimeSpent: timeSpent,
	}
	if started != nil {
		record.Started = (*gojira.Time)(started)
	}
	r, _, err := c.jc.Issue.AddWorklogRecord(key, record)
	if err != nil {
		return nil, fmt.Errorf("jira add worklog %s: %w", key, err)
	}
	return r, nil
}

// UpdateWorklog updates a worklog entry.
func (c *Client) UpdateWorklog(key, worklogID string, timeSpent *string, comment *string) error {
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
	req, err := c.jc.NewRequest(http.MethodPut, urlStr, body)
	if err != nil {
		return fmt.Errorf("jira update worklog %s/%s: %w", key, worklogID, err)
	}
	_, err = c.jc.Do(req, nil)
	if err != nil {
		return fmt.Errorf("jira update worklog %s/%s: %w", key, worklogID, err)
	}
	return nil
}

// DeleteWorklog deletes a worklog entry.
func (c *Client) DeleteWorklog(key, worklogID string) error {
	urlStr := fmt.Sprintf("rest/api/2/issue/%s/worklog/%s", key, worklogID)
	req, err := c.jc.NewRequest(http.MethodDelete, urlStr, nil)
	if err != nil {
		return fmt.Errorf("jira delete worklog %s/%s: %w", key, worklogID, err)
	}
	_, err = c.jc.Do(req, nil)
	if err != nil {
		return fmt.Errorf("jira delete worklog %s/%s: %w", key, worklogID, err)
	}
	return nil
}
