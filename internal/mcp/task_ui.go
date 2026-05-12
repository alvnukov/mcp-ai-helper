package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

type taskUIBackend interface {
	List(context.Context, string) ([]tasks.Task, string, error)
	Get(context.Context, string, string) (tasks.Task, string, error)
	Upsert(context.Context, tasks.AddRequest) (leanMutationResult, error)
	SetStatus(context.Context, tasks.StatusRequest) (leanMutationResult, error)
}

type serverTaskUIBackend struct {
	deps *Server
}

func (b serverTaskUIBackend) List(ctx context.Context, repoPath string) ([]tasks.Task, string, error) {
	backend := b.deps.loadTaskBackend()
	return backend.ListAll(ctx, repoPath)
}

func (b serverTaskUIBackend) Get(ctx context.Context, repoPath string, id string) (tasks.Task, string, error) {
	backend := b.deps.loadTaskBackend()
	return backend.Get(ctx, repoPath, id)
}

func (b serverTaskUIBackend) Upsert(ctx context.Context, req tasks.AddRequest) (leanMutationResult, error) {
	backend := b.deps.loadTaskBackend()
	return backend.Upsert(ctx, req)
}

func (b serverTaskUIBackend) SetStatus(ctx context.Context, req tasks.StatusRequest) (leanMutationResult, error) {
	backend := b.deps.loadTaskBackend()
	return backend.SetStatus(ctx, req)
}

func newServerTaskUIHandler(deps *Server) http.Handler {
	return newTaskUIHandler(serverTaskUIBackend{deps: deps})
}

func newTaskUIHandler(backend taskUIBackend) http.Handler {
	ui := &taskUI{backend: backend}
	mux := http.NewServeMux()
	mux.HandleFunc("/", ui.index)
	mux.HandleFunc("/api/tasks", ui.tasks)
	mux.HandleFunc("/api/tasks/", ui.taskByID)
	return mux
}

type taskUI struct{ backend taskUIBackend }

type taskUpsertRequest struct {
	RepoPath           string   `json:"repo_path"`
	ID                 string   `json:"id"`
	TaskType           string   `json:"task_type"`
	Title              string   `json:"title"`
	Body               string   `json:"body"`
	Status             string   `json:"status"`
	Priority           string   `json:"priority"`
	ModelLevel         string   `json:"model_level"`
	Tags               []string `json:"tags"`
	AcceptanceCriteria []string `json:"acceptance_criteria"`
	VerificationPlan   []string `json:"verification_plan"`
	UpdatedAt          string   `json:"updated_at"`
}

type taskStatusRequest struct {
	RepoPath  string `json:"repo_path"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updated_at"`
}

func (ui *taskUI) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(taskUIHTML))
}

func (ui *taskUI) tasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		repoPath := strings.TrimSpace(r.URL.Query().Get("repo_path"))
		items, source, err := ui.backend.List(r.Context(), repoPath)
		if err != nil {
			writeTaskUIError(w, http.StatusBadGateway, err)
			return
		}
		writeTaskUIJSON(w, http.StatusOK, map[string]any{"tasks": filterUITasks(items, r), "source": source})
	case http.MethodPost:
		var req taskUpsertRequest
		if err := decodeTaskUIJSON(r, &req); err != nil {
			writeTaskUIError(w, http.StatusBadRequest, err)
			return
		}
		if strings.TrimSpace(req.ID) != "" && strings.TrimSpace(req.UpdatedAt) != "" {
			if err := ui.checkStale(r.Context(), req.RepoPath, req.ID, req.UpdatedAt); err != nil {
				writeTaskUIError(w, http.StatusConflict, err)
				return
			}
		}
		result, err := ui.backend.Upsert(r.Context(), upsertRequestFromUI(req))
		if err != nil {
			writeTaskUIError(w, http.StatusBadGateway, err)
			return
		}
		writeTaskUIJSON(w, http.StatusOK, result)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeTaskUIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

