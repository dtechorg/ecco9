// Package server exposes the OrchestratorService over REST, including the
// echo state thread pool controller endpoints.
package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dtechorg/ecco9-orchestrator/orchestrator"
	"github.com/dtechorg/ecco9-sdk-go/contracts"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the engine and the homeostatic controller.
type Service struct {
	Engine *orchestrator.Engine
	Ctrl   *orchestrator.HomeostaticController
	Health *health.Reporter
}

// New constructs the orchestrator service.
func New() *Service {
	return &Service{
		Engine: orchestrator.NewEngine(),
		Ctrl:   orchestrator.NewHomeostaticController(),
		Health: health.NewReporter(),
	}
}

// Routes returns the HTTP handler.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	mux.HandleFunc("/v1/orchestrator/agents", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			var a orchestrator.Agent
			if err := json.NewDecoder(r.Body).Decode(&a); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			created, _ := s.Engine.CreateAgent(&a)
			writeJSON(w, map[string]any{"id": created.ID})
		case http.MethodGet:
			writeJSON(w, map[string]any{"agents": s.Engine.ListAgents()})
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/v1/orchestrator/tasks", func(w http.ResponseWriter, r *http.Request) {
		var t orchestrator.Task
		if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		created, err := s.Engine.SubmitTask(&t)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"id": created.ID})
	})

	mux.HandleFunc("/v1/orchestrator/workflows", func(w http.ResponseWriter, r *http.Request) {
		var wf orchestrator.Workflow
		if err := json.NewDecoder(r.Body).Decode(&wf); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		created, _ := s.Engine.SubmitWorkflow(&wf)
		writeJSON(w, map[string]any{"id": created.ID})
	})

	// Echo state thread pool controller endpoints.
	mux.HandleFunc("/v1/orchestrator/metrics", func(w http.ResponseWriter, r *http.Request) {
		var m contracts.CognitiveMetrics
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		m.Timestamp = time.Now()
		s.Ctrl.ReportMetrics(m)
		s.Health.SetLoad(s.Ctrl.GlobalLoad())
		writeJSON(w, map[string]any{"accepted": true})
	})

	mux.HandleFunc("/v1/orchestrator/load", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"global_cognitive_load": s.Ctrl.GlobalLoad()})
	})

	mux.HandleFunc("/v1/orchestrator/directives", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"directives": s.Ctrl.Directives()})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
