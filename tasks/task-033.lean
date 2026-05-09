/- mcp-ai-helper-task {"id":"task-033","status":"done","title":"Use home helper directory as default config location","body":"Fix default startup config location to use ~/.mcp-ai-helper/config.yaml as originally specified. Repo-local .mcp-ai-helper/config.yaml should only be an explicit/local override, not the production default.","priority":"critical","tags":["config","startup","paths"],"created_at":"2026-05-08T14:56:58.603161Z","updated_at":"2026-05-08T15:09:19Z"} -/
namespace MCPAIHelper.Tasks

def task_033_id : String := "task-033"
def task_033_status : String := "done"
def task_033_title : String := "Use home helper directory as default config location"
def task_033_body : String := "Fix default startup config location to use ~/.mcp-ai-helper/config.yaml as originally specified. Repo-local .mcp-ai-helper/config.yaml should only be an explicit/local override, not the production default."
def task_033_priority : String := "critical"

end MCPAIHelper.Tasks
