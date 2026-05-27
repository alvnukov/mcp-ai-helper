package pipeline

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/config"
	"github.com/zol/mcp-ai-helper/internal/webfetch"
)

func TestWorkflowCommandConsumesWebDocArtifact(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		_, _ = w.Write([]byte("artifact bytes"))
	}))
	defer srv.Close()
	repo := t.TempDir()
	cfg := &config.Config{
		CommandPolicy: config.CommandPolicy{AllowedCWDs: []string{repo}, DefaultTimeoutSeconds: 2, MaxOutputBytes: 1000, MaxLines: 20},
		WebPolicy:     config.WebPolicy{CacheDir: t.TempDir(), MaxSourceBytes: 1024, TimeoutSeconds: 2, MaxRedirects: 3, AllowedSchemes: []string{"http"}, AllowedHosts: []string{"127.0.0.1"}, AcceptedContentTypes: []string{"text/plain"}},
	}
	fetchResult, err := webfetch.NewClient(cfg.WebPolicy).Fetch(context.Background(), webfetch.FetchRequest{URL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	runner := NewRunner(cfg, nil)
	result, err := runner.RunWorkflow(context.Background(), WorkflowRequest{RepoPath: repo, Steps: []WorkflowStep{{ID: "wc", Tool: "command", Args: map[string]any{"command": "wc -c < \"$HELPER_WEB_DOC_PATH\"", "web_doc_id": fetchResult.DocID}}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || len(result.CheckResults) != 1 || result.CheckResults[0].ExitCode != 0 {
		t.Fatalf("result = %#v", result)
	}
}
