/- mcp-ai-helper-task {"id":"task-011","status":"done","title":"Make helper layers configurable and hide disabled surfaces","body":"Added config layers for logs, tasks, guidance, models, commands and workflows. Disabled layers should not register their MCP tools/prompts/resources during discovery, reducing token load for the calling model.","priority":"high","tags":["config","layers","discovery"],"created_at":"2026-05-08T13:00:51.969095Z","updated_at":"2026-05-08T13:00:51.969095Z"} -/
namespace MCPAIHelper.Tasks

def task_011_id : String := "task-011"
def task_011_status : String := "done"
def task_011_title : String := "Make helper layers configurable and hide disabled surfaces"
def task_011_body : String := "Added config layers for logs, tasks, guidance, models, commands and workflows. Disabled layers should not register their MCP tools/prompts/resources during discovery, reducing token load for the calling model."
def task_011_priority : String := "high"

end MCPAIHelper.Tasks
