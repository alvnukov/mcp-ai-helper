package confluence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// Without an explicit client timeout a hung Confluence host pins the tool
// call until whatever outer context expires — possibly never.
func TestConfluenceClientEnforcesHTTPTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		<-release
	}))
	defer srv.Close()
	defer close(release)

	orig := defaultHTTPTimeout
	defaultHTTPTimeout = 100 * time.Millisecond
	defer func() { defaultHTTPTimeout = orig }()

	c, err := NewClient(Config{URL: srv.URL, APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	if _, err := c.GetSpacesContext(context.Background()); err == nil {
		t.Fatal("hung server should time out")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("timeout took %s; the client has no effective timeout", elapsed)
	}
}
