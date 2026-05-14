import MCPAIHelperProject.ProjectState

namespace MCPAIHelperProject
namespace ActiveTasks

def activeArtifacts : List Artifact :=
  [
    { 
  id := "task-006",
  kind := .feature,
  status := MCPAIHelperProject.LifecycleStatus.todo,
  title := "Test task 006",
  body := "Test body for task-006",
  priority := .high,
  tags := ["test"]
}
  ]

def activeRelations : List ArtifactRelation :=
  []

end ActiveTasks
end MCPAIHelperProject
