package lake

import "testing"

func TestResolveWorkspaceDetectsRepositoryRoot(t *testing.T) {
	ws, err := ResolveWorkspace("testdata/valid")
	if err != nil {
		t.Fatalf("ResolveWorkspace returned error: %v", err)
	}
	if ws.Dir == "" {
		t.Fatal("workspace dir is empty")
	}
}
