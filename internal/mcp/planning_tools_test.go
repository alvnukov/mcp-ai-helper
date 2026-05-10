package mcp

import (
	"strings"
	"testing"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

func TestPlanTaskExecutionMediumTaskCanProceed(t *testing.T) {
	result := planTaskExecution([]tasks.Task{{
		ID:         "impl",
		Status:     "in_progress",
		Title:      "Implement thing",
		TaskType:   "type-implementation",
		ModelLevel: "medium",
		Priority:   "critical",
	}}, planTaskExecutionRequest{CurrentModelLevel: "medium"})

	if result.Mode != "weak_safe" || result.Action != "proceed" || result.PlanningOnly {
		t.Fatalf("result = %#v", result)
	}
	if result.RequiredLLMLevel != "medium" || result.ModelLevel != "medium" || result.RequiredTaskType != "type-implementation" {
		t.Fatalf("task routing = level %q model %q type %q", result.RequiredLLMLevel, result.ModelLevel, result.RequiredTaskType)
	}
	if len(result.AllowedDelegateLevels) != 3 || result.AllowedDelegateLevels[0] != "medium" {
		t.Fatalf("allowed delegate levels = %#v", result.AllowedDelegateLevels)
	}
	if len(result.RequiredPipeline) == 0 || result.RequiredPipeline[len(result.RequiredPipeline)-1] != "run_workflow" {
		t.Fatalf("pipeline = %#v", result.RequiredPipeline)
	}
}

func TestPlanTaskExecutionVeryHighDelegatesLowerLevelTask(t *testing.T) {
	result := planTaskExecution([]tasks.Task{{
		ID:         "impl",
		Status:     "in_progress",
		Title:      "Implement thing",
		TaskType:   "type-implementation",
		ModelLevel: "medium",
	}}, planTaskExecutionRequest{CurrentModelLevel: "very_high"})

	if result.Action != "delegate_required" || !result.PlanningOnly {
		t.Fatalf("very_high must plan/delegate lower-level tasks: %#v", result)
	}
	if result.SwitchReason == "" {
		t.Fatal("delegate reason is required")
	}
}

func TestPlanTaskExecutionHighTaskRequiresSwitchWhenUnknown(t *testing.T) {
	result := planTaskExecution([]tasks.Task{{
		ID:         "design",
		Status:     "todo",
		Title:      "Design thing",
		TaskType:   "type-design",
		ModelLevel: "high",
	}}, planTaskExecutionRequest{})

	if result.Mode != "high_required" || result.Action != "switch_model_required" || !result.PlanningOnly {
		t.Fatalf("result = %#v", result)
	}
	if result.SwitchReason == "" {
		t.Fatal("switch reason is required")
	}
}

func TestBuildTaskPacketIncludesStructuredExecutionScope(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:                 "impl",
		Title:              "Implement thing",
		Body:               "local implementation only",
		TaskType:           "type-implementation",
		ModelLevel:         "medium",
		Tags:               []string{"planning"},
		AcceptanceCriteria: []string{"criterion"},
		VerificationPlan:   []string{"go test ./internal/mcp"},
	}, "medium")

	if packet.Action != "proceed" || packet.Readiness != "ready" || packet.RequiredLLMLevel != "medium" || packet.ModelLevel != "medium" || !packet.ReadyForStandardModel {
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
	if len(packet.ReasoningPatterns) == 0 || packet.PatternGate == "" {
		t.Fatalf("packet missing reasoning pattern gates: %#v", packet)
	}
	assertPatternID(t, packet.ReasoningPatterns, "invariant_preservation.v1")
	assertPatternID(t, packet.ReasoningPatterns, "test_oracle_regression.v1")
}

func TestBuildTaskPacketCanDisableReasoningPatterns(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:         "impl",
		Title:      "Implement thing",
		TaskType:   "type-implementation",
		ModelLevel: "medium",
	}, "medium", false)

	if len(packet.ReasoningPatterns) != 0 || packet.PatternGate != "" {
		t.Fatalf("reasoning patterns should be disabled: %#v", packet)
	}
}

func TestReasoningPatternsCatalogIncludesCorePatterns(t *testing.T) {
	patterns := defaultReasoningPatterns()
	if len(patterns) < 14 {
		t.Fatalf("catalog too small: %#v", patterns)
	}
	assertPatternID(t, patterns, "precedence_fallback.v1")
	assertPatternID(t, patterns, "state_machine_transaction.v1")
	assertPatternID(t, patterns, "boundary_ownership.v1")
	assertPatternID(t, patterns, "invariant_preservation.v1")
	assertPatternID(t, patterns, "schema_contract_compatibility.v1")
	assertPatternID(t, patterns, "failure_semantics.v1")
	assertPatternID(t, patterns, "concurrency_ordering.v1")
	assertPatternID(t, patterns, "data_migration_projection.v1")
	assertPatternID(t, patterns, "parser_serializer_roundtrip.v1")
	assertPatternID(t, patterns, "security_policy_boundary.v1")
	assertPatternID(t, patterns, "evidence_grounding.v1")
	assertPatternID(t, patterns, "test_oracle_regression.v1")
	assertPatternID(t, patterns, "resource_budget_loop_control.v1")
	assertPatternID(t, patterns, "adapter_integration.v1")
}

