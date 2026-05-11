import Lean
import Lean.Server.Rpc.RequestHandling
import MCPAIHelperProject.Registry

open Lean
open Lean Server

namespace MCPAIHelperProject
namespace TaskRegistryExport

def kindString : ArtifactKind -> String
  | .task => "task"
  | .module => "module"
  | .tool => "tool"
  | .test => "test"
  | .document => "document"

def statusString : LifecycleStatus -> String
  | .proposed => "todo"
  | .active => "in_progress"
  | .verified => "done"
  | .blocked => "blocked"
  | .archived => "done"

def priorityString : Priority -> String
  | .low => "low"
  | .normal => "medium"
  | .high => "high"
  | .critical => "critical"

def modelLevelString : ModelLevel -> String
  | .low => "low"
  | .medium => "medium"
  | .high => "high"
  | .veryHigh => "very_high"

def modelLevelString? : Option ModelLevel -> String
  | some level => modelLevelString level
  | none => ""

def isTask (artifact : Artifact) : Bool :=
  match artifact.kind with
  | .task => true
  | _ => false

def isActiveTask (artifact : Artifact) : Bool :=
  isTask artifact &&
    match artifact.status with
    | .proposed => true
    | .active => true
    | .blocked => true
    | _ => false

def relationTargets (registry : Registry) (artifact : Artifact) (kind : RelationKind) : List String :=
  registry.relations.filterMap fun relation =>
    if relation.source.value == artifact.id.value && relation.kind == kind then
      some relation.target.value
    else
      none

def stringArrayJson (values : List String) : Json :=
  Json.arr (values.map Json.str).toArray

def artifactJson (registry : Registry) (artifact : Artifact) : Json :=
  Json.mkObj [
    ("id", Json.str artifact.id.value),
    ("status", Json.str (statusString artifact.status)),
    ("title", Json.str artifact.title),
    ("body", Json.str artifact.body),
    ("priority", Json.str (priorityString artifact.priority)),
    ("model_level", Json.str (modelLevelString? artifact.modelLevel)),
    ("tags", stringArrayJson artifact.tags),
    ("task_type", Json.str artifact.taskType),
    ("branch", Json.str artifact.branch),
    ("worktree_path", Json.str artifact.worktreePath),
    ("acceptance_criteria", stringArrayJson artifact.acceptanceCriteria),
    ("verification_plan", stringArrayJson artifact.verificationPlan),
    ("created_at", Json.str artifact.createdAt),
    ("updated_at", Json.str artifact.updatedAt),
    ("depends_on", stringArrayJson (relationTargets registry artifact .dependsOn)),
    ("blocks", stringArrayJson (relationTargets registry artifact .blocks))]

def taskArtifacts (registry : Registry) : List Artifact :=
  registry.artifacts.filter isTask

def activeTaskArtifacts (registry : Registry) : List Artifact :=
  registry.artifacts.filter isActiveTask

def taskListJson (registry : Registry) (artifacts : List Artifact) : Json :=
  Json.mkObj [("tasks", Json.arr (artifacts.map (artifactJson registry)).toArray)]

def getTaskJson? (registry : Registry) (id : String) : Option Json :=
  (taskArtifacts registry).find? (fun artifact => artifact.id.value == id) |>.map (artifactJson registry)

structure TaskListRequest where
  active : Bool
  deriving FromJson, ToJson

structure TaskGetRequest where
  id : String
  deriving FromJson, ToJson

structure TaskTransitionRequest where
  id : String
  to : String
  deriving FromJson, ToJson

structure TaskUpsertRequest where
  id : String
  status : String
  title : String
  body : String
  priority : String
  model_level : String
  tags : List String
  task_type : String
  branch : String
  worktree_path : String
  acceptance_criteria : List String
  verification_plan : List String
  deriving FromJson, ToJson

structure TaskBatchUpsertRequest where
  tasks : List TaskUpsertRequest
  close_missing : Bool
  missing_status : String
  active_statuses : List String
  deriving FromJson, ToJson

structure TaskDeleteRequest where
  id : String
  deriving FromJson, ToJson

structure ActiveTasksWriteRequest where
  source : String
  deriving FromJson, ToJson

def activeTasksLeanPath : String :=
  "MCPAIHelperProject/ActiveTasks.lean"

def stringContains (text needle : String) : Bool :=
  match text.splitOn needle with
  | _ :: _ :: _ => true
  | _ => false

