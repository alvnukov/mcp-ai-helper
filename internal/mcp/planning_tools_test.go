package mcp

import (
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
		Tags:               []string{"type-implementation-standard", "llm-standard"},
		AcceptanceCriteria: []string{"criterion"},
		VerificationPlan:   []string{"go test ./internal/mcp"},
	}, "standard")

	if packet.Action != "proceed" || packet.RequiredLLMLevel != "standard" {
		t.Fatalf("packet = %#v", packet)
	}
	if len(packet.AcceptanceCriteria) != 1 || len(packet.VerificationPlan) != 1 {
		t.Fatalf("packet missing structured fields: %#v", packet)
	}
	if len(packet.ForbiddenShortcuts) == 0 || len(packet.ExpectedOutput) == 0 {
		t.Fatalf("packet missing delegation guardrails: %#v", packet)
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

func TestPlanTaskExecutionBlocksWithoutCurrentTask(t *testing.T) {
	result := planTaskExecution(nil, planTaskExecutionRequest{CurrentModelLevel: "standard"})

	if result.Mode != "blocked" || result.Action != "blocked" || !result.PlanningOnly {
		t.Fatalf("result = %#v", result)
	}
	if len(result.MinimalNextDataCollection) == 0 {
		t.Fatal("expected minimal data collection hints")
	}
}