func TestBuildTaskPacketSelectsPrecedencePattern(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:         "task-048",
		Title:      "run_workflow git_commit_owned ignores top-level owned_files",
		Body:       "Choose precedence between step args, top-level owned_files, and changedSet fallback. Do not let changedSet override explicit top-level owned_files.",
		TaskType:   "type-implementation",
		ModelLevel: "high",
		Tags:       []string{"workflow", "git"},
	}, "high")

	pattern := findPattern(t, packet.ReasoningPatterns, "precedence_fallback.v1")
	assertContainsText(t, pattern.RequiredArtifacts, "source precedence matrix")
	assertContainsText(t, pattern.ValidationGates, "fallback sources are after explicit sources")
	assertPatternID(t, packet.ReasoningPatterns, "boundary_ownership.v1")
	assertPatternID(t, packet.ReasoningPatterns, "failure_semantics.v1")
}

func TestBuildTaskPacketReturnsFullCatalogForPromptPatternDesign(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:         "task-050",
		Title:      "Спроектировать формат техзадания и паттерн-промпты",
		Body:       "Нужен полный список prompt-patterns и критерии spec-quality для младшей модели.",
		TaskType:   "type-design",
		ModelLevel: "very_high",
		Tags:       []string{"prompt-patterns", "spec-quality"},
	}, "very_high")

	if got, want := len(packet.ReasoningPatterns), len(defaultReasoningPatterns()); got != want {
		t.Fatalf("task-050 should expose full catalog: got %d want %d", got, want)
	}
}

func TestBuildTaskPacketPriorityDoesNotInferModelLevel(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:       "task-044",
		Title:    "Добавить task execution packet и readiness contract",
		Body:     "Packet must contain context, owned files, forbidden files, risks, gates, and readiness.",
		Priority: "critical",
		Tags:     []string{"tasks", "workflow", "planning", "llm-ergonomics"},
	}, "very_high")

	if packet.Action != "blocked" || packet.RequiredLLMLevel != "unknown" || packet.ModelLevel != "unknown" || packet.Readiness != "blocked" {
		t.Fatalf("priority must not imply model_level: %#v", packet)
	}
}

func TestBuildTaskPacketHighTaskRequiresSwitch(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:         "design",
		Title:      "Design thing",
		TaskType:   "type-design",
		ModelLevel: "high",
	}, "medium")

	if packet.Action != "switch_model_required" || packet.SwitchReason == "" || packet.Readiness != "requires_higher_model_level" {
		t.Fatalf("packet = %#v", packet)
	}
	if len(packet.AcceptanceCriteria) != 0 || len(packet.VerificationPlan) != 0 {
		t.Fatalf("higher-level packet should not expose execution instructions: %#v", packet)
	}
}

func TestBuildTaskPacketForLeanRegistryForbidsGoSideSourceMutation(t *testing.T) {
	packet := buildTaskPacket(tasks.Task{
		ID:         "task-056",
		Title:      "Закрепить запрет Go-side Lean registry parsing и mutation",
		Priority:   "high",
		ModelLevel: "high",
		Tags:       []string{"tasks", "lean-registry", "lake-server"},
	}, "high")

	if packet.Action != "proceed" {
		t.Fatalf("high model should be allowed to inspect hardening packet: %#v", packet)
	}
	assertContainsText(t, packet.ForbiddenShortcuts, "regex-mutate Lean registry source")
	assertContainsText(t, packet.ForbiddenFiles, "Go production code that parses or regex-mutates")
	assertContainsText(t, packet.KnownRisks, "not an allowed production fallback")
	assertContainsText(t, packet.RequiredGates, "lake build")
}

func TestPlanTaskExecutionBlocksWithoutCurrentTask(t *testing.T) {
	result := planTaskExecution(nil, planTaskExecutionRequest{CurrentModelLevel: "medium"})

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

func assertPatternID(t *testing.T, patterns []reasoningPattern, id string) {
	t.Helper()
	_ = findPattern(t, patterns, id)
}

func findPattern(t *testing.T, patterns []reasoningPattern, id string) reasoningPattern {
	t.Helper()
	for _, pattern := range patterns {
		if pattern.ID == id {
			return pattern
		}
	}
	t.Fatalf("pattern %q not found in %#v", id, patterns)
	return reasoningPattern{}
}
