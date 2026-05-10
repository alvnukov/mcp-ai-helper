package jira

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	gojira "github.com/andygrunwald/go-jira"

	"github.com/zol/mcp-ai-helper/internal/config"
)

func newTestClient(url string) (*gojira.Client, error) {
	return gojira.NewClient(http.DefaultClient, url)
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

func TestSearchIssues(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"issues":[{"key":"TEST-1","fields":{"summary":"test","status":{"name":"Open"}}}]}`))
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

func TestWorklogOperations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.Method {
		case http.MethodGet:
			w.Write([]byte(`{"worklogs":[{"id":"1","timeSpent":"1h","timeSpentSeconds":3600,"author":{"name":"testuser"},"started":"2026-05-10T10:00:00.000+0000"}]}`))
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

	records, err := c.GetWorklogs("TEST-1", time.Time{}, time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}

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

func strPtr(s string) *string { return &s }
