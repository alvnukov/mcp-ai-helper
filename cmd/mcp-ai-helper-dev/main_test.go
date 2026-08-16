package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestResolveGoBinaryWithEmptyPath(t *testing.T) {
	t.Setenv("PATH", "")
	goBinary := resolveGoBinary(t.TempDir())
	if !filepath.IsAbs(goBinary) {
		t.Fatalf("resolveGoBinary returned %q, want an absolute path with empty PATH", goBinary)
	}
	if _, err := os.Stat(goBinary); err != nil {
		t.Fatalf("resolved Go binary %q is not accessible: %v", goBinary, err)
	}
	if output, err := exec.Command(goBinary, "version").CombinedOutput(); err != nil {
		t.Fatalf("resolved Go binary failed: %v: %s", err, output)
	}
}

func TestChildManagerBuildWithEmptyPath(t *testing.T) {
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	binaryPath := filepath.Join(t.TempDir(), "mcp-ai-helper")
	manager := childManager{repoRoot: repoRoot, binaryPath: binaryPath}

	t.Setenv("PATH", "")
	if err := manager.build(); err != nil {
		t.Fatalf("build with empty PATH: %v", err)
	}
	if info, err := os.Stat(binaryPath); err != nil || info.IsDir() {
		t.Fatalf("built child binary is unavailable: info=%v err=%v", info, err)
	}
}
