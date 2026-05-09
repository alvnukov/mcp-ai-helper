/- mcp-ai-helper-task {"id":"task-012","status":"todo","title":"Add declarative conditional workflow execution","body":"Implement if/then/else logic in workflows: branch on command exit code, file state, test result, grep match, task state or validation result. This is required for senior-model orders like edit files, run checks, update the current task status, and commit only on success.","priority":"critical","tags":["workflow","conditionals","reliability","tasks"],"created_at":"2026-05-08T13:00:51.969415Z","updated_at":"2026-05-08T13:02:35.693917Z"} -/
namespace MCPAIHelper.Tasks

def task_012_id : String := "task-012"
def task_012_status : String := "todo"
def task_012_title : String := "Add declarative conditional workflow execution"
def task_012_body : String := "Implement if/then/else logic in workflows: branch on command exit code, file state, test result, grep match, task state or validation result. This is required for senior-model orders like edit files, run checks, update the current task status, and commit only on success."
def task_012_priority : String := "critical"

end MCPAIHelper.Tasks