def leanEscapedChar (char : Char) : String :=
  if char == '\\' then "\\\\"
  else if char == '"' then "\\\""
  else if char == '\n' then "\\n"
  else if char == '\r' then "\\r"
  else if char == '\t' then "\\t"
  else toString char

def leanString (value : String) : String :=
  "\"" ++ String.join (value.toList.map leanEscapedChar) ++ "\""

def leanStringList (values : List String) : String :=
  "[" ++ String.intercalate ", " (values.map leanString) ++ "]"

def lifecycleConstructor : LifecycleStatus -> String
  | .proposed => ".proposed"
  | .active => ".active"
  | .verified => ".verified"
  | .blocked => ".blocked"
  | .archived => ".archived"

def priorityConstructor : Priority -> String
  | .low => ".low"
  | .normal => ".normal"
  | .high => ".high"
  | .critical => ".critical"

def modelLevelConstructor : ModelLevel -> String
  | .low => ".low"
  | .medium => ".medium"
  | .high => ".high"
  | .veryHigh => ".veryHigh"

def relationKindConstructor : RelationKind -> String
  | .dependsOn => ".dependsOn"
  | .blocks => ".blocks"

def statusFromString? : String -> Option LifecycleStatus
  | "todo" => some .proposed
  | "in_progress" => some .active
  | "done" => some .verified
  | "blocked" => some .blocked
  | _ => none

def priorityFromString : String -> Priority
  | "low" => .low
  | "high" => .high
  | "critical" => .critical
  | _ => .normal

def modelLevelFromString? : String -> Option ModelLevel
  | "low" => some .low
  | "medium" => some .medium
  | "high" => some .high
  | "very_high" => some .veryHigh
  | _ => none

def renderModelLevelLine : Option ModelLevel -> String
  | none => ""
  | some level => "    modelLevel := some " ++ modelLevelConstructor level ++ ",\n"

def renderArtifactDefs (index : Nat) (artifact : Artifact) : String :=
  let name := "task" ++ toString index
  "def " ++ name ++ "Id : ArtifactId :=\n" ++
  "  { value := " ++ leanString artifact.id.value ++ " }\n\n" ++
  "def " ++ name ++ "Tags : List String :=\n" ++
  "  " ++ leanStringList artifact.tags ++ "\n\n" ++
  "def " ++ name ++ "AcceptanceCriteria : List String :=\n" ++
  "  " ++ leanStringList artifact.acceptanceCriteria ++ "\n\n" ++
  "def " ++ name ++ "VerificationPlan : List String :=\n" ++
  "  " ++ leanStringList artifact.verificationPlan ++ "\n\n" ++
  "def " ++ name ++ "Artifact : Artifact :=\n" ++
  "  { id := " ++ name ++ "Id,\n" ++
  "    kind := .task,\n" ++
  "    status := " ++ lifecycleConstructor artifact.status ++ ",\n" ++
  "    priority := " ++ priorityConstructor artifact.priority ++ ",\n" ++
  renderModelLevelLine artifact.modelLevel ++
  "    title := " ++ leanString artifact.title ++ ",\n" ++
  "    body := " ++ leanString artifact.body ++ ",\n" ++
  "    tags := " ++ name ++ "Tags,\n" ++
  "    taskType := " ++ leanString artifact.taskType ++ ",\n" ++
  "    branch := " ++ leanString artifact.branch ++ ",\n" ++
  "    worktreePath := " ++ leanString artifact.worktreePath ++ ",\n" ++
  "    acceptanceCriteria := " ++ name ++ "AcceptanceCriteria,\n" ++
  "    verificationPlan := " ++ name ++ "VerificationPlan,\n" ++
  "    createdAt := " ++ leanString artifact.createdAt ++ ",\n" ++
  "    updatedAt := " ++ leanString artifact.updatedAt ++ " }\n"

def renderRelationDefs (index : Nat) (relation : ArtifactRelation) : String :=
  let name := "relation" ++ toString index
  "def " ++ name ++ " : ArtifactRelation :=\n" ++
  "  { source := { value := " ++ leanString relation.source.value ++ " },\n" ++
  "    target := { value := " ++ leanString relation.target.value ++ " },\n" ++
  "    kind := " ++ relationKindConstructor relation.kind ++ " }\n"

