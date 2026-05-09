/- mcp-ai-helper-task {"id":"task-030","status":"in_progress","title":"Sanitize provider references and rewrite GitHub history","body":"Remove Polza-specific references from config, code and tests; verify the repository tree no longer contains provider-specific names; rewrite GitHub main history to a sanitized single commit.","priority":"critical","tags":["security","github","config","history"],"created_at":"2026-05-08T14:26:43.644302Z","updated_at":"2026-05-08T14:26:43.644302Z"} -/
namespace MCPAIHelper.Tasks

def task_030_id : String := "task-030"
def task_030_status : String := "in_progress"
def task_030_title : String := "Sanitize provider references and rewrite GitHub history"
def task_030_body : String := "Remove Polza-specific references from config, code and tests; verify the repository tree no longer contains provider-specific names; rewrite GitHub main history to a sanitized single commit."
def task_030_priority : String := "critical"

end MCPAIHelper.Tasks
