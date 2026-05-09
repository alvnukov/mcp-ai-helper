/- mcp-ai-helper-task {"id":"task-026","status":"done","title":"Update task status from every pipeline run","body":"Done in commit 804e107. Pipeline and workflow requests now accept current_task_id plus task_on_start/task_on_success/task_on_failure; execution updates the current task through the Runner task store in the same call.","priority":"critical","tags":["pipeline","tasks","workflow","status"],"created_at":"2026-05-08T13:02:35.69245Z","updated_at":"2026-05-08T13:14:32.318436Z"} -/
namespace MCPAIHelper.Tasks

def task_026_id : String := "task-026"
def task_026_status : String := "done"
def task_026_title : String := "Update task status from every pipeline run"
def task_026_body : String := "Done in commit 804e107. Pipeline and workflow requests now accept current_task_id plus task_on_start/task_on_success/task_on_failure; execution updates the current task through the Runner task store in the same call."
def task_026_priority : String := "critical"

end MCPAIHelper.Tasks