func (ui *taskUI) taskByID(w http.ResponseWriter, r *http.Request) {
	rest := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	if strings.HasSuffix(rest, "/status") {
		id := strings.TrimSuffix(rest, "/status")
		ui.taskStatus(w, r, id)
		return
	}
	id := strings.Trim(rest, "/")
	if id == "" || strings.Contains(id, "/") {
		http.NotFound(w, r)
		return
	}
	switch r.Method {
	case http.MethodGet:
		task, source, err := ui.backend.Get(r.Context(), r.URL.Query().Get("repo_path"), id)
		if err != nil {
			writeTaskUIError(w, http.StatusNotFound, err)
			return
		}
		writeTaskUIJSON(w, http.StatusOK, map[string]any{"task": task, "source": source})
	case http.MethodPost:
		var req taskUpsertRequest
		if err := decodeTaskUIJSON(r, &req); err != nil {
			writeTaskUIError(w, http.StatusBadRequest, err)
			return
		}
		req.ID = id
		if err := ui.checkStale(r.Context(), req.RepoPath, id, req.UpdatedAt); err != nil {
			writeTaskUIError(w, http.StatusConflict, err)
			return
		}
		result, err := ui.backend.Upsert(r.Context(), upsertRequestFromUI(req))
		if err != nil {
			writeTaskUIError(w, http.StatusBadGateway, err)
			return
		}
		writeTaskUIJSON(w, http.StatusOK, result)
	default:
		w.Header().Set("Allow", "GET, POST")
		writeTaskUIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
	}
}

func (ui *taskUI) taskStatus(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeTaskUIError(w, http.StatusMethodNotAllowed, errors.New("method not allowed"))
		return
	}
	var req taskStatusRequest
	if err := decodeTaskUIJSON(r, &req); err != nil {
		writeTaskUIError(w, http.StatusBadRequest, err)
		return
	}
	if err := ui.checkStale(r.Context(), req.RepoPath, id, req.UpdatedAt); err != nil {
		writeTaskUIError(w, http.StatusConflict, err)
		return
	}
	result, err := ui.backend.SetStatus(r.Context(), tasks.StatusRequest{RepoPath: req.RepoPath, ID: id, Status: req.Status})
	if err != nil {
		writeTaskUIError(w, http.StatusBadGateway, err)
		return
	}
	writeTaskUIJSON(w, http.StatusOK, result)
}

func (ui *taskUI) checkStale(ctx context.Context, repoPath string, id string, expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return errors.New("updated_at is required for guarded edits")
	}
	existing, _, err := ui.backend.Get(ctx, repoPath, id)
	if err != nil {
		return err
	}
	actual := existing.UpdatedAt.UTC().Format(time.RFC3339Nano)
	if actual != expected {
		return fmt.Errorf("stale task edit: updated_at is %s, got %s", actual, expected)
	}
	return nil
}

func upsertRequestFromUI(req taskUpsertRequest) tasks.AddRequest {
	return tasks.AddRequest{RepoPath: req.RepoPath, ID: req.ID, TaskType: req.TaskType, Status: req.Status, Title: req.Title, Body: req.Body, Priority: req.Priority, ModelLevel: req.ModelLevel, Tags: req.Tags, AcceptanceCriteria: req.AcceptanceCriteria, VerificationPlan: req.VerificationPlan}
}

func decodeTaskUIJSON(r *http.Request, target any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(target)
}

func writeTaskUIJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeTaskUIError(w http.ResponseWriter, status int, err error) {
	writeTaskUIJSON(w, status, map[string]any{"ok": false, "error": map[string]string{"message": err.Error()}})
}

