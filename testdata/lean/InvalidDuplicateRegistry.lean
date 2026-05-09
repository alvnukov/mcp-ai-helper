import MCPAIHelperProject.Registry

open MCPAIHelperProject

def invalidRegistry : Registry :=
  { artifacts := [Samples.task034Artifact, Samples.task034Artifact],
    relations := [] }

#guard Registry.isValid invalidRegistry
