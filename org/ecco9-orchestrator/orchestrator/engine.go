package orchestrator

import (
	"fmt"
	"sync"
	"time"
)

// Agent is a cognitive agent CRD (CognitiveAgent).
type Agent struct {
	ID           string
	Name         string
	Type         string
	Models       []string
	Capabilities []string
	Config       map[string]string
	CreatedAt    time.Time
}

// Task is a unit of work (CognitiveTask).
type Task struct {
	ID        string
	Type      string
	Input     string
	Output    string
	Status    string
	AgentID   string
	CreatedAt time.Time
}

// Workflow is a DAG of tasks (CognitiveWorkflow).
type Workflow struct {
	ID    string
	Name  string
	Tasks []*Task
}

// Engine manages agent/task/workflow lifecycle.
type Engine struct {
	mu        sync.RWMutex
	agents    map[string]*Agent
	tasks     map[string]*Task
	workflows map[string]*Workflow
	seq       uint64
}

// NewEngine constructs an empty orchestration engine.
func NewEngine() *Engine {
	return &Engine{
		agents:    make(map[string]*Agent),
		tasks:     make(map[string]*Task),
		workflows: make(map[string]*Workflow),
	}
}

func (e *Engine) nextID(prefix string) string {
	e.seq++
	return fmt.Sprintf("%s-%d", prefix, e.seq)
}

// CreateAgent registers a cognitive agent.
func (e *Engine) CreateAgent(a *Agent) (*Agent, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if a.ID == "" {
		a.ID = e.nextID("agent")
	}
	a.CreatedAt = time.Now()
	e.agents[a.ID] = a
	return a, nil
}

// GetAgent returns an agent by ID.
func (e *Engine) GetAgent(id string) (*Agent, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	a, ok := e.agents[id]
	return a, ok
}

// ListAgents returns all agents.
func (e *Engine) ListAgents() []*Agent {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]*Agent, 0, len(e.agents))
	for _, a := range e.agents {
		out = append(out, a)
	}
	return out
}

// SubmitTask enqueues a task for an agent.
func (e *Engine) SubmitTask(t *Task) (*Task, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if t.AgentID != "" {
		if _, ok := e.agents[t.AgentID]; !ok {
			return nil, fmt.Errorf("unknown agent %q", t.AgentID)
		}
	}
	if t.ID == "" {
		t.ID = e.nextID("task")
	}
	t.Status = "pending"
	t.CreatedAt = time.Now()
	e.tasks[t.ID] = t
	return t, nil
}

// SubmitWorkflow registers a workflow and its tasks.
func (e *Engine) SubmitWorkflow(w *Workflow) (*Workflow, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if w.ID == "" {
		w.ID = e.nextID("wf")
	}
	for _, t := range w.Tasks {
		if t.ID == "" {
			t.ID = e.nextID("task")
		}
		if t.Status == "" {
			t.Status = "pending"
		}
		t.CreatedAt = time.Now()
		e.tasks[t.ID] = t
	}
	e.workflows[w.ID] = w
	return w, nil
}

// Stats returns engine counts.
func (e *Engine) Stats() (agents, tasks, workflows int) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.agents), len(e.tasks), len(e.workflows)
}
