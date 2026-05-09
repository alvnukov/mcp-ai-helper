/- mcp-ai-helper-task {"id":"task-003","status":"done","title":"Implement local command pipeline with bounded output","body":"Added command execution through MCP with repo_path, cwd, timeout and output limits. The pipeline returns compact evidence instead of forcing the caller to read raw noisy logs.","priority":"critical","tags":["pipeline","commands","safety"],"created_at":"2026-05-08T13:00:51.967135Z","updated_at":"2026-05-08T13:00:51.967135Z"} -/
namespace MCPAIHelper.Tasks

def task_003_id : String := "task-003"
def task_003_status : String := "done"
def task_003_title : String := "Implement local command pipeline with bounded output"
def task_003_body : String := "Added command execution through MCP with repo_path, cwd, timeout and output limits. The pipeline returns compact evidence instead of forcing the caller to read raw noisy logs."
def task_003_priority : String := "critical"

end MCPAIHelper.Tasks
