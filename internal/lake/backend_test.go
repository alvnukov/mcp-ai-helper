package lake

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

type fakeRunner struct {
	result CommandResult
	dir    string
	args   []string
}

func (f *fakeRunner) Run(_ context.Context, dir string, args []string) (CommandResult, error) {
	f.dir = dir
	f.args = append([]string(nil), args...)
	r := f.result
	r.WorkspaceDetected = true
	r.WorkspaceDir = dir
	r.Command = append([]string(nil), args...)
	return r, nil
}

func TestResolveWorkspaceDetectsLakeProject(t *testing.T) {
	ws, err := ResolveWorkspace("testdata/valid")
	if err != nil {
		t.Fatalf("ResolveWorkspace returned error: %v", err)
	}
	if ws.Dir == "" {
		t.Fatal("workspace dir is empty")
	}
}

func TestResolveWorkspaceMissingLakeProjectReturnsBlocker(t *testing.T) {
	_, err := ResolveWorkspace("testdata/missing")
	if err == nil {
		t.Fatal("expected missing workspace blocker")
	}
	if !strings.Contains(err.Error(), "missing lean-toolchain") {
		t.Fatalf("unexpected blocker: %v", err)
	}
}

func TestBuildUsesLakeBuild(t *testing.T) {
	runner := &fakeRunner{}
	got, err := Build(context.Background(), "testdata/valid", runner)
	if err != nil {
		t.Fatalf("Build returned error: %v", err)
	}
	if !got.WorkspaceDetected || got.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v", got)
	}
	if !reflect.DeepEqual(runner.args, []string{"lake", "build"}) {
		t.Fatalf("unexpected args: %#v", runner.args)
	}
}

func TestCheckFileUsesLakeEnvLean(t *testing.T) {
	runner := &fakeRunner{}
	got, err := CheckFile(context.Background(), "testdata/valid", "Valid.lean", runner)
	if err != nil {
		t.Fatalf("CheckFile returned error: %v", err)
	}
	if !got.WorkspaceDetected || got.ExitCode != 0 {
		t.Fatalf("unexpected result: %+v", got)
	}
	want := []string{"lake", "env", "lean", "Valid.lean"}
	if !reflect.DeepEqual(runner.args, want) {
		t.Fatalf("unexpected args: got %#v want %#v", runner.args, want)
	}
}

func TestCheckFileInvalidLeanDiagnosticsAreFiltered(t *testing.T) {
	runner := &fakeRunner{result: CommandResult{ExitCode: 1, Diagnostics: FilterDiagnostics("Valid.lean:1:0: warning: ok\nInvalid.lean:2:7: error: unexpected token\ntrace noise\n")}}
	got, err := CheckFile(context.Background(), "testdata/valid", "Invalid.lean", runner)
	if err != nil {
		t.Fatalf("CheckFile returned error: %v", err)
	}
	if got.ExitCode != 1 {
		t.Fatalf("expected failing check, got %+v", got)
	}
	if len(got.Diagnostics) != 2 || !strings.Contains(got.Diagnostics[1], "error") {
		t.Fatalf("unexpected diagnostics: %#v", got.Diagnostics)
	}
}

func TestCheckFileRejectsPathEscape(t *testing.T) {
	got, err := CheckFile(context.Background(), "testdata/valid", "../Invalid.lean", &fakeRunner{})
	if err != nil {
		t.Fatalf("CheckFile returned error: %v", err)
	}
	if got.Blocker == "" {
		t.Fatalf("expected blocker, got %+v", got)
	}
}

func TestShellQuoteKeepsLeanCommandsReadable(t *testing.T) {
	got := shellQuote([]string{"lake", "env", "lean", "Foo Bar.lean"})
	if got != "lake env lean 'Foo Bar.lean'" {
		t.Fatalf("quote = %q", got)
	}
}
