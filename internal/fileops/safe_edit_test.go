package fileops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestApplyGuardedReplaceRejectsHashMismatch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(path, []byte("one\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	result, err := ApplyGuardedReplace(ReplaceRequest{Path: path, ExpectedHash: "bad", Old: "one", New: "two"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "conflict" {
		t.Fatalf("status = %q, want conflict", result.Status)
	}
	// #nosec G304 -- test reads a file created inside t.TempDir.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "one\n" {
		t.Fatalf("file was modified: %q", string(data))
	}
}

func TestApplyGuardedReplaceIsIdempotentWhenDesiredTextPresent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.txt")
	if err := os.WriteFile(path, []byte("two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	snapshot, err := ReadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	result, err := ApplyGuardedReplace(ReplaceRequest{Path: path, ExpectedHash: snapshot.Hash, Old: "one", New: "two"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "ok" || result.Changed {
		t.Fatalf("result = %+v, want ok unchanged", result)
	}
}

func TestReadSnapshotInRepoRejectsEscapingPath(t *testing.T) {
	_, err := ReadSnapshotInRepo(t.TempDir(), "../x.txt")
	if err == nil {
		t.Fatal("expected repo escape error")
	}
}
