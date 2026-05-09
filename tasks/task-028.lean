/- mcp-ai-helper-task {"id":"task-028","status":"done","title":"Add feature flag to disable feedback issue intake tools","body":"Done in commit 2554388. Added an issues layer flag in config/default config/example config. When layers.issues.enabled=false, issue_add, issue_list and issue_accept are hidden from MCP discovery while regular task tools remain available. This lets dev machines keep feedback intake enabled and production disable it.","priority":"high","tags":["config","issues","production","mcp"],"created_at":"2026-05-08T14:20:00.271466Z","updated_at":"2026-05-08T14:21:26.352953Z"} -/
namespace MCPAIHelper.Tasks

def task_028_id : String := "task-028"
def task_028_status : String := "done"
def task_028_title : String := "Add feature flag to disable feedback issue intake tools"
def task_028_body : String := "Done in commit 2554388. Added an issues layer flag in config/default config/example config. When layers.issues.enabled=false, issue_add, issue_list and issue_accept are hidden from MCP discovery while regular task tools remain available. This lets dev machines keep feedback intake enabled and production disable it."
def task_028_priority : String := "high"

end MCPAIHelper.Tasks
