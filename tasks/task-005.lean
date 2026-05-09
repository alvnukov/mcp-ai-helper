/- mcp-ai-helper-task {"id":"task-005","status":"done","title":"Move logs to repo-scoped storage","body":"Moved command logs under the helper home repo namespace, targeting ~/.mcp-ai-helper/repos/\u003cproject\u003e/logs so logs can be searched, compressed, archived and cleaned per project.","priority":"high","tags":["logs","storage"],"created_at":"2026-05-08T13:00:51.967592Z","updated_at":"2026-05-08T13:00:51.967592Z"} -/
namespace MCPAIHelper.Tasks

def task_005_id : String := "task-005"
def task_005_status : String := "done"
def task_005_title : String := "Move logs to repo-scoped storage"
def task_005_body : String := "Moved command logs under the helper home repo namespace, targeting ~/.mcp-ai-helper/repos/<project>/logs so logs can be searched, compressed, archived and cleaned per project."
def task_005_priority : String := "high"

end MCPAIHelper.Tasks
