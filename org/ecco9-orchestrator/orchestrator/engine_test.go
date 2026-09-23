package orchestrator

import (
	"strings"
	"testing"
)

func TestCreateAgentAssignsIDAndLists(t *testing.T) {
	e := NewEngine()
	a, err := e.CreateAgent(&Agent{Name: "scholar", Type: "reflective"})
	if err != nil {
		t.Fatalf("CreateAgent: %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected an assigned agent ID")
	}
	if a.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}

	got, ok := e.GetAgent(a.ID)
	if !ok || got.Name != "scholar" {
		t.Fatalf("GetAgent(%q) = %+v, %v", a.ID, got, ok)
	}
	if _, ok := e.GetAgent("nope"); ok {
		t.Fatal("GetAgent should report missing agents as not-ok")
	}

	agents := e.ListAgents()
	if len(agents) != 1 || agents[0].ID != a.ID {
		t.Fatalf("ListAgents = %v, want the created agent", agents)
	}
}

func TestSubmitTaskRejectsUnknownAgent(t *testing.T) {
	e := NewEngine()
	if _, err := e.SubmitTask(&Task{Type: "cognitive", Input: "x", AgentID: "ghost"}); err == nil {
		t.Fatal("expected error submitting task for unknown agent")
	} else if !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("error should name the unknown agent: %v", err)
	}
}

func TestSubmitTaskLifecycle(t *testing.T) {
	e := NewEngine()
	a, _ := e.CreateAgent(&Agent{Name: "worker", Type: "general"})
	task, err := e.SubmitTask(&Task{Type: "cognitive", Input: "ponder", AgentID: a.ID})
	if err != nil {
		t.Fatalf("SubmitTask: %v", err)
	}
	if task.ID == "" || task.Status != "pending" {
		t.Fatalf("unexpected task: %+v", task)
	}
	if task.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestSubmitWorkflowRegistersTasks(t *testing.T) {
	e := NewEngine()
	w, err := e.SubmitWorkflow(&Workflow{
		Name: "dream-cycle",
		Tasks: []*Task{
			{Type: "sense", Input: "a"},
			{ID: "fixed-task", Type: "integrate", Input: "b"},
		},
	})
	if err != nil {
		t.Fatalf("SubmitWorkflow: %v", err)
	}
	if w.ID == "" {
		t.Fatal("expected an assigned workflow ID")
	}
	if w.Tasks[0].ID == "" {
		t.Fatal("expected workflow task IDs to be assigned")
	}
	if w.Tasks[1].ID != "fixed-task" {
		t.Fatalf("pre-assigned task ID should be preserved, got %q", w.Tasks[1].ID)
	}
	for _, task := range w.Tasks {
		if task.Status != "pending" {
			t.Fatalf("workflow task %q status = %q, want pending", task.ID, task.Status)
		}
	}

	agents, tasks, workflows := e.Stats()
	if agents != 0 || tasks != 2 || workflows != 1 {
		t.Fatalf("Stats = (%d, %d, %d), want (0, 2, 1)", agents, tasks, workflows)
	}
}
