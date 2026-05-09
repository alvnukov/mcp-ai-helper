/- mcp-ai-helper-task {"id":"task-015","status":"todo","title":"Harden git commit workflow for dirty and partially owned files","body":"Add preflight and commit semantics that detect already modified files, staged unrelated files, untracked files, changed hashes after snapshot, and patch conflicts. Commit only owned files and fail closed with a compact blocker report.","priority":"critical","tags":["git","safety","workflow"],"created_at":"2026-05-08T13:00:51.970055Z","updated_at":"2026-05-08T13:00:51.970055Z"} -/
namespace MCPAIHelper.Tasks

def task_015_id : String := "task-015"
def task_015_status : String := "todo"
def task_015_title : String := "Harden git commit workflow for dirty and partially owned files"
def task_015_body : String := "Add preflight and commit semantics that detect already modified files, staged unrelated files, untracked files, changed hashes after snapshot, and patch conflicts. Commit only owned files and fail closed with a compact blocker report."
def task_015_priority : String := "critical"

end MCPAIHelper.Tasks
