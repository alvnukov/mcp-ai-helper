package confluence

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func writeConfluenceTestResponse(t *testing.T, w http.ResponseWriter, body []byte) {
	t.Helper()
	if _, err := w.Write(body); err != nil {
		t.Errorf("write test response: %v", err)
	}
}

func TestNewClient_MissingURL(t *testing.T) {
	_, err := NewClient(Config{URL: ""})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewClient_MissingAPIKey(t *testing.T) {
	_, err := NewClient(Config{URL: "https://example.com/wiki/rest/api"})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestNewClient_Success(t *testing.T) {
	c, err := NewClient(Config{
		URL:    "https://example.com/wiki/rest/api",
		APIKey: "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected client")
	}
}

func TestNewClient_EnvAPIKey(t *testing.T) {
	t.Setenv("TEST_CONF_KEY", "env-token")

	c, err := NewClient(Config{
		URL:       "https://example.com/wiki/rest/api",
		APIKeyEnv: "TEST_CONF_KEY",
	})
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("expected client")
	}
}

func TestSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeConfluenceTestResponse(t, w, []byte(`{"results":[{"id":"123","type":"page","title":"Test Page","status":"current"}],"totalSize":1}`))
	}))
	defer srv.Close()

	c, err := NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}

	results, err := c.Search("title ~ Test", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Test Page" {
		t.Fatalf("unexpected results: %+v", results)
	}
}

func TestGetContentByID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeConfluenceTestResponse(t, w, []byte(`{"id":"123","type":"page","title":"Test Page","body":{"storage":{"value":"<p>hello</p>","representation":"storage"}},"version":{"number":1}}`))
	}))
	defer srv.Close()

	c, err := NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}

	page, err := c.GetContentByID("123")
	if err != nil {
		t.Fatal(err)
	}
	if page.ID != "123" || page.Title != "Test Page" {
		t.Fatalf("unexpected page: %+v", page)
	}
}

func TestGetSpaces(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		writeConfluenceTestResponse(t, w, []byte(`{"results":[{"id":1,"key":"DEV","name":"Development","type":"global"}],"size":1}`))
	}))
	defer srv.Close()

	c, err := NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}

	spaces, err := c.GetSpaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(spaces) != 1 || spaces[0].Key != "DEV" {
		t.Fatalf("unexpected spaces: %+v", spaces)
	}
}

func TestSearch_NestedContentID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Search returns nested content.id (real Confluence API format)
		writeConfluenceTestResponse(t, w, []byte(`{"results":[{"content":{"id":"999","type":"page"},"title":"Nested Page"}],"totalSize":1}`))
	}))
	defer srv.Close()

	c, err := NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}

	results, err := c.Search("title ~ Nested", 10)
	if err != nil {
		t.Fatal(err)
	}
	if results[0].ID != "999" {
		t.Fatalf("expected ID 999 from nested content, got %q", results[0].ID)
	}
}

func TestGetSpaces_Pagination(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		callCount++
		if callCount == 1 {
			writeConfluenceTestResponse(t, w, []byte(`{"results":[{"id":1,"key":"A"},{"id":2,"key":"B"},{"id":3,"key":"C"}],"size":3}`))
		} else {
			writeConfluenceTestResponse(t, w, []byte(`{"results":[],"size":0}`))
		}
	}))
	defer srv.Close()

	c, err := NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}

	spaces, err := c.GetSpaces()
	if err != nil {
		t.Fatal(err)
	}
	if len(spaces) != 3 {
		t.Fatalf("expected 3 spaces, got %d", len(spaces))
	}
}

func TestSearchContextCancellation(t *testing.T) {
	requestStarted := make(chan struct{})
	handlerStopped := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		close(requestStarted)
		<-r.Context().Done()
		close(handlerStopped)
	}))
	defer srv.Close()

	client, err := NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, searchErr := client.SearchContext(ctx, "title ~ Test", 10)
		result <- searchErr
	}()

	select {
	case <-requestStarted:
	case <-time.After(time.Second):
		t.Fatal("Confluence request did not start")
	}
	cancel()

	select {
	case searchErr := <-result:
		if !errors.Is(searchErr, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", searchErr)
		}
	case <-time.After(time.Second):
		t.Fatal("SearchContext did not return after cancellation")
	}
	select {
	case <-handlerStopped:
	case <-time.After(time.Second):
		t.Fatal("HTTP handler did not observe request cancellation")
	}
}

func TestSearchContextExpiredDeadline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unexpected request", http.StatusInternalServerError)
	}))
	defer srv.Close()

	client, err := NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	_, err = client.SearchContext(ctx, "title ~ Test", 10)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
}
