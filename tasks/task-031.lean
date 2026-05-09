/- mcp-ai-helper-task {"id":"task-031","status":"in_progress","title":"Remove dev wrapper from public production release","body":"Public repository and README should describe production usage through built artifacts/binaries, not local dev wrappers. Remove wrapper from public tree or clearly exclude it from release-facing docs, then rewrite GitHub public history if needed.","priority":"high","tags":["production","docs","release","security"],"created_at":"2026-05-08T14:34:30.870161Z","updated_at":"2026-05-08T14:34:30.870161Z"} -/
namespace MCPAIHelper.Tasks

def task_031_id : String := "task-031"
def task_031_status : String := "in_progress"
def task_031_title : String := "Remove dev wrapper from public production release"
def task_031_body : String := "Public repository and README should describe production usage through built artifacts/binaries, not local dev wrappers. Remove wrapper from public tree or clearly exclude it from release-facing docs, then rewrite GitHub public history if needed."
def task_031_priority : String := "high"

end MCPAIHelper.Tasks
