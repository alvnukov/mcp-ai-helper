import Lake
open Lake DSL

package mcp_ai_helper

@[default_target]
lean_lib MCPAIHelperProject

lean_exe task_registry_export where
  root := `MCPAIHelperProject.TaskRegistryExport
