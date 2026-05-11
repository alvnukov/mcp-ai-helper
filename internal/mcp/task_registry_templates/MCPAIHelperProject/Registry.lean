import MCPAIHelperProject.Samples
import MCPAIHelperProject.ActiveTasks

namespace MCPAIHelperProject

structure Registry where
  artifacts : List Artifact
  relations : List ArtifactRelation

def artifactIdValues (artifacts : List Artifact) : List String :=
  artifacts.map (fun artifact => artifact.id.value)

def uniqueStrings : List String -> Bool
  | [] => true
  | id :: rest => !rest.contains id && uniqueStrings rest

def hasArtifactId (artifacts : List Artifact) (id : ArtifactId) : Bool :=
  artifacts.any (fun artifact => artifact.id.value == id.value)

def relationEndpointsExist (artifacts : List Artifact) (relation : ArtifactRelation) : Bool :=
  hasArtifactId artifacts relation.source && hasArtifactId artifacts relation.target

def relationIsNotSelf (relation : ArtifactRelation) : Bool :=
  relation.source.value != relation.target.value

namespace Registry

def idsAreNonEmpty (registry : Registry) : Bool :=
  registry.artifacts.all (fun artifact => !artifact.id.value.isEmpty)

def idsAreUnique (registry : Registry) : Bool :=
  uniqueStrings (artifactIdValues registry.artifacts)

def relationEndpointsAreKnown (registry : Registry) : Bool :=
  registry.relations.all (relationEndpointsExist registry.artifacts)

def relationsAreNotSelfReferential (registry : Registry) : Bool :=
  registry.relations.all relationIsNotSelf

def isValid (registry : Registry) : Bool :=
  idsAreNonEmpty registry &&
    idsAreUnique registry &&
    relationEndpointsAreKnown registry &&
    relationsAreNotSelfReferential registry

end Registry

def projectRegistry : Registry :=
  { artifacts := Samples.verifiedArtifacts ++ ActiveTasks.activeArtifacts,
    relations := Samples.verifiedRelations ++ ActiveTasks.activeRelations }

#guard Registry.isValid projectRegistry

end MCPAIHelperProject
