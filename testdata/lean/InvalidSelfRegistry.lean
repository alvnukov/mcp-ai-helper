import MCPAIHelperProject.Registry

open MCPAIHelperProject

def invalidRegistry : Registry :=
  { artifacts := [Samples.task034Artifact],
    relations := [dependency Samples.task034Id Samples.task034Id] }

#guard Registry.isValid invalidRegistry
