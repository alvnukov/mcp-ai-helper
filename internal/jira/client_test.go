package jira

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	gojira "github.com/andygrunwald/go-jira"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func newTestClient(url string) (*gojira.Client, error) {
	return gojira.NewClient(http.DefaultClient, url)
}

// --- Constructor tests ---

func TestNewClient_BasicAuth(t *testing.T) {
	c, err := NewClient(config.JiraConfig{
		URL:      "https://example.atlassian.net",
		Username: "user@example.com",
		APIKey:   "secret-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected client")
	}
}

func TestNewClient_BearerAuth(t *testing.T) {
	c, err := NewClient(config.JiraConfig{
		URL:    "https://example.atlassian.net",
		APIKey: "pat-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected client")
	}
}

func TestNewClient_EnvAPIKey(t *testing.T) {
	os.Setenv("TEST_JIRA_KEY", "env-key-value")
	defer os.Unsetenv("TEST_JIRA_KEY")

	c, err := NewClient(config.JiraConfig{
		URL:       "https://example.atlassian.net",
		Username:  "user@example.com",
		APIKeyEnv: "TEST_JIRA_KEY",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected client")
	}
}

func TestNewClient_MissingURL(t *testing.T) {
	_, err := NewClient(config.JiraConfig{URL: ""})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewClient_MissingAPIKey(t *testing.T) {
	_, err := NewClient(config.JiraConfig{URL: "https://example.atlassian.net"})
	if err == nil {
		t.Fatal("expected error")
	}
}

// --- Issue tests ---

func TestSearchIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issues":[{"key":"TEST-1","fields":{"summary":"test","status":{"name":"Open"},"priority":{"name":"High"},"assignee":{"displayName":"User"}}}]}`))
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	issues, err := c.SearchIssues("project=TEST", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(issues) != 1 || issues[0].Key != "TEST-1" {
		t.Fatalf("unexpected issues: %+v", issues)
	}
}

func TestGetIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"key":"TEST-1","fields":{"summary":"test issue"}}`))
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	issue, err := c.GetIssue("TEST-1")
	if err != nil {
		t.Fatal(err)
	}
	if issue.Key != "TEST-1" {
		t.Fatalf("unexpected key: %s", issue.Key)
	}
}

func TestUpdateIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			w.Write([]byte(`{"key":"TEST-1","fields":{"summary":"old","unknowns":{}}}`))
			return
		}
		// PUT — update
		w.Write([]byte(`{"key":"TEST-1","fields":{"summary":"new"}}`))
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	err = c.UpdateIssue("TEST-1", map[string]interface{}{"summary": "new"})
	if err != nil {
		t.Fatal(err)
	}
}

// --- Transition tests ---

func TestGetTransitions(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"transitions":[{"id":"1","name":"Done"}]}`))
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	transitions, err := c.GetTransitions("TEST-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(transitions) != 1 || transitions[0].Name != "Done" {
		t.Fatalf("unexpected transitions: %+v", transitions)
	}
}

func TestDoTransition_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"transitions":[{"id":"1","name":"In Progress"}]}`))
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	err = c.DoTransition("TEST-1", "Done")
	if err == nil {
		t.Fatal("expected error for missing transition")
	}
}

// --- Assign tests ---

func TestAssignIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	err = c.AssignIssue("TEST-1", "testuser")
	if err != nil {
		t.Fatal(err)
	}
}

func TestUnassignIssue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	err = c.UnassignIssue("TEST-1")
	if err != nil {
		t.Fatal(err)
	}
}

// --- Worklog tests ---

func TestGetWorklogs_DateFilter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"startAt":0,"maxResults":50,"total":1,"worklogs":[{"id":"1","timeSpent":"1h","timeSpentSeconds":3600,"author":{"name":"user"},"started":"2026-05-05T10:00:00.000+0000"}]}`))
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	// within range
	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	records, err := c.GetWorklogs("TEST-1", since, until)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

	// outside range
	since = time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	records, err = c.GetWorklogs("TEST-1", since, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records, got %d", len(records))
	}
}

func TestGetWorklogs_NilStarted(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// worklog with nil started — should be skipped in filtered results
		w.Write([]byte(`{"startAt":0,"maxResults":50,"total":1,"worklogs":[{"id":"1","timeSpent":"1h","timeSpentSeconds":3600,"author":{"name":"user"}}]}`))
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// nil Started → skipped by date filter, should not panic
	records, err := c.GetWorklogs("TEST-1", since, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 0 {
		t.Fatalf("expected 0 records for nil started, got %d", len(records))
	}
}

func TestGetWorklogs_Pagination(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		// return full page (50 items) on first call, then less (20) on second
		if callCount == 1 {
			w.Write([]byte(`{"startAt":0,"maxResults":50,"total":70,"worklogs":[` + repeatWorklog(50, `"id":"1"`) + `]}`))
		} else {
			w.Write([]byte(`{"startAt":50,"maxResults":50,"total":70,"worklogs":[` + repeatWorklog(20, `"id":"2"`) + `]}`))
		}
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	records, err := c.GetWorklogs("TEST-1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if callCount < 2 {
		t.Fatalf("expected at least 2 pages, got %d calls", callCount)
	}
	if len(records) != 70 {
		t.Fatalf("expected 70 records, got %d", len(records))
	}
}

func repeatWorklog(n int, id string) string {
	var s string
	for i := 0; i < n; i++ {
		if i > 0 {
			s += ","
		}
		s += `{` + id + `,"timeSpent":"1h","timeSpentSeconds":3600}`
	}
	return s
}

func TestWorklogCRUD(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.Write([]byte(`{"startAt":0,"maxResults":50,"total":1,"worklogs":[{"id":"1","timeSpent":"1h","timeSpentSeconds":3600,"author":{"name":"testuser"},"started":"2026-05-10T10:00:00.000+0000"}]}`))
		case http.MethodPost:
			w.Write([]byte(`{"id":"2","timeSpent":"2h","timeSpentSeconds":7200}`))
		default:
			w.WriteHeader(200)
		}
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	record, err := c.AddWorklog("TEST-1", "2h", "test comment", nil)
	if err != nil {
		t.Fatal(err)
	}
	if record.ID != "2" {
		t.Fatalf("unexpected record id: %s", record.ID)
	}

	err = c.UpdateWorklog("TEST-1", "1", strPtr("3h"), nil)
	if err != nil {
		t.Fatal(err)
	}

	err = c.DeleteWorklog("TEST-1", "1")
	if err != nil {
		t.Fatal(err)
	}
}

// --- GetWorklogsByUser ---

func TestGetWorklogsByUser(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		path := r.URL.Path
		if path == "/rest/api/2/search" {
			w.Write([]byte(`{"issues":[{"key":"TEST-1","fields":{"summary":"task"}}]}`))
			return
		}
		if path == "/rest/api/2/issue/TEST-1/worklog" {
			w.Write([]byte(`{"startAt":0,"maxResults":50,"total":1,"worklogs":[{"id":"1","timeSpent":"2h","timeSpentSeconds":7200,"author":{"name":"testuser"},"started":"2026-05-05T10:00:00.000+0000"}]}`))
			return
		}
		w.WriteHeader(404)
	}))
	defer srv.Close()

	jc, err := newTestClient(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	c := &Client{jc: jc}

	since := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 5, 10, 0, 0, 0, 0, time.UTC)
	entries, err := c.GetWorklogsByUser("testuser", since, until)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].IssueKey != "TEST-1" {
		t.Fatalf("expected issue key TEST-1, got %s", entries[0].IssueKey)
	}
}

func strPtr(s string) *string { return &s }
