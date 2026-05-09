/- mcp-ai-helper-task {"id":"task-014","status":"todo","title":"Add stronger idempotent file editing tools","body":"Expand beyond guarded replace: structured patch application, create-if-absent, replace block by markers, JSON/YAML edits, append unique line/block, delete exact block, and dry-run diff. All edits must be guarded by hashes or explicit conflict policy.","priority":"critical","tags":["fileops","idempotency","safety"],"created_at":"2026-05-08T13:00:51.969875Z","updated_at":"2026-05-08T13:00:51.969875Z"} -/
namespace MCPAIHelper.Tasks

def task_014_id : String := "task-014"
def task_014_status : String := "todo"
def task_014_title : String := "Add stronger idempotent file editing tools"
def task_014_body : String := "Expand beyond guarded replace: structured patch application, create-if-absent, replace block by markers, JSON/YAML edits, append unique line/block, delete exact block, and dry-run diff. All edits must be guarded by hashes or explicit conflict policy."
def task_014_priority : String := "critical"

end MCPAIHelper.Tasks