def renderActiveTasksSource (artifacts : List Artifact) (relations : List ArtifactRelation) : String :=
  let indexed := List.zip (List.range artifacts.length) artifacts
  let relationIndexed := List.zip (List.range relations.length) relations
  let defs := String.intercalate "\n" (indexed.map (fun pair => renderArtifactDefs pair.fst pair.snd))
  let relationDefs := String.intercalate "\n" (relationIndexed.map (fun pair => renderRelationDefs pair.fst pair.snd))
  let names := indexed.map (fun pair => "task" ++ toString pair.fst ++ "Artifact")
  let relationNames := relationIndexed.map (fun pair => "relation" ++ toString pair.fst)
  "import MCPAIHelperProject.ProjectState\n\n" ++
  "namespace MCPAIHelperProject\n" ++
  "namespace ActiveTasks\n\n" ++
  defs ++ "\n" ++
  relationDefs ++ "\n" ++
  "def activeArtifacts : List Artifact :=\n" ++
  "  [" ++ String.intercalate ", " names ++ "]\n\n" ++
  "def activeRelations : List ArtifactRelation :=\n" ++
  "  [" ++ String.intercalate ", " relationNames ++ "]\n\n" ++
  "end ActiveTasks\n" ++
  "end MCPAIHelperProject\n"

def registryFromActiveArtifacts (artifacts : List Artifact) : Registry :=
  { artifacts := Samples.verifiedArtifacts ++ artifacts,
    relations := Samples.verifiedRelations ++ ActiveTasks.activeRelations }

def upsertArtifact : List Artifact -> Artifact -> List Artifact
  | [], artifact => [artifact]
  | current :: rest, artifact =>
      if current.id.value == artifact.id.value then
        artifact :: rest
      else
        current :: upsertArtifact rest artifact

def deleteArtifactById : List Artifact -> String -> List Artifact
  | [], _ => []
  | current :: rest, id =>
      if current.id.value == id then
        rest
      else
        current :: deleteArtifactById rest id

def taskIdFromRequest (req : TaskUpsertRequest) : String :=
  if req.id == "" then req.title else req.id

def artifactFromRequest (req : TaskUpsertRequest) (existing? : Option Artifact) : Except String Artifact :=
  if req.title == "" then
    Except.error "title is required"
  else
    match statusFromString? (if req.status == "" then "todo" else req.status) with
    | none => Except.error ("unsupported task status: " ++ req.status)
    | some status =>
        let id := taskIdFromRequest req
        if id == "" then
          Except.error "task id is empty after normalization"
        else
          Except.ok {
            id := { value := id },
            kind := .task,
            status := status,
            priority := priorityFromString req.priority,
            modelLevel := modelLevelFromString? req.model_level,
            title := req.title,
            body := req.body,
            tags := req.tags,
            taskType := req.task_type,
            branch := req.branch,
            worktreePath := req.worktree_path,
            acceptanceCriteria := req.acceptance_criteria,
            verificationPlan := req.verification_plan,
            createdAt := match existing? with | some existing => existing.createdAt | none => "",
            updatedAt := match existing? with | some existing => existing.updatedAt | none => "" }

def activeStatusStrings (statuses : List String) : List String :=
  match statuses with
  | [] => ["todo", "in_progress", "blocked"]
  | values => values

