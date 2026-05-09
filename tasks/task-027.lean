/- mcp-ai-helper-task {"id":"task-027","status":"done","title":"Accept cross-repository feedback as actionable issues","body":"Done in commit f2e97a1. Added issue_add, issue_list and issue_accept MCP tools backed by the repo task store. Feedback from another repository can be recorded into the target repository as a todo issue with source_repo_path preserved; later, in the target repo, issue_accept moves it to in_progress.","priority":"critical","tags":["feedback","issues","tasks","mcp"],"created_at":"2026-05-08T13:19:00.467388Z","updated_at":"2026-05-08T13:21:11.878201Z"} -/
namespace MCPAIHelper.Tasks

def task_027_id : String := "task-027"
def task_027_status : String := "done"
def task_027_title : String := "Accept cross-repository feedback as actionable issues"
def task_027_body : String := "Done in commit f2e97a1. Added issue_add, issue_list and issue_accept MCP tools backed by the repo task store. Feedback from another repository can be recorded into the target repository as a todo issue with source_repo_path preserved; later, in the target repo, issue_accept moves it to in_progress."
def task_027_priority : String := "critical"

end MCPAIHelper.Tasks
