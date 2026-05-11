import MCPAIHelperProject.ProjectState

namespace MCPAIHelperProject
namespace Samples

def task034Id : ArtifactId :=
  { value := "task-034" }

def task035Id : ArtifactId :=
  { value := "task-035" }

def task034LakeBackendId : ArtifactId :=
  { value := "lake-backend-smoke-path" }

def task034Artifact : Artifact :=
  { id := task034Id,
    kind := .task,
    status := .verified,
    priority := .critical,
    title := "Add Lean/Lake workspace backend smoke path" }

def task035Artifact : Artifact :=
  { id := task035Id,
    kind := .task,
    status := .verified,
    priority := .critical,
    title := "Bootstrap repo-local Lake project for verified project state" }

def task034LakeBackendArtifact : Artifact :=
  { id := task034LakeBackendId,
    kind := .tool,
    status := .verified,
    priority := .critical,
    title := "Lake backend can detect and run repo-local workspaces" }

def task034BackendDependency : ArtifactRelation :=
  dependency task034Id task034LakeBackendId

def task035DependsOnTask034 : ArtifactRelation :=
  dependency task035Id task034Id

def verifiedArtifacts : List Artifact :=
  [task034Artifact, task034LakeBackendArtifact, task035Artifact]

def verifiedRelations : List ArtifactRelation :=
  [task034BackendDependency, task035DependsOnTask034]

end Samples
end MCPAIHelperProject
