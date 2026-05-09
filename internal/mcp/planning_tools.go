package mcp

import (
	"context"
	"strings"

	basemcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

type planTaskExecutionRequest struct {
	RepoPath          string `json:"repo_path"`
	Task              string `json:"task"`
	CurrentModelLevel string `json:"current_model_level"`
}

type taskPlanSummary struct {
	ID                 string   `json:"id"`
	Title              string   `json:"title"`
	Status             string   `json:"status"`
	Priority           string   `json:"priority,omitempty"`
	TaskType           string   `json:"task_type,omitempty"`
	RequiredLLMLevel   string   `json:"required_llm_level,omitempty"`
	AcceptanceCriteria []string `json:"acceptance_criteria,omitempty"`
	VerificationPlan   []string `json:"verification_plan,omitempty"`
}

type taskPacketRequest struct {
	RepoPath          string `json:"repo_path"`
	ID                string `json:"id"`
	CurrentModelLevel string `json:"current_model_level"`
}

type taskPacketResult struct {
	TaskID                 string   `json:"task_id"`
	Action                 string   `json:"action"`
	Readiness              string   `json:"readiness"`
	ReadyForStandardModel  bool     `json:"ready_for_standard_model"`
	NeedsStrongModel       bool     `json:"needs_strong_model"`
	RequiredTaskType       string   `json:"required_task_type,omitempty"`
	RequiredLLMLevel       string   `json:"required_llm_level,omitempty"`
	AllowedModelLevels     []string `json:"allowed_model_levels,omitempty"`
	Objective              string   `json:"objective,omitempty"`
	Body                   string   `json:"body,omitempty"`
	MinimalRequiredContext []string `json:"minimal_required_context,omitempty"`
	OwnedFiles             []string `json:"owned_files,omitempty"`
	ForbiddenFiles         []string `json:"forbidden_files,omitempty"`
	KnownRisks             []string `json:"known_risks,omitempty"`
	RequiredGates          []string `json:"required_gates,omitempty"`
	AcceptanceCriteria     []string `json:"acceptance_criteria,omitempty"`
	VerificationPlan       []string `json:"verification_plan,omitempty"`
	ForbiddenShortcuts     []string `json:"forbidden_shortcuts,omitempty"`
	ExpectedOutput         []string `json:"expected_output,omitempty"`
	StatusUpdate           string   `json:"status_update,omitempty"`
	SwitchReason           string   `json:"switch_reason,omitempty"`
}

type planTaskExecutionResult struct {
	Mode                      string            `json:"mode"`
	Action                    string            `json:"action"`
	RequiredPipeline          []string          `json:"required_pipeline"`
	Goal                      *taskPlanSummary  `json:"goal,omitempty"`
	CurrentTask               *taskPlanSummary  `json:"current_task,omitempty"`
	ReadyTasks                []taskPlanSummary `json:"ready_tasks,omitempty"`
	BlockedTasks              []taskPlanSummary `json:"blocked_tasks,omitempty"`
	SuggestedTaskType         string            `json:"suggested_task_type,omitempty"`
	RequiredTaskType          string            `json:"required_task_type,omitempty"`
	RequiredLLMLevel          string            `json:"required_llm_level,omitempty"`
	CurrentModelLevel         string            `json:"current_model_level"`
	AllowedDelegateLevels     []string          `json:"allowed_delegate_levels,omitempty"`
	SwitchReason              string            `json:"switch_reason,omitempty"`
	PlanningOnly              bool              `json:"planning_only"`
	MinimalNextDataCollection []string          `json:"minimal_next_data_collection,omitempty"`
}

func registerPlanningTools(srv *server.MCPServer, deps *Server) {
	srv.AddTool(basemcp.NewTool("task_packet",
		basemcp.WithDescription("Return a compact task packet for junior-model execution."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("id", basemcp.Required()),
		basemcp.WithString("current_model_level"),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args taskPacketRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		task, err := store.Get(tasks.GetRequest{RepoPath: args.RepoPath, ID: args.ID})
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(buildTaskPacket(task, args.CurrentModelLevel))
	})

	srv.AddTool(basemcp.NewTool("plan_task_execution",
		basemcp.WithDescription("Return compact planning gate for the next repository task."),
		basemcp.WithString("repo_path", basemcp.Required()),
		basemcp.WithString("task"),
		basemcp.WithString("current_model_level"),
	), func(_ context.Context, req basemcp.CallToolRequest) (*basemcp.CallToolResult, error) {
		var args planTaskExecutionRequest
		if err := bind(req, &args); err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		_, _, _, _, store := deps.loadDeps()
		list, err := store.List(tasks.ListRequest{RepoPath: args.RepoPath})
		if err != nil {
			return basemcp.NewToolResultError(err.Error()), nil
		}
		return structured(planTaskExecution(list, args))
	})
}

func buildTaskPacket(task tasks.Task, currentModelLevel string) taskPacketResult {
	taskType, level := taskTypeAndLevel(task)
	if strings.TrimSpace(currentModelLevel) == "" {
		currentModelLevel = "unknown"
	}
	action := actionForLevel(level, currentModelLevel)
	forbiddenShortcuts := []string{
		"do not change unrelated roadmap or parent planning policy",
		"do not mark design/research tasks done without strong-level review",
		"do not weaken tests or checks to get a green result",
	}
	if hasAnyTag(task.Tags, "tasks", "lean-registry", "lake-server") {
		forbiddenShortcuts = append(forbiddenShortcuts, leanRegistryGoSourceMutationBan())
	}
	packet := taskPacketResult{
		TaskID:                 task.ID,
		Action:                 action,
		Readiness:              readinessForAction(action),
		ReadyForStandardModel:  actionForLevel(level, "standard") == "proceed",
		NeedsStrongModel:       level == "strong",
		RequiredTaskType:       taskType,
		RequiredLLMLevel:       level,
		AllowedModelLevels:     allowedDelegateLevels(level),
		Objective:              task.Title,
		Body:                   task.Body,
		MinimalRequiredContext: minimalContextForTask(task),
		OwnedFiles:             ownedFilesForTask(task),
		ForbiddenFiles:         forbiddenFilesForTask(task),
		KnownRisks:             risksForTask(task),
		RequiredGates:          requiredGatesForTask(task),
		ForbiddenShortcuts:     forbiddenShortcuts,
		ExpectedOutput: []string{
			"changed files or explicit no-code result",
			"minimal verification evidence",
			"task status update",
		},
		StatusUpdate: "set task to done only after verification passes; otherwise set blocked with reason",
	}
	if packet.Action == "switch_model_required" {
		packet.SwitchReason = "task requires a stronger model level"
		return packet
	}
	packet.AcceptanceCriteria = task.AcceptanceCriteria
	packet.VerificationPlan = task.VerificationPlan
	return packet
}

func planTaskExecution(list []tasks.Task, req planTaskExecutionRequest) planTaskExecutionResult {
	currentModelLevel := strings.TrimSpace(req.CurrentModelLevel)
	if currentModelLevel == "" {
		currentModelLevel = "unknown"
	}

	var goal *taskPlanSummary
	var current *taskPlanSummary
	ready := make([]taskPlanSummary, 0)
	blocked := make([]taskPlanSummary, 0)
	for _, task := range currentTasks(list) {
		summary := summarizeTaskForPlan(task)
		if hasTag(task.Tags, "goal") && task.ParentID == "" {
			item := summary
			goal = &item
			continue
		}
		switch task.Status {
		case "in_progress":
			if current == nil {
				item := summary
				current = &item
			}
		case "todo":
			ready = append(ready, summary)
		case "blocked":
			blocked = append(blocked, summary)
		}
	}
	if current == nil && len(ready) > 0 {
		item := ready[0]
		current = &item
	}

	result := planTaskExecutionResult{
		Mode:              "blocked",
		Action:            "blocked",
		RequiredPipeline:  []string{"task_current"},
		Goal:              goal,
		CurrentTask:       current,
		ReadyTasks:        ready,
		BlockedTasks:      blocked,
		CurrentModelLevel: currentModelLevel,
		PlanningOnly:      true,
	}
	if current == nil {
		result.MinimalNextDataCollection = []string{"task_current", "task_tree"}
		return result
	}

	result.SuggestedTaskType = current.TaskType
	result.RequiredTaskType = current.TaskType
	result.RequiredLLMLevel = current.RequiredLLMLevel
	result.AllowedDelegateLevels = allowedDelegateLevels(current.RequiredLLMLevel)
	result.RequiredPipeline = pipelineForTask(current)
	result.Mode = modeForLevel(current.RequiredLLMLevel)
	result.Action = actionForLevel(current.RequiredLLMLevel, currentModelLevel)
	result.PlanningOnly = result.Action != "proceed"
	if result.Action == "switch_model_required" {
		result.SwitchReason = "current model level is below required task level"
	}
	return result
}

func summarizeTaskForPlan(task tasks.Task) taskPlanSummary {
	taskType, level := taskTypeAndLevel(task)
	return taskPlanSummary{
		ID:                 task.ID,
		Title:              task.Title,
		Status:             task.Status,
		Priority:           task.Priority,
		TaskType:           taskType,
		RequiredLLMLevel:   level,
		AcceptanceCriteria: task.AcceptanceCriteria,
		VerificationPlan:   task.VerificationPlan,
	}
}

func readinessForAction(action string) string {
	switch action {
	case "proceed":
		return "ready"
	case "switch_model_required":
		return "requires_strong_model"
	default:
		return "blocked"
	}
}

func minimalContextForTask(task tasks.Task) []string {
	context := []string{"task_current", "task_get " + task.ID}
	for _, file := range ownedFilesForTask(task) {
		if file == "MCPAIHelperProject/ActiveTasks.lean" {
			continue
		}
		context = append(context, "read_file "+file, "snapshot_file "+file)
	}
	return uniqueStrings(context)
}

func forbiddenFilesForTask(task tasks.Task) []string {
	files := []string{
		"legacy task projection files",
		"unrelated roadmap or guidance files",
		"MCPAIHelperProject/ActiveTasks.lean direct edits; use task_transition/task tools only",
	}
	if hasAnyTag(task.Tags, "tasks", "lean-registry", "lake-server") {
		files = append(files, "Go production code that parses or regex-mutates MCPAIHelperProject/ActiveTasks.lean")
	}
	return files
}

func risksForTask(task tasks.Task) []string {
	risks := []string{"premature done status without required gates and owned-files commit"}
	if hasAnyTag(task.Tags, "workflow", "tasks", "git", "fileops") {
		risks = append(risks, "shared workflow semantics can affect unrelated repo tasks")
	}
	if hasAnyTag(task.Tags, "lean-registry", "tasks") {
		risks = append(risks, "Lean registry mutation must validate with lake build", "Go-side Lean registry source parsing/mutation is not an allowed production fallback")
	}
	if hasAnyTag(task.Tags, "logs", "output", "filtering") {
		risks = append(risks, "large command output must stay compact and evidence-linked")
	}
	return uniqueStrings(risks)
}

func requiredGatesForTask(task tasks.Task) []string {
	if len(task.VerificationPlan) > 0 {
		return task.VerificationPlan
	}
	gates := []string{"gofmt on changed Go files"}
	switch {
	case hasAnyTag(task.Tags, "planning"):
		gates = append(gates, "go test ./internal/mcp")
	case hasAnyTag(task.Tags, "lean-registry", "lake-server", "tasks"):
		gates = append(gates, "go test ./internal/mcp", "lake build")
	case hasAnyTag(task.Tags, "workflow"):
		gates = append(gates, "go test ./internal/pipeline ./internal/mcp")
	case hasAnyTag(task.Tags, "fileops"):
		gates = append(gates, "go test ./internal/fileops ./internal/pipeline")
	case hasAnyTag(task.Tags, "git"):
		gates = append(gates, "go test ./internal/gitops ./internal/pipeline")
	case hasAnyTag(task.Tags, "logs", "output", "filtering"):
		gates = append(gates, "go test ./internal/command")
	case hasAnyTag(task.Tags, "models", "providers", "routing"):
		gates = append(gates, "go test ./internal/provider ./internal/mcp")
	case hasAnyTag(task.Tags, "security", "commands", "policy"):
		gates = append(gates, "go test ./internal/command ./internal/config")
	default:
		gates = append(gates, "targeted go test for affected packages")
	}
	return uniqueStrings(gates)
}

func leanRegistryGoSourceMutationBan() string {
	return "do not parse or regex-mutate Lean registry source in Go production paths; use Lean-owned lake serve/exporter/task tools"
}

func ownedFilesForTask(task tasks.Task) []string {
	files := []string{"MCPAIHelperProject/ActiveTasks.lean"}
	if hasAnyTag(task.Tags, "planning") {
		files = append(files, "internal/mcp/planning_tools.go", "internal/mcp/planning_tools_test.go")
	}
	if hasAnyTag(task.Tags, "workflow") {
		files = append(files, "internal/pipeline/pipeline.go", "internal/pipeline/pipeline_test.go", "internal/mcp/pipeline_tools.go")
	}
	if hasAnyTag(task.Tags, "fileops") {
		files = append(files, "internal/fileops/safe_edit.go", "internal/fileops/safe_edit_test.go")
	}
	if hasAnyTag(task.Tags, "git") {
		files = append(files, "internal/gitops/git.go", "internal/gitops/git_test.go")
	}
	if hasAnyTag(task.Tags, "tasks", "lean-registry") {
		files = append(files, "internal/mcp/task_tools.go", "internal/mcp/task_lean_read.go", "internal/mcp/task_lean_mutation.go")
	}
	if hasAnyTag(task.Tags, "logs", "output", "filtering") {
		files = append(files, "internal/command/runner.go", "internal/command/history.go", "internal/command/runner_test.go")
	}
	if hasAnyTag(task.Tags, "models", "providers", "routing") {
		files = append(files, "internal/provider/openai.go", "internal/mcp/model_tools.go", "internal/config/config.go")
	}
	if hasAnyTag(task.Tags, "security", "commands", "policy") {
		files = append(files, "internal/command/runner.go", "internal/config/config.go")
	}
	return uniqueStrings(files)
}

func hasAnyTag(tags []string, targets ...string) bool {
	for _, tag := range tags {
		for _, target := range targets {
			if tag == target {
				return true
			}
		}
	}
	return false
}

func taskTypeAndLevel(task tasks.Task) (string, string) {
	for _, tag := range task.Tags {
		if strings.HasPrefix(tag, "type-") {
			return tag, llmLevelFromTags(task.Tags)
		}
	}
	start := strings.Index(task.Title, "[")
	end := strings.Index(task.Title, "]")
	if start >= 0 && end > start {
		candidate := task.Title[start+1 : end]
		if strings.HasPrefix(candidate, "type-") {
			return candidate, llmLevelFromTaskType(candidate)
		}
	}
	return inferredTaskTypeAndLevel(task)
}

func inferredTaskTypeAndLevel(task tasks.Task) (string, string) {
	switch {
	case task.Priority == "critical" || hasAnyTag(task.Tags, "fileops", "git", "security", "routing", "models", "providers", "lean-registry", "evidence", "quality"):
		return "type-implementation-strong", "strong"
	case hasAnyTag(task.Tags, "workflow") && task.Priority != "medium":
		return "type-implementation-strong", "strong"
	case hasAnyTag(task.Tags, "logs", "output", "filtering", "observability", "audit", "testing", "planning"):
		return "type-implementation-standard", "standard"
	default:
		return "", "unknown"
	}
}

func llmLevelFromTags(tags []string) string {
	for _, tag := range tags {
		switch tag {
		case "llm-strong":
			return "strong"
		case "llm-standard":
			return "standard"
		case "llm-cheap":
			return "cheap"
		}
	}
	return "unknown"
}

func llmLevelFromTaskType(taskType string) string {
	if strings.HasSuffix(taskType, "-strong") {
		return "strong"
	}
	if strings.HasSuffix(taskType, "-standard") {
		return "standard"
	}
	if strings.HasSuffix(taskType, "-cheap") {
		return "cheap"
	}
	return "unknown"
}

func pipelineForTask(task *taskPlanSummary) []string {
	pipeline := []string{"task_current"}
	if strings.Contains(task.TaskType, "implementation") || strings.Contains(task.TaskType, "test") {
		pipeline = append(pipeline, "read_file", "snapshot_file", "run_workflow")
	}
	if strings.Contains(task.TaskType, "docs") {
		pipeline = append(pipeline, "read_file", "snapshot_file", "run_workflow")
	}
	if len(pipeline) == 1 {
		pipeline = append(pipeline, "read_file")
	}
	return pipeline
}

func modeForLevel(level string) string {
	if level == "strong" {
		return "strong_required"
	}
	if level == "cheap" || level == "standard" {
		return "weak_safe"
	}
	return "blocked"
}

func allowedDelegateLevels(required string) []string {
	switch required {
	case "strong":
		return []string{"strong"}
	case "standard":
		return []string{"standard", "strong"}
	case "cheap":
		return []string{"cheap", "standard", "strong"}
	default:
		return nil
	}
}

func actionForLevel(required string, current string) string {
	if required == "unknown" || required == "" {
		return "blocked"
	}
	if required == "strong" && current != "strong" {
		return "switch_model_required"
	}
	if required == "standard" && current == "cheap" {
		return "switch_model_required"
	}
	return "proceed"
}
