package lake

import "testing"

func TestResolveWorkspaceDetectsRepositoryRoot(t *testing.T) {
	ws, err := ResolveWorkspace("../..")
	if err != nil {
		t.Fatalf("ResolveWorkspace repository root returned error: %v", err)
	}
	if ws.Dir == "" {
		t.Fatal("workspace dir is empty")
	}
}
