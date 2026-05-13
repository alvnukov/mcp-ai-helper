package mcp

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/zol/mcp-ai-helper/internal/tasks"
)

// TaskGraphRequest is the input for building a task graph.
type TaskGraphRequest struct {
	RepoPath    string `json:"repo_path"`
	FocusTaskID string `json:"focus_task_id,omitempty"`
	MaxNodes    int    `json:"max_nodes,omitempty"`
	MaxBytes    int    `json:"max_bytes,omitempty"`
}

// TaskGraph is the output graph structure.
type TaskGraph struct {
	Nodes      []TaskGraphNode      `json:"nodes"`
	Edges      []TaskGraphEdge      `json:"edges"`
	Provenance TaskGraphProvenance   `json:"provenance"`
	Truncated  *TaskGraphTruncation `json:"truncated,omitempty"`
}

// TaskGraphNode is a single node in the task graph.
type TaskGraphNode struct {
	ID         string   `json:"id"`
	Status     string   `json:"status"`
	Title      string   `json:"title"`
	Priority   string   `json:"priority,omitempty"`
	ModelLevel string   `json:"model_level,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	ParentID   string   `json:"parent_id,omitempty"`
	TaskType   string   `json:"task_type,omitempty"`
}

// TaskGraphEdge is a directed edge between two nodes.
type TaskGraphEdge struct {
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Provenance string `json:"provenance"` // "explicit" or "inferred"
}

// TaskGraphProvenance describes the data source and relationship semantics.
type TaskGraphProvenance struct {
	Source    string            `json:"source"`
	EdgeKinds map[string]string `json:"edge_kinds"`
}

// TaskGraphTruncation records what was omitted due to limits.
type TaskGraphTruncation struct {
	OmittedNodes int    `json:"omitted_nodes"`
	OmittedEdges int    `json:"omitted_edges"`
	Reason       string `json:"reason,omitempty"`
}

const (
	defaultMaxNodes = 50
	defaultMaxBytes = 8192
	edgeKindParent  = "parent_child"
)

func defaultGraphLimits(req TaskGraphRequest) (int, int) {
	maxNodes := req.MaxNodes
	if maxNodes <= 0 {
		maxNodes = defaultMaxNodes
	}
	maxBytes := req.MaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultMaxBytes
	}
	return maxNodes, maxBytes
}

// BuildTaskGraph constructs a task graph from canonical task data.
func BuildTaskGraph(all []tasks.Task, req TaskGraphRequest) (TaskGraph, error) {
	maxNodes, maxBytes := defaultGraphLimits(req)

	if req.FocusTaskID != "" {
		return buildFocusedGraph(all, req.FocusTaskID, maxNodes, maxBytes)
	}
	return buildFullGraph(all, maxNodes, maxBytes)
}

func buildFullGraph(all []tasks.Task, maxNodes, maxBytes int) (TaskGraph, error) {
	nodes := make([]TaskGraphNode, 0, len(all))
	edges := make([]TaskGraphEdge, 0)

	taskMap := make(map[string]tasks.Task, len(all))
	for _, t := range all {
		taskMap[t.ID] = t
	}

	truncatedByNodes := false
	omittedNodes := 0
	omittedEdges := 0

	for _, t := range all {
		if len(nodes) >= maxNodes {
			omittedNodes++
			if t.ParentID != "" {
				if _, exists := taskMap[t.ParentID]; exists {
					omittedEdges++
				}
			}
			truncatedByNodes = true
			continue
		}
		nodes = append(nodes, taskToNode(t))
		if t.ParentID != "" {
			if _, exists := taskMap[t.ParentID]; exists {
				edges = append(edges, TaskGraphEdge{
					From:       t.ParentID,
					To:         t.ID,
					Kind:       edgeKindParent,
					Provenance: "explicit",
				})
			}
		}
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	result := TaskGraph{
		Nodes: nodes,
		Edges: edges,
		Provenance: TaskGraphProvenance{
			Source: "lean_registry",
			EdgeKinds: map[string]string{
				edgeKindParent: "explicit parent-child relationship from task parent_id field",
			},
		},
	}

	if truncatedByNodes {
		result.Truncated = &TaskGraphTruncation{
			OmittedNodes: omittedNodes,
			OmittedEdges: omittedEdges,
			Reason:       fmt.Sprintf("max_nodes limit reached (%d)", maxNodes),
		}
	}

	result = enforceMaxBytes(result, maxBytes, "")
	return result, nil
}

func buildFocusedGraph(all []tasks.Task, focusID string, maxNodes, maxBytes int) (TaskGraph, error) {
	taskMap := make(map[string]tasks.Task, len(all))
	for i := range all {
		taskMap[all[i].ID] = all[i]
	}

	var focusTask *tasks.Task
	for i := range all {
		if all[i].ID == focusID {
			t := all[i]
			focusTask = &t
			break
		}
	}
	if focusTask == nil {
		return TaskGraph{}, fmt.Errorf("focus task %q not found", focusID)
	}

	relevant := make(map[string]bool)
	relevant[focusTask.ID] = true

	// Ancestors (parent chain)
	parentID := focusTask.ParentID
	for parentID != "" {
		relevant[parentID] = true
		parent, ok := taskMap[parentID]
		if !ok {
			break
		}
		parentID = parent.ParentID
	}

	// Children of focus task
	for _, t := range all {
		if t.ParentID == focusTask.ID {
			relevant[t.ID] = true
		}
	}

	// Siblings (tasks sharing the same parent as focus)
	if focusTask.ParentID != "" {
		for _, t := range all {
			if t.ParentID == focusTask.ParentID && t.ID != focusTask.ID {
				relevant[t.ID] = true
			}
		}
	}

	// Build nodes: focus task always first, then remaining relevant tasks
	nodes := make([]TaskGraphNode, 0)
	nodes = append(nodes, taskToNode(*focusTask))
	for _, t := range all {
		if t.ID == focusTask.ID {
			continue
		}
		if relevant[t.ID] {
			if len(nodes) >= maxNodes {
				break
			}
			nodes = append(nodes, taskToNode(t))
		}
	}

	edges := make([]TaskGraphEdge, 0)
	for _, t := range all {
		if !relevant[t.ID] || t.ParentID == "" {
			continue
		}
		if relevant[t.ParentID] {
			edges = append(edges, TaskGraphEdge{
				From:       t.ParentID,
				To:         t.ID,
				Kind:       edgeKindParent,
				Provenance: "explicit",
			})
		}
	}

	sort.Slice(nodes[1:], func(i, j int) bool { return nodes[1+i].ID < nodes[1+j].ID })
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}
		return edges[i].To < edges[j].To
	})

	result := TaskGraph{
		Nodes: nodes,
		Edges: edges,
		Provenance: TaskGraphProvenance{
			Source: "lean_registry",
			EdgeKinds: map[string]string{
				edgeKindParent: "explicit parent-child relationship from task parent_id field",
			},
		},
	}

	totalRelevant := len(relevant)
	if len(nodes) < totalRelevant {
		result.Truncated = &TaskGraphTruncation{
			OmittedNodes: totalRelevant - len(nodes),
			Reason:       fmt.Sprintf("max_nodes limit reached (%d)", maxNodes),
		}
	}

	result = enforceMaxBytes(result, maxBytes, focusID)
	return result, nil
}

func taskToNode(t tasks.Task) TaskGraphNode {
	node := TaskGraphNode{
		ID:         t.ID,
		Status:     t.Status,
		Title:      t.Title,
		Priority:   t.Priority,
		ModelLevel: t.ModelLevel,
		ParentID:   t.ParentID,
		TaskType:   t.TaskType,
	}
	if len(t.Tags) > 0 {
		node.Tags = make([]string, len(t.Tags))
		copy(node.Tags, t.Tags)
	}
	return node
}

// validateTaskGraphRequest checks request arguments before graph construction.
func validateTaskGraphRequest(req TaskGraphRequest) error {
	if strings.TrimSpace(req.RepoPath) == "" {
		return fmt.Errorf("repo_path is required")
	}
	if req.MaxNodes < 0 {
		return fmt.Errorf("max_nodes must be >= 0, got %d", req.MaxNodes)
	}
	if req.MaxBytes < 0 {
		return fmt.Errorf("max_bytes must be >= 0, got %d", req.MaxBytes)
	}
	return nil
}

// enforceMaxBytes reduces the graph if its JSON serialization exceeds maxBytes.
// In focused mode, the focus task is never removed.
func enforceMaxBytes(g TaskGraph, maxBytes int, protectID string) TaskGraph {
	if maxBytes <= 0 {
		return g
	}
	data, err := json.Marshal(g)
	if err != nil || len(data) <= maxBytes {
		return g
	}

	omittedByBytes := 0
	omittedEdgesByBytes := 0
	for len(g.Nodes) > 0 {
		last := len(g.Nodes) - 1
		if protectID != "" && g.Nodes[last].ID == protectID {
			// Cannot remove focus task; swap with previous and remove that
			if last == 0 {
				break
			}
			g.Nodes[last-1], g.Nodes[last] = g.Nodes[last], g.Nodes[last-1]
			last--
		}
		removedID := g.Nodes[last].ID
		g.Nodes = g.Nodes[:last]
		omittedByBytes++

		// Remove edges referencing the removed node
		filtered := make([]TaskGraphEdge, 0, len(g.Edges))
		for _, e := range g.Edges {
			if e.From == removedID || e.To == removedID {
				omittedEdgesByBytes++
				continue
			}
			filtered = append(filtered, e)
		}
		g.Edges = filtered

		data, err = json.Marshal(g)
		if err == nil && len(data) <= maxBytes {
			break
		}
	}

	// Merge with existing truncation
	existingOmittedNodes := 0
	existingOmittedEdges := 0
	existingReason := ""
	if g.Truncated != nil {
		existingOmittedNodes = g.Truncated.OmittedNodes
		existingOmittedEdges = g.Truncated.OmittedEdges
		existingReason = g.Truncated.Reason + "; "
	}
	if omittedByBytes > 0 || omittedEdgesByBytes > 0 {
		g.Truncated = &TaskGraphTruncation{
			OmittedNodes: existingOmittedNodes + omittedByBytes,
			OmittedEdges: existingOmittedEdges + omittedEdgesByBytes,
			Reason:       existingReason + fmt.Sprintf("max_bytes limit reached (%d)", maxBytes),
		}
	}
	return g
}
