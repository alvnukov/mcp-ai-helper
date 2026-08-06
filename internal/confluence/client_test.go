package confluence

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// recordedRequest is one call that reached Confluence.
type recordedRequest struct {
	method string
	body   map[string]any
}

type editRecorder struct {
	mu       sync.Mutex
	requests []recordedRequest
}

func (r *editRecorder) add(req recordedRequest) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.requests = append(r.requests, req)
}

func (r *editRecorder) all() []recordedRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedRequest(nil), r.requests...)
}

// editTestServer answers every call with response and records what reached it.
//
// The recording is the point. A test that only read the returned EditResult
// would pass just as well if a refused edit had been sent anyway and the server
// happened to answer, and it would not notice a payload Confluence rejects —
// the library marshals title, space and body whether or not they were filled
// in, so an update that omits them sends nulls rather than nothing.
func editTestServer(t *testing.T, response string) (*Client, *editRecorder) {
	t.Helper()
	recorder := &editRecorder{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		record := recordedRequest{method: r.Method}
		payload, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read request body: %v", err)
		}
		if len(payload) > 0 {
			if err := json.Unmarshal(payload, &record.body); err != nil {
				t.Errorf("request body is not JSON: %v", err)
			}
		}
		recorder.add(record)
		w.Header().Set("Content-Type", "application/json")
		writeConfluenceTestResponse(t, w, []byte(response))
	}))
	t.Cleanup(srv.Close)

	client, err := NewClientWithHTTP(srv.URL, srv.Client())
	if err != nil {
		t.Fatal(err)
	}
	return client, recorder
}

// field reads one value out of a decoded request body by path, so a test can
// name what it cares about instead of asserting through three type assertions.
func field(t *testing.T, body map[string]any, path ...string) any {
	t.Helper()
	var current any = body
	for i, step := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%v is not an object at %q", path[:i], step)
		}
		current, ok = object[step]
		if !ok {
			t.Fatalf("request body has no %v", path[:i+1])
		}
	}
	return current
}

func onlyRequest(t *testing.T, recorder *editRecorder) recordedRequest {
	t.Helper()
	requests := recorder.all()
	if len(requests) != 1 {
		t.Fatalf("expected exactly one request to Confluence, got %d", len(requests))
	}
	return requests[0]
}

func requireNoRequest(t *testing.T, recorder *editRecorder) {
	t.Helper()
	if requests := recorder.all(); len(requests) != 0 {
		t.Fatalf("a refused edit still reached Confluence: %+v", requests)
	}
}

