package mcp

import (
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func TestPlanTaskExecutionStandardTaskCanProceed(t *testing.T) {
	result := planTaskExecution([]tasks.Task{{
		ID:       "impl",
		Status:   "in_progress",
		Title:    "[type-implementation-standard] Implement thing",
		Tags:     []string{"type-implementation-standard", "llm-standard"},
		Priority: "high",
	}}, planTaskExecutionRequest{CurrentModelLevel: "standard"})

	if result.Mode != "weak_safe" || result.Action != "proceed" || result.PlanningOnly {
		t.Fatalf("result = %#v", result)
	}
	if result.RequiredLLMLevel != "standard" || result.RequiredTaskType != "type-implementation-standard" {
		t.Fatalf("task routing = level %q type %q", result.RequiredLLMLevel, result.RequiredTaskType)
	}
	if len(result.AllowedDelegateLevels) != 2 || result.AllowedDelegateLevels[0] != "standard" {
		t.Fatalf("allowed delegate levels = %#v", result.AllowedDelegateLevels)
	}
	if len(result.RequiredPipeline) == 0 || result.RequiredPipeline[len(result.RequiredPipeline)-1] != "run_workflow" {
		t.Fatalf("pipeline = %#v", result.RequiredPipeline)
	}
}

func TestPlanTaskExecutionStrongTaskRequiresSwitchWhenUnknown(t *testing.T) {
	result := planTaskExecution([]tasks.Task{{
		ID:     "design",
		Status: "todo",
		Title:  "[type-design-strong] Design thing",
		Tags:   []string{"type-design-strong", "llm-strong"},
	}}, planTaskExecutionRequest{})

	if result.Mode != "strong_required" || result.Action != "switch_model_required" || !result.PlanningOnly {
		t.Fatalf("result = %#v", result)
	}
	if result.SwitchReason == "" {
		t.Fatal("switch reason is required")
	}
}

func TestBuildTaskPacketIncludesStructuredExecutionScope(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:                 "impl",
		Title:              "[type-implementation-standard] Implement thing",
		Body:               "local implementation only",
		Tags:               []string{"type-implementation-standard", "llm-standard", "planning"},
		AcceptanceCriteria: []string{"criterion"},
		VerificationPlan:   []string{"go test ./internal/mcp"},
	}, "standard")

	if packet.Action != "proceed" || packet.Readiness != "ready" || packet.RequiredLLMLevel != "standard" || !packet.ReadyForStandardModel {
		t.Fatalf("packet = %#v", packet)
	}
	if len(packet.AcceptanceCriteria) != 1 || len(packet.VerificationPlan) != 1 {
		t.Fatalf("packet missing structured fields: %#v", packet)
	}
	if len(packet.MinimalRequiredContext) == 0 || len(packet.OwnedFiles) == 0 || len(packet.ForbiddenFiles) == 0 || len(packet.KnownRisks) == 0 || len(packet.RequiredGates) == 0 {
		t.Fatalf("packet missing execution contract: %#v", packet)
	}
	if len(packet.ForbiddenShortcuts) == 0 || len(packet.ExpectedOutput) == 0 {
		t.Fatalf("packet missing delegation guardrails: %#v", packet)
	}
}

func TestBuildTaskPacketInfersCriticalTaskNeedsStrongModel(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:       "task-044",
		Title:    "Добавить task execution packet и readiness contract",
		Body:     "Packet must contain context, owned files, forbidden files, risks, gates, and readiness.",
		Priority: "critical",
		Tags:     []string{"tasks", "workflow", "planning", "llm-ergonomics"},
	}, "standard")

	if packet.Action != "switch_model_required" || packet.Readiness != "requires_strong_model" || !packet.NeedsStrongModel || packet.ReadyForStandardModel {
		t.Fatalf("packet routing = %#v", packet)
	}
	for _, field := range [][]string{packet.MinimalRequiredContext, packet.OwnedFiles, packet.ForbiddenFiles, packet.KnownRisks, packet.RequiredGates} {
		if len(field) == 0 {
			t.Fatalf("packet missing readiness field: %#v", packet)
		}
	}
}

func TestBuildTaskPacketStrongTaskRequiresSwitch(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:    "design",
		Title: "[type-design-strong] Design thing",
		Tags:  []string{"type-design-strong", "llm-strong"},
	}, "standard")

	if packet.Action != "switch_model_required" || packet.SwitchReason == "" {
		t.Fatalf("packet = %#v", packet)
	}
	if len(packet.AcceptanceCriteria) != 0 || len(packet.VerificationPlan) != 0 {
		t.Fatalf("strong packet should not expose execution instructions: %#v", packet)
	}
}

func TestBuildTaskPacketForLeanRegistryForbidsGoSideSourceMutation(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:       "task-056",
		Title:    "Закрепить запрет Go-side Lean registry parsing и mutation",
		Priority: "high",
		Tags:     []string{"tasks", "lean-registry", "lake-server", "llm-strong"},
	}, "strong")

	if packet.Action != "proceed" {
		t.Fatalf("strong model should be allowed to inspect hardening packet: %#v", packet)
	}
	assertContainsText(t, packet.ForbiddenShortcuts, "regex-mutate Lean registry source")
	assertContainsText(t, packet.ForbiddenFiles, "Go production code that parses or regex-mutates")
	assertContainsText(t, packet.KnownRisks, "not an allowed production fallback")
	assertContainsText(t, packet.RequiredGates, "lake build")
}

func TestPlanTaskExecutionBlocksWithoutCurrentTask(t *testing.T) {
	result := planTaskExecution(nil, planTaskExecutionRequest{CurrentModelLevel: "standard"})

	if result.Mode != "blocked" || result.Action != "blocked" || !result.PlanningOnly {
		t.Fatalf("result = %#v", result)
	}
	if len(result.MinimalNextDataCollection) == 0 {
		t.Fatal("expected minimal data collection hints")
	}
}

func assertContainsText(t *testing.T, values []string, needle string) {
	t.Helper()
	for _, value := range values {
		if strings.Contains(value, needle) {
			return
		}
	}
	t.Fatalf("%q not found in %#v", needle, values)
}
