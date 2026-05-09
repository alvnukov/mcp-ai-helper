/- mcp-ai-helper-task {"id":"task-032","status":"in_progress","title":"Ensure config is created on first server startup","body":"Diagnose and fix why .mcp-ai-helper/config.yaml is not created on first startup in both dev and production launch paths. Add a regression test and keep public history sanitized.","priority":"critical","tags":["config","startup","production"],"created_at":"2026-05-08T14:55:49.393926Z","updated_at":"2026-05-08T14:55:49.393926Z"} -/
namespace MCPAIHelper.Tasks

def task_032_id : String := "task-032"
def task_032_status : String := "in_progress"
def task_032_title : String := "Ensure config is created on first server startup"
def task_032_body : String := "Diagnose and fix why .mcp-ai-helper/config.yaml is not created on first startup in both dev and production launch paths. Add a regression test and keep public history sanitized."
def task_032_priority : String := "critical"

end MCPAIHelper.Tasks