func TestUpdatePageSendsTheFieldsConfluenceRequires(t *testing.T) {
	client, recorder := editTestServer(t, `{"id":"123","type":"page","title":"Capacity","version":{"number":8},"_links":{"webui":"/display/VEGA/Capacity"}}`)
	current := &PageInfo{ID: "123", Type: "page", Title: "Capacity", Space: "VEGA", Version: 7, Body: "<p>old</p>"}

	result, err := client.UpdatePageContext(context.Background(), current, UpdateRequest{
		PageID: "123", ExpectedVersion: 7, Body: "<p>new</p>", VersionMessage: "helper edit",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || !result.Changed || result.Version != 8 {
		t.Fatalf("unexpected result: %+v", result)
	}

	sent := onlyRequest(t, recorder)
	if sent.method != http.MethodPut {
		t.Errorf("method = %s, want PUT", sent.method)
	}
	// An update carrying no space or title is not a partial update: the library
	// marshals those fields as null and Confluence refuses the request.
	if got := field(t, sent.body, "space", "key"); got != "VEGA" {
		t.Errorf("space.key = %v, want VEGA", got)
	}
	if got := field(t, sent.body, "title"); got != "Capacity" {
		t.Errorf("title = %v, want the current title", got)
	}
	if got := field(t, sent.body, "type"); got != "page" {
		t.Errorf("type = %v, want page", got)
	}
	if got := field(t, sent.body, "body", "storage", "representation"); got != "storage" {
		t.Errorf("representation = %v, want storage", got)
	}
	if got := field(t, sent.body, "body", "storage", "value"); got != "<p>new</p>" {
		t.Errorf("body = %v, want the new body", got)
	}
	if got := field(t, sent.body, "version", "number"); got != float64(8) {
		t.Errorf("version.number = %v, want 8: Confluence accepts an update only at the next version", got)
	}
	if got := field(t, sent.body, "version", "message"); got != "helper edit" {
		t.Errorf("version.message = %v, want the caller's message", got)
	}
}

func TestUpdatePageReplacesOneSpanOfTheCurrentBody(t *testing.T) {
	client, recorder := editTestServer(t, `{"id":"123","version":{"number":4}}`)
	current := &PageInfo{ID: "123", Type: "page", Title: "Capacity", Space: "VEGA", Version: 3,
		Body: "<table><tr><td>2 CPU</td><td>8 GB</td></tr></table>"}

	result, err := client.UpdatePageContext(context.Background(), current, UpdateRequest{
		PageID: "123", ExpectedVersion: 3, Old: "<td>2 CPU</td>", New: "<td>4 CPU</td>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || !result.Changed {
		t.Fatalf("unexpected result: %+v", result)
	}

	sent := onlyRequest(t, recorder)
	want := "<table><tr><td>4 CPU</td><td>8 GB</td></tr></table>"
	if got := field(t, sent.body, "body", "storage", "value"); got != want {
		t.Errorf("body = %v, want %q: the rest of the page must survive a span edit", got, want)
	}
}

// A page the caller has not re-read is a page the caller does not know. Sending
// version+1 from a stale read is how a concurrent edit gets overwritten, so the
// guard has to stop before the request, not report afterwards.
func TestUpdatePageRefusesAStaleVersionWithoutWriting(t *testing.T) {
	client, recorder := editTestServer(t, `{"id":"123"}`)
	current := &PageInfo{ID: "123", Type: "page", Title: "Capacity", Space: "VEGA", Version: 9, Body: "<p>old</p>"}

	result, err := client.UpdatePageContext(context.Background(), current, UpdateRequest{
		PageID: "123", ExpectedVersion: 7, Body: "<p>new</p>",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "conflict" || result.Changed {
		t.Fatalf("expected a conflict, got %+v", result)
	}
	if result.Version != 9 {
		t.Errorf("version = %d, want the version the page is actually at", result.Version)
	}
	requireNoRequest(t, recorder)
}

func TestUpdatePageRefusesASpanThatIsNotUnique(t *testing.T) {
	client, recorder := editTestServer(t, `{"id":"123"}`)
	current := &PageInfo{ID: "123", Type: "page", Title: "Capacity", Space: "VEGA", Version: 3,
		Body: "<p>2 CPU</p><p>2 CPU</p>"}

	result, err := client.UpdatePageContext(context.Background(), current, UpdateRequest{
		PageID: "123", ExpectedVersion: 3, Old: "2 CPU", New: "4 CPU",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "conflict" || result.Reason != "old text is not unique" {
		t.Fatalf("expected the ambiguous-span conflict, got %+v", result)
	}
	requireNoRequest(t, recorder)
}

// Confluence records a version for every update it accepts, including one that
// changes nothing, so a re-run of an edit that already landed must not write.
func TestUpdatePageDoesNotBumpTheVersionForAnEditAlreadyApplied(t *testing.T) {
	client, recorder := editTestServer(t, `{"id":"123"}`)
	current := &PageInfo{ID: "123", Type: "page", Title: "Capacity", Space: "VEGA", Version: 3,
		Body: "<p>4 CPU</p>"}

	result, err := client.UpdatePageContext(context.Background(), current, UpdateRequest{
		PageID: "123", ExpectedVersion: 3, Old: "2 CPU", New: "4 CPU",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || result.Changed {
		t.Fatalf("expected an unchanged ok, got %+v", result)
	}
	if result.Version != 3 {
		t.Errorf("version = %d, want the page left where it was", result.Version)
	}
	requireNoRequest(t, recorder)
}

func TestUpdatePageRejectsBodyAndSpanTogether(t *testing.T) {
	client, recorder := editTestServer(t, `{"id":"123"}`)
	current := &PageInfo{ID: "123", Type: "page", Title: "Capacity", Space: "VEGA", Version: 3, Body: "<p>old</p>"}

	_, err := client.UpdatePageContext(context.Background(), current, UpdateRequest{
		PageID: "123", ExpectedVersion: 3, Body: "<p>new</p>", Old: "old", New: "new",
	})
	if err == nil {
		t.Fatal("expected an error: body and old/new describe two different edits")
	}
	requireNoRequest(t, recorder)
}

func TestUpdatePageRequiresAnExpectedVersion(t *testing.T) {
	client, recorder := editTestServer(t, `{"id":"123"}`)
	current := &PageInfo{ID: "123", Type: "page", Title: "Capacity", Space: "VEGA", Version: 3, Body: "<p>old</p>"}

	_, err := client.UpdatePageContext(context.Background(), current, UpdateRequest{PageID: "123", Body: "<p>new</p>"})
	if err == nil {
		t.Fatal("expected an error: an unguarded update is the overwrite the guard exists to prevent")
	}
	requireNoRequest(t, recorder)
}

func TestCreatePagePostsSpaceTitleAndParent(t *testing.T) {
	client, recorder := editTestServer(t, `{"id":"456","version":{"number":1},"_links":{"webui":"/display/VEGA/New"}}`)

	result, err := client.CreatePageContext(context.Background(), CreateRequest{
		SpaceKey: "VEGA", Title: "New", Body: "<p>hello</p>", ParentID: "123",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.PageID != "456" || result.Version != 1 || result.URL != "/display/VEGA/New" {
		t.Fatalf("the created page's own id, version and link must come back: %+v", result)
	}

	sent := onlyRequest(t, recorder)
	if sent.method != http.MethodPost {
		t.Errorf("method = %s, want POST", sent.method)
	}
	if got := field(t, sent.body, "space", "key"); got != "VEGA" {
		t.Errorf("space.key = %v, want VEGA", got)
	}
	if got := field(t, sent.body, "type"); got != "page" {
		t.Errorf("type = %v, want page", got)
	}
	ancestors, ok := field(t, sent.body, "ancestors").([]any)
	if !ok || len(ancestors) != 1 {
		t.Fatalf("ancestors = %v, want the one parent", field(t, sent.body, "ancestors"))
	}
	if got := field(t, ancestors[0].(map[string]any), "id"); got != "123" {
		t.Errorf("ancestor id = %v, want 123", got)
	}
}

func TestCreatePageRequiresSpaceAndTitle(t *testing.T) {
	client, recorder := editTestServer(t, `{"id":"456"}`)

	if _, err := client.CreatePageContext(context.Background(), CreateRequest{Title: "New"}); err == nil {
		t.Error("expected an error without a space key")
	}
	if _, err := client.CreatePageContext(context.Background(), CreateRequest{SpaceKey: "VEGA"}); err == nil {
		t.Error("expected an error without a title")
	}
	requireNoRequest(t, recorder)
}

func TestDeletePageSendsDeleteOnlyAtTheExpectedVersion(t *testing.T) {
	client, recorder := editTestServer(t, `{}`)
	current := &PageInfo{ID: "123", Type: "page", Title: "Capacity", Space: "VEGA", Version: 3}

	result, err := client.DeletePageContext(context.Background(), current, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || !result.Changed {
		t.Fatalf("unexpected result: %+v", result)
	}
	if sent := onlyRequest(t, recorder); sent.method != http.MethodDelete {
		t.Errorf("method = %s, want DELETE", sent.method)
	}
}

func TestDeletePageRefusesAStaleVersionWithoutDeleting(t *testing.T) {
	client, recorder := editTestServer(t, `{}`)
	current := &PageInfo{ID: "123", Type: "page", Title: "Capacity", Space: "VEGA", Version: 9}

	result, err := client.DeletePageContext(context.Background(), current, 3)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "conflict" {
		t.Fatalf("expected a conflict, got %+v", result)
	}
	requireNoRequest(t, recorder)
}

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
