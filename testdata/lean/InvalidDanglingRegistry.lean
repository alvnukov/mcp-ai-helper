import MCPAIHelperProject.Registry

open MCPAIHelperProject

def missingArtifactId : ArtifactId :=
  { value := "missing-artifact" }

def invalidRegistry : Registry :=
  { artifacts := [Samples.task034Artifact],
    relations := [dependency Samples.task034Id missingArtifactId] }

#guard Registry.isValid invalidRegistry
