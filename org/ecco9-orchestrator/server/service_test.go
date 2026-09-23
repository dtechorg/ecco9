package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHealthEndpoint(t *testing.T) {
	s := New()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "identity_coherence") {
		t.Fatalf("healthz missing identity_coherence: %s", rec.Body.String())
	}
}

func TestCreateAndListAgents(t *testing.T) {
	s := New()
	req := httptest.NewRequest(http.MethodPost, "/v1/orchestrator/agents",
		strings.NewReader(`{"Name":"scholar","Type":"reflective"}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("create agent = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"agent-`) {
		t.Fatalf("expected assigned agent id, got %s", rec.Body.String())
	}

	lreq := httptest.NewRequest(http.MethodGet, "/v1/orchestrator/agents", nil)
	lrec := httptest.NewRecorder()
	s.Routes().ServeHTTP(lrec, lreq)
	if !strings.Contains(lrec.Body.String(), "scholar") {
		t.Fatalf("list missing created agent: %s", lrec.Body.String())
	}

	// Unsupported method on the collection is rejected.
	mreq := httptest.NewRequest(http.MethodDelete, "/v1/orchestrator/agents", nil)
	mrec := httptest.NewRecorder()
	s.Routes().ServeHTTP(mrec, mreq)
	if mrec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("DELETE agents = %d, want 405", mrec.Code)
	}
}

func TestSubmitTaskEndpoint(t *testing.T) {
	s := New()
	req := httptest.NewRequest(http.MethodPost, "/v1/orchestrator/tasks",
		strings.NewReader(`{"Type":"cognitive","Input":"ponder"}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("submit task = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"task-`) {
		t.Fatalf("expected assigned task id, got %s", rec.Body.String())
	}
}

func TestSubmitTaskUnknownAgentReturns400(t *testing.T) {
	s := New()
	req := httptest.NewRequest(http.MethodPost, "/v1/orchestrator/tasks",
		strings.NewReader(`{"type":"cognitive","input":"x","AgentID":"ghost"}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown agent = %d, want 400; body=%q", rec.Code, rec.Body.String())
	}
}

func TestSubmitWorkflowEndpoint(t *testing.T) {
	s := New()
	body := `{"Name":"dream-cycle","Tasks":[{"Type":"sense","Input":"a"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/orchestrator/workflows", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("submit workflow = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"id":"wf-`) {
		t.Fatalf("expected assigned workflow id, got %s", rec.Body.String())
	}
}

func TestMetricsLoadAndDirectivesEndpoints(t *testing.T) {
	s := New()

	mreq := httptest.NewRequest(http.MethodPost, "/v1/orchestrator/metrics",
		strings.NewReader(`{"ServiceName":"ecco9-reservoir","InstanceID":"r1","ThoughtQueueDepth":100}`))
	mrec := httptest.NewRecorder()
	s.Routes().ServeHTTP(mrec, mreq)
	if mrec.Code != http.StatusOK || !strings.Contains(mrec.Body.String(), `"accepted":true`) {
		t.Fatalf("metrics = %d; body=%q", mrec.Code, mrec.Body.String())
	}

	lreq := httptest.NewRequest(http.MethodGet, "/v1/orchestrator/load", nil)
	lrec := httptest.NewRecorder()
	s.Routes().ServeHTTP(lrec, lreq)
	if !strings.Contains(lrec.Body.String(), "global_cognitive_load") {
		t.Fatalf("load missing global_cognitive_load: %s", lrec.Body.String())
	}

	dreq := httptest.NewRequest(http.MethodGet, "/v1/orchestrator/directives", nil)
	drec := httptest.NewRecorder()
	s.Routes().ServeHTTP(drec, dreq)
	body := drec.Body.String()
	for _, svc := range []string{"ecco9-reservoir", "ecco9-echobeats", "ecco9-runner"} {
		if !strings.Contains(body, svc) {
			t.Fatalf("directives missing %s: %s", svc, body)
		}
	}
}