def registryOkEnvelopeWithChanges (operation : String) (data : Json) (changedFiles : List String) (summary : String) : Json :=
  Json.mkObj [
    ("schema_version", (1 : Json)),
    ("ok", (true : Json)),
    ("operation", Json.str operation),
    ("data", data),
    ("diagnostics", Json.arr #[]),
    ("changed_files", stringArrayJson changedFiles),
    ("validation", Json.mkObj [("checked", (true : Json)), ("summary", Json.str summary)])]

def registryOkEnvelope (operation : String) (data : Json) : Json :=
  registryOkEnvelopeWithChanges operation data [] "read-only query"

def registryErrorEnvelope (operation code message : String) : Json :=
  Json.mkObj [
    ("schema_version", (1 : Json)),
    ("ok", (false : Json)),
    ("operation", Json.str operation),
    ("data", Json.mkObj []),
    ("diagnostics", Json.arr #[Json.mkObj [("code", Json.str code), ("message", Json.str message), ("severity", Json.str "error")]]),
    ("changed_files", Json.arr #[]),
    ("validation", Json.mkObj [("checked", (true : Json)), ("summary", Json.str "read-only query")])]

def transitionTaskEnvelope (req : TaskTransitionRequest) : Json :=
  if !Registry.isValid projectRegistry then
    registryErrorEnvelope "task.transition" "invalid_registry" "registry invariant failed"
  else
    match statusFromString? req.to with
    | none =>
        registryErrorEnvelope "task.transition" "invalid_status" ("unsupported task status: " ++ req.to)
    | some next =>
        match (taskArtifacts projectRegistry).find? (fun artifact => artifact.id.value == req.id) with
        | none =>
            registryErrorEnvelope "task.transition" "not_found" ("task not found: " ++ req.id)
        | some artifact =>
            let updated := { artifact with status := next }
            registryOkEnvelopeWithChanges
              "task.transition"
              (Json.mkObj [("task", artifactJson projectRegistry updated)])
              ["MCPAIHelperProject/ActiveTasks.lean"]
              "server-side transition validation passed"

@[server_rpc_method]
def taskList (req : TaskListRequest) : RequestM (RequestTask Json) := do
  RequestM.asTask do
    if !Registry.isValid projectRegistry then
      pure (registryErrorEnvelope "task.list" "invalid_registry" "registry invariant failed")
    else
      let artifacts := if req.active then activeTaskArtifacts projectRegistry else taskArtifacts projectRegistry
      pure (registryOkEnvelope "task.list" (taskListJson projectRegistry artifacts))

@[server_rpc_method]
def taskGet (req : TaskGetRequest) : RequestM (RequestTask Json) := do
  RequestM.asTask do
    match getTaskJson? projectRegistry req.id with
    | some data => pure (registryOkEnvelope "task.get" data)
    | none => pure (registryErrorEnvelope "task.get" "not_found" ("task not found: " ++ req.id))

@[server_rpc_method]
def taskTransition (req : TaskTransitionRequest) : RequestM (RequestTask Json) := do
  RequestM.asTask do
    pure (transitionTaskEnvelope req)

@[server_rpc_method]
def taskTransitionApply (req : TaskTransitionRequest) : RequestM (RequestTask Json) := do
  RequestM.asTask do
    if !Registry.isValid projectRegistry then
      pure (registryErrorEnvelope "task.transition.apply" "invalid_registry" "registry invariant failed")
    else
      match statusFromString? req.to with
      | none =>
          pure (registryErrorEnvelope "task.transition.apply" "invalid_status" ("unsupported task status: " ++ req.to))
      | some next =>
          match ActiveTasks.activeArtifacts.find? (fun artifact => artifact.id.value == req.id) with
          | none =>
              pure (registryErrorEnvelope "task.transition.apply" "not_found" ("task not found: " ++ req.id))
          | some artifact =>
              let previous ← IO.FS.readFile activeTasksLeanPath
              let updated := { artifact with status := next }
              let artifacts := upsertArtifact ActiveTasks.activeArtifacts updated
              IO.FS.writeFile activeTasksLeanPath (renderActiveTasksSource artifacts ActiveTasks.activeRelations)
              let registry := registryFromActiveArtifacts artifacts
              pure (registryOkEnvelopeWithChanges
                "task.transition.apply"
                (Json.mkObj [("task", artifactJson registry updated), ("previous_source", Json.str previous)])
                [activeTasksLeanPath]
                "server-side transition applied")

@[server_rpc_method]
def taskUpsertApply (req : TaskUpsertRequest) : RequestM (RequestTask Json) := do
  RequestM.asTask do
    if !Registry.isValid projectRegistry then
      pure (registryErrorEnvelope "task.upsert.apply" "invalid_registry" "registry invariant failed")
    else
      let previous ← IO.FS.readFile activeTasksLeanPath
      let existing? := ActiveTasks.activeArtifacts.find? (fun artifact => artifact.id.value == taskIdFromRequest req)
      match artifactFromRequest req existing? with
      | Except.error message =>
          pure (registryErrorEnvelope "task.upsert.apply" "invalid_request" message)
      | Except.ok artifact =>
          let artifacts := upsertArtifact ActiveTasks.activeArtifacts artifact
          IO.FS.writeFile activeTasksLeanPath (renderActiveTasksSource artifacts ActiveTasks.activeRelations)
          let registry := registryFromActiveArtifacts artifacts
          pure (registryOkEnvelopeWithChanges
            "task.upsert.apply"
            (Json.mkObj [("task", artifactJson registry artifact), ("previous_source", Json.str previous)])
            [activeTasksLeanPath]
            "server-side upsert applied")

@[server_rpc_method]
def taskBatchUpsertApply (req : TaskBatchUpsertRequest) : RequestM (RequestTask Json) := do
  RequestM.asTask do
    if !Registry.isValid projectRegistry then
      pure (registryErrorEnvelope "task.batch_upsert.apply" "invalid_registry" "registry invariant failed")
    else
      let previous ← IO.FS.readFile activeTasksLeanPath
      let initialArtifacts := ActiveTasks.activeArtifacts
      let step := fun (state : List Artifact × List Artifact) (item : TaskUpsertRequest) =>
        let existing? := state.fst.find? (fun artifact => artifact.id.value == taskIdFromRequest item)
        match artifactFromRequest item existing? with
        | Except.error _ => state
        | Except.ok artifact => (upsertArtifact state.fst artifact, state.snd ++ [artifact])
      let state := req.tasks.foldl step (initialArtifacts, [])
      let seen := state.snd.map (fun artifact => artifact.id.value)
      let activeStatuses := activeStatusStrings req.active_statuses
      let missingStatus := if req.missing_status == "" then "done" else req.missing_status
      match statusFromString? missingStatus with
      | none =>
          pure (registryErrorEnvelope "task.batch_upsert.apply" "invalid_status" ("unsupported task status: " ++ missingStatus))
      | some closeStatus =>
          let closed := if req.close_missing then
              state.fst.filterMap fun artifact =>
                if seen.contains artifact.id.value then
                  none
                else if activeStatuses.contains (statusString artifact.status) then
                  some { artifact with status := closeStatus }
                else
                  none
            else
              []
          let finalArtifacts := state.fst.map fun artifact =>
            match closed.find? (fun closedArtifact => closedArtifact.id.value == artifact.id.value) with
            | some closedArtifact => closedArtifact
            | none => artifact
          IO.FS.writeFile activeTasksLeanPath (renderActiveTasksSource finalArtifacts ActiveTasks.activeRelations)
          let registry := registryFromActiveArtifacts finalArtifacts
          pure (registryOkEnvelopeWithChanges
            "task.batch_upsert.apply"
            (Json.mkObj [
              ("upserted", Json.arr (state.snd.map (artifactJson registry)).toArray),
              ("closed", Json.arr (closed.map (artifactJson registry)).toArray),
              ("previous_source", Json.str previous)])
            [activeTasksLeanPath]
            "server-side batch upsert applied")

@[server_rpc_method]
def taskDeleteApply (req : TaskDeleteRequest) : RequestM (RequestTask Json) := do
  RequestM.asTask do
    if !Registry.isValid projectRegistry then
      pure (registryErrorEnvelope "task.delete.apply" "invalid_registry" "registry invariant failed")
    else
      match ActiveTasks.activeArtifacts.find? (fun artifact => artifact.id.value == req.id) with
      | none =>
          pure (registryErrorEnvelope "task.delete.apply" "not_found" ("task not found: " ++ req.id))
      | some artifact =>
          let previous ← IO.FS.readFile activeTasksLeanPath
          let artifacts := deleteArtifactById ActiveTasks.activeArtifacts req.id
          IO.FS.writeFile activeTasksLeanPath (renderActiveTasksSource artifacts ActiveTasks.activeRelations)
          let registry := registryFromActiveArtifacts artifacts
          pure (registryOkEnvelopeWithChanges
            "task.delete.apply"
            (Json.mkObj [("task", artifactJson registry artifact), ("previous_source", Json.str previous)])
            [activeTasksLeanPath]
            "server-side delete applied")

@[server_rpc_method]
def activeTasksWrite (req : ActiveTasksWriteRequest) : RequestM (RequestTask Json) := do
  RequestM.asTask do
    IO.FS.writeFile activeTasksLeanPath req.source
    pure (registryOkEnvelopeWithChanges "task.active_tasks.write" (Json.mkObj []) [activeTasksLeanPath] "active tasks source written")

def writeJson (json : Json) : IO Unit :=
  IO.println json.compress

def fail (message : String) : IO UInt32 := do
  IO.eprintln message
  return 1

def run (args : List String) : IO UInt32 := do
  if !Registry.isValid projectRegistry then
    return ← fail "registry invariant failed"
  match args with
  | ["--list-active"] =>
      writeJson (taskListJson projectRegistry (activeTaskArtifacts projectRegistry))
      return 0
  | ["--list-all"] =>
      writeJson (taskListJson projectRegistry (taskArtifacts projectRegistry))
      return 0
  | ["--get", id] =>
      match getTaskJson? projectRegistry id with
      | some json =>
          writeJson json
          return 0
      | none =>
          return ← fail ("task not found: " ++ id)
  | _ =>
      return ← fail "usage: task_registry_export --list-active | --list-all | --get <task-id>"

end TaskRegistryExport
end MCPAIHelperProject

def main (args : List String) : IO UInt32 :=
  MCPAIHelperProject.TaskRegistryExport.run args
