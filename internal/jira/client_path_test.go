package jira

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// Issue and property keys are caller-supplied strings interpolated into the
// request path. Unescaped, "../" or "?" in a property key retargets the
// request to a different REST path on the Jira host.
func TestSetIssuePropertyEscapesPathSegments(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	jc, err := NewClient(config.JiraConfig{URL: srv.URL, Username: "u", APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if err := jc.SetIssueProperty("PROJ-1", "../x?y", map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
	if want := "/rest/api/2/issue/PROJ-1/properties/..%2Fx%3Fy"; gotPath != want {
		t.Fatalf("escaped path = %q, want %q", gotPath, want)
	}
}
