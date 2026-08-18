package pipeline

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// A workflow started from an MCP request must keep running when that request
// times out or disconnects: the request context used to be handed straight to
// RunWorkflow, so a client timeout cancelled every step — the workflow could
// finish its commit and still leave the caller with no result. Runs are now
// detached onto a background context and recorded here, addressable by
// workflow_id for the life of the process.

// WorkflowRun is the durable record of one detached workflow execution.
type WorkflowRun struct {
	WorkflowID string          `json:"workflow_id"`
	RepoPath   string          `json:"repo_path"`
	Status     string          `json:"status"`
	StartedAt  time.Time       `json:"started_at"`
	FinishedAt *time.Time      `json:"finished_at,omitempty"`
	Result     *WorkflowResult `json:"result,omitempty"`
	Error      string          `json:"error,omitempty"`
}

// The wait budget must stay well under MCP client timeouts — common harnesses
// cut a call at ~30s — so the workflow action blocks for at most 25s and
// answers running + workflow_id past that.
const (
	defaultWorkflowWaitSeconds = 10
	maxWorkflowWaitSeconds     = 25
	workflowRegistryCapacity   = 128
)

// WorkflowWaitBudget clamps a caller-requested wait in seconds to the range
// the workflow action is allowed to block for.
func WorkflowWaitBudget(seconds int) time.Duration {
	if seconds <= 0 {
		seconds = defaultWorkflowWaitSeconds
	}
	if seconds > maxWorkflowWaitSeconds {
		seconds = maxWorkflowWaitSeconds
	}
	return time.Duration(seconds) * time.Second
}

// WorkflowRegistry tracks detached workflow runs. The MCP layer resolves a
// fresh Runner per call when repo-local config applies, so run records must
// live outside any single Runner.
type WorkflowRegistry struct {
	mu    sync.Mutex
	runs  map[string]*workflowRunRecord
	order []string
}

type workflowRunRecord struct {
	run  WorkflowRun
	done chan struct{}
}

// NewWorkflowRegistry creates an empty registry.
func NewWorkflowRegistry() *WorkflowRegistry {
	return &WorkflowRegistry{runs: map[string]*workflowRunRecord{}}
}

var defaultWorkflowRegistry = NewWorkflowRegistry()

// DefaultWorkflowRegistry returns the process-wide registry the MCP run tool
// shares across per-request Runner instances.
func DefaultWorkflowRegistry() *WorkflowRegistry { return defaultWorkflowRegistry }

func newWorkflowID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("wf-%d", time.Now().UnixNano())
	}
	return "wf-" + hex.EncodeToString(buf)
}

// Start executes req on a background context and returns the initial running
// record at once. The caller's context is deliberately not accepted: its
// cancellation must never reach workflow steps.
func (r *WorkflowRegistry) Start(runner *Runner, req WorkflowRequest) WorkflowRun {
	id := newWorkflowID()
	rec := &workflowRunRecord{
		run: WorkflowRun{
			WorkflowID: id,
			RepoPath:   req.RepoPath,
			Status:     "running",
			StartedAt:  time.Now().UTC(),
		},
		done: make(chan struct{}),
	}
	r.mu.Lock()
	r.runs[id] = rec
	r.order = append(r.order, id)
	r.pruneLocked()
	r.mu.Unlock()

	go func() {
		defer close(rec.done)
		result, err := runner.RunWorkflow(context.Background(), req)
		finished := time.Now().UTC()
		r.mu.Lock()
		defer r.mu.Unlock()
		rec.run.FinishedAt = &finished
		if err != nil {
			rec.run.Status = "error"
			rec.run.Error = err.Error()
			return
		}
		rec.run.Status = result.Status
		rec.run.Result = &result
	}()

	return rec.run
}

// Get returns a snapshot of the current record for id.
func (r *WorkflowRegistry) Get(id string) (WorkflowRun, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rec, ok := r.runs[id]
	if !ok {
		return WorkflowRun{}, false
	}
	return rec.run, true
}

// WaitFor returns a snapshot of the record for id, blocking up to timeout for
// the run to finish; a non-positive timeout returns the snapshot immediately.
func (r *WorkflowRegistry) WaitFor(id string, timeout time.Duration) (WorkflowRun, bool) {
	r.mu.Lock()
	rec, ok := r.runs[id]
	r.mu.Unlock()
	if !ok {
		return WorkflowRun{}, false
	}
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		select {
		case <-rec.done:
			timer.Stop()
		case <-timer.C:
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return rec.run, true
}

// pruneLocked drops the oldest finished records past the capacity; a record
// that is still executing is never dropped.
func (r *WorkflowRegistry) pruneLocked() {
	for len(r.order) > workflowRegistryCapacity {
		id := r.order[0]
		rec, ok := r.runs[id]
		if !ok || rec.run.FinishedAt == nil {
			return
		}
		r.order = r.order[1:]
		delete(r.runs, id)
	}
}