func filterUITasks(items []tasks.Task, r *http.Request) []tasks.Task {
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("query")))
	status := strings.TrimSpace(r.URL.Query().Get("status"))
	priority := strings.TrimSpace(r.URL.Query().Get("priority"))
	tag := strings.TrimSpace(r.URL.Query().Get("tag"))
	out := make([]tasks.Task, 0, len(items))
	for _, item := range items {
		if status != "" && item.Status != status {
			continue
		}
		if priority != "" && item.Priority != priority {
			continue
		}
		if tag != "" && !taskHasTag(item, tag) {
			continue
		}
		if q != "" && !taskUIQueryMatch(item, q) {
			continue
		}
		out = append(out, item)
	}
	return out
}

func taskHasTag(task tasks.Task, tag string) bool {
	for _, item := range task.Tags {
		if item == tag {
			return true
		}
	}
	return false
}

func taskUIQueryMatch(task tasks.Task, q string) bool {
	haystack := strings.ToLower(strings.Join([]string{task.ID, task.Title, task.Body, task.Status, task.Priority, task.ModelLevel, strings.Join(task.Tags, " ")}, " "))
	return strings.Contains(haystack, q)
}

const taskUIHTML = `<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>mcp-ai-helper tasks</title><style>
body{margin:0;font-family:Inter,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#f6f7f9;color:#17191c}button,input,textarea,select{font:inherit}header{display:flex;gap:12px;align-items:center;padding:12px 16px;background:#20252b;color:white}header input{width:min(560px,48vw);padding:7px 9px;border:1px solid #535b66;border-radius:6px;background:#11161b;color:white}.wrap{display:grid;grid-template-columns:360px 1fr;height:calc(100vh - 57px)}aside{border-right:1px solid #d8dde3;overflow:auto;background:white}.filters{display:grid;grid-template-columns:1fr 1fr;gap:8px;padding:10px;border-bottom:1px solid #e2e6eb}.filters input,.filters select{padding:6px;border:1px solid #c9d0d8;border-radius:6px}.task{padding:10px 12px;border-bottom:1px solid #edf0f3;cursor:pointer}.task:hover,.task.active{background:#edf4ff}.task b{display:block;font-size:14px}.meta{font-size:12px;color:#5c6470;margin-top:4px}.main{overflow:auto;padding:16px 20px}.grid{display:grid;grid-template-columns:1fr 140px 140px 160px;gap:10px;margin-bottom:10px}label{font-size:12px;color:#555d67;display:block;margin-bottom:4px}input,textarea,select{box-sizing:border-box;width:100%;border:1px solid #c8cfd8;border-radius:6px;padding:7px;background:white}textarea{min-height:120px;resize:vertical}.actions{display:flex;gap:8px;margin:12px 0}.actions button{border:1px solid #b9c2cd;border-radius:6px;background:white;padding:7px 10px;cursor:pointer}.actions button.primary{background:#1769e0;border-color:#1769e0;color:white}.diag{white-space:pre-wrap;background:#11161b;color:#d6e2ff;border-radius:6px;padding:10px;font-size:12px;min-height:18px}.empty{padding:20px;color:#69727e}</style></head><body><header><strong>mcp-ai-helper tasks</strong><input id="repo" placeholder="repo_path"/><button onclick="loadTasks()">Load</button><button onclick="newTask()">New</button></header><div class="wrap"><aside><div class="filters"><select id="status" onchange="loadTasks()"><option value="">status</option><option>todo</option><option>in_progress</option><option>blocked</option><option>done</option></select><select id="priority" onchange="loadTasks()"><option value="">priority</option><option>critical</option><option>high</option><option>medium</option><option>low</option></select><input id="tag" placeholder="tag" oninput="debouncedLoad()"><input id="query" placeholder="query" oninput="debouncedLoad()"></div><div id="list" class="empty">Set repo_path and load tasks.</div></aside><main class="main"><div class="grid"><div><label>Title</label><input id="title"></div><div><label>Status</label><select id="edit_status"><option>todo</option><option>in_progress</option><option>blocked</option><option>done</option></select></div><div><label>Priority</label><select id="edit_priority"><option></option><option>critical</option><option>high</option><option>medium</option><option>low</option></select></div><div><label>Model</label><select id="model"><option></option><option>low</option><option>medium</option><option>high</option><option>very_high</option></select></div></div><label>ID</label><input id="id"><label>Tags</label><input id="tags"><label>Body</label><textarea id="body"></textarea><label>Acceptance criteria</label><textarea id="acceptance"></textarea><label>Verification plan</label><textarea id="verification"></textarea><div class="actions"><button class="primary" onclick="saveTask()">Save</button><button onclick="setStatus()">Set status</button></div><div id="diag" class="diag"></div></main></div><script>
const $=id=>document.getElementById(id);let current=null,timer=null;function lines(id){return $(id).value.split('\n').map(s=>s.trim()).filter(Boolean)}function tags(){return $('tags').value.split(',').map(s=>s.trim()).filter(Boolean)}function debouncedLoad(){clearTimeout(timer);timer=setTimeout(loadTasks,250)}function repo(){return $('repo').value.trim()}function qs(){let p=new URLSearchParams({repo_path:repo()});['status','priority','tag','query'].forEach(id=>{if($(id).value)p.set(id,$(id).value)});return p}async function api(url,opt){let r=await fetch(url,opt);let j=await r.json().catch(()=>({}));if(!r.ok)throw new Error(j.error?.message||r.statusText);return j}async function loadTasks(){try{let j=await api('/api/tasks?'+qs());$('list').innerHTML='';if(!j.tasks.length){$('list').className='empty';$('list').textContent='No tasks';return}$('list').className='';j.tasks.forEach(t=>{let d=document.createElement('div');d.className='task'+(current&&current.id===t.id?' active':'');d.innerHTML='<b>'+esc(t.id)+' '+esc(t.title)+'</b><div class="meta">'+esc(t.status)+' · '+esc(t.priority||'')+' · '+esc((t.tags||[]).join(', '))+'</div>';d.onclick=()=>openTask(t.id);$('list').appendChild(d)})}catch(e){diag(e.message)}}async function openTask(id){try{let j=await api('/api/tasks/'+encodeURIComponent(id)+'?repo_path='+encodeURIComponent(repo()));current=j.task;fill(current);loadTasks()}catch(e){diag(e.message)}}function fill(t){$('id').value=t.id||'';$('title').value=t.title||'';$('edit_status').value=t.status||'todo';$('edit_priority').value=t.priority||'';$('model').value=t.model_level||'';$('tags').value=(t.tags||[]).join(', ');$('body').value=t.body||'';$('acceptance').value=(t.acceptance_criteria||[]).join('\n');$('verification').value=(t.verification_plan||[]).join('\n');diag('source: '+(t.projection_source||''))}function payload(){return{repo_path:repo(),id:$('id').value.trim(),title:$('title').value,body:$('body').value,status:$('edit_status').value,priority:$('edit_priority').value,model_level:$('model').value,tags:tags(),acceptance_criteria:lines('acceptance'),verification_plan:lines('verification'),updated_at:current?.updated_at||''}}async function saveTask(){try{let p=payload();let url=p.id&&current?'/api/tasks/'+encodeURIComponent(p.id):'/api/tasks';let j=await api(url,{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify(p)});current=j.task;fill(current);await loadTasks();diag('saved')}catch(e){diag(e.message)}}async function setStatus(){try{if(!current)throw new Error('open a task first');let j=await api('/api/tasks/'+encodeURIComponent(current.id)+'/status',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({repo_path:repo(),status:$('edit_status').value,updated_at:current.updated_at})});current=j.task;fill(current);await loadTasks();diag('status updated')}catch(e){diag(e.message)}}function newTask(){current=null;fill({status:'todo'});diag('')}function diag(s){$('diag').textContent=s}function esc(s){return String(s||'').replace(/[&<>]/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;'}[c]))}const initialRepo=new URLSearchParams(location.search).get('repo_path');if(initialRepo){$('repo').value=initialRepo;loadTasks()}</script></body></html>`
