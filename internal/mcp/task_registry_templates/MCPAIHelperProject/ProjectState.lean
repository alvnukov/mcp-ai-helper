namespace MCPAIHelperProject

structure ArtifactId where
  value : String
deriving Repr, DecidableEq

inductive ArtifactKind where
  | task
  | module
  | tool
  | test
  | document
deriving Repr, DecidableEq

inductive LifecycleStatus where
  | proposed
  | active
  | verified
  | blocked
  | archived
deriving Repr, DecidableEq

inductive Priority where
  | low
  | normal
  | high
  | critical
deriving Repr, DecidableEq

inductive ModelLevel where
  | low
  | medium
  | high
  | veryHigh
deriving Repr, DecidableEq

structure Artifact where
  id : ArtifactId
  kind : ArtifactKind
  status : LifecycleStatus
  priority : Priority
  modelLevel : Option ModelLevel := none
  title : String
  body : String := ""
  tags : List String := []
  taskType : String := ""
  branch : String := ""
  worktreePath : String := ""
  acceptanceCriteria : List String := []
  verificationPlan : List String := []
  createdAt : String := ""
  updatedAt : String := ""
deriving Repr, DecidableEq

inductive RelationKind where
  | dependsOn
  | blocks
deriving Repr, DecidableEq

structure ArtifactRelation where
  source : ArtifactId
  target : ArtifactId
  kind : RelationKind
deriving Repr, DecidableEq

def dependency (source target : ArtifactId) : ArtifactRelation :=
  { source := source, target := target, kind := .dependsOn }

def blocking (source target : ArtifactId) : ArtifactRelation :=
  { source := source, target := target, kind := .blocks }

end MCPAIHelperProject
