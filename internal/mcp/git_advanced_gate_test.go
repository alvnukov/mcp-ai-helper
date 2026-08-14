package mcp

import (
	"context"
	"sync"
	"testing"

	basemcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/alvnukov/mcp-ai-helper/internal/config"
)

// The git_advanced gate is the one layer check that runs at call time, which
// puts it beside config_reload swapping s.cfg under s.mu. Reading that pointer
// without the mutex is a data race even when both sides behave otherwise, and
// one unsynchronized read after one locked write is enough for the race
// detector to hear it.
func TestGitAdvancedGateMustNotRaceConfigReload(t *testing.T) {
	deps := &Server{cfg: &config.Config{}}
	handler := gitAdvancedAction("log", deps)
	var req basemcp.CallToolRequest
	req.Params.Name = "git"

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			deps.mu.Lock()
			deps.cfg = &config.Config{}
			deps.mu.Unlock()
		}
	}()
	for i := 0; i < 20; i++ {
		if _, err := handler(context.Background(), req); err != nil {
			t.Fatalf("handler error: %v", err)
		}
	}
	wg.Wait()
}
