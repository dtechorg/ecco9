// Package server wires the metacognitive monitor into a network service
// exposing the MetaCogService contract.
package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dtechorg/ecco9-metacog/metacog"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the metacognitive monitor and health reporter.
type Service struct {
	Monitor *metacog.Monitor
	Health  *health.Reporter
}

// New constructs the metacog service.
func New() *Service {
	return &Service{
		Monitor: metacog.NewMonitor(),
		Health:  health.NewReporter(),
	}
}

// HTTP handlers (REST adapter for the gRPC contracts defined in the proto).
// The gRPC server is generated from ecco9-proto; this REST surface keeps the
// service runnable in the offline sandbox and for simple health checks.

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// MetaCogService.Monitor
	mux.HandleFunc("POST /v1/metacog/monitor", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Process struct {
				ID           string  `json:"id"`
				Type         string  `json:"type"`
				Service      string  `json:"service"`
				QualityScore float64 `json:"quality_score"`
				Progress     float64 `json:"progress"`
				Completed    bool    `json:"completed"`
				Success      bool    `json:"success"`
				StartedAtMs  int64   `json:"started_at_unix_ms"`
			} `json:"process"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		started := time.Now()
		if req.Process.StartedAtMs > 0 {
			started = time.UnixMilli(req.Process.StartedAtMs)
		}
		id, tracked := s.Monitor.Track(metacog.CognitiveProcess{
			ID: req.Process.ID, Name: req.Process.Type, Type: req.Process.Type,
			Service: req.Process.Service, StartTime: started,
			Progress: req.Process.Progress, QualityScore: req.Process.QualityScore,
			Completed: req.Process.Completed, Success: req.Process.Success,
		})
		writeJSON(w, map[string]any{"tracked": tracked, "process_id": id})
	})

	// MetaCogService.SelectStrategy
	mux.HandleFunc("POST /v1/metacog/strategy", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ProblemContext      string   `json:"problem_context"`
			CandidateStrategies []string `json:"candidate_strategies"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		selected, confidence := s.Monitor.SelectStrategy(req.ProblemContext, req.CandidateStrategies)
		writeJSON(w, map[string]any{
			"selected_strategy": selected,
			"confidence":        confidence,
		})
	})

	// MetaCogService.ListLearningGaps
	mux.HandleFunc("GET /v1/metacog/learning_gaps", func(w http.ResponseWriter, _ *http.Request) {
		gaps := s.Monitor.LearningGaps()
		out := make([]map[string]any, 0, len(gaps))
		for _, g := range gaps {
			out = append(out, map[string]any{
				"id": g.ID, "domain": g.Domain,
				"description": g.Description, "severity": g.Severity,
			})
		}
		writeJSON(w, map[string]any{"gaps": out})
	})

	// MetaCogService.AssessSystem
	mux.HandleFunc("GET /v1/metacog/assessment", func(w http.ResponseWriter, _ *http.Request) {
		a := s.Monitor.AssessSystem()
		s.Health.SetLoad(a.GlobalCognitiveLoad)
		s.Health.SetCoherence(a.IdentityCoherence)
		writeJSON(w, map[string]any{
			"global_cognitive_load": a.GlobalCognitiveLoad,
			"identity_coherence":    a.IdentityCoherence,
			"recommendations":       a.Recommendations,
		})
	})

	// Auxiliary endpoints (not part of the proto): decision recording and
	// quality assessment, gap registration, and self-awareness status.
	mux.HandleFunc("POST /v1/metacog/decisions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Context    string  `json:"context"`
			Chosen     string  `json:"chosen"`
			Rationale  string  `json:"rationale"`
			Confidence float64 `json:"confidence"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id := s.Monitor.RecordDecision(req.Context, req.Chosen, req.Rationale, req.Confidence)
		writeJSON(w, map[string]any{"decision_id": id})
	})

	mux.HandleFunc("POST /v1/metacog/decisions/assess", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			DecisionID      string  `json:"decision_id"`
			Success         bool    `json:"success"`
			ExpectedBenefit float64 `json:"expected_benefit"`
			ActualBenefit   float64 `json:"actual_benefit"`
			SideEffects     int     `json:"side_effects"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		a := s.Monitor.AssessDecision(req.DecisionID, req.Success, req.ExpectedBenefit, req.ActualBenefit, req.SideEffects)
		if a == nil {
			http.Error(w, "decision not found", http.StatusNotFound)
			return
		}
		writeJSON(w, a)
	})

	mux.HandleFunc("POST /v1/metacog/learning_gaps", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Domain      string  `json:"domain"`
			Description string  `json:"description"`
			Severity    float64 `json:"severity"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"id": s.Monitor.AddGap(req.Domain, req.Description, req.Severity)})
	})

	mux.HandleFunc("GET /v1/metacog/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, s.Monitor.Status())
	})

	// Training quality validation (pipeline step 4): evaluate a reservoir
	// checkpoint's held-out prediction error and record the outcome.
	mux.HandleFunc("POST /v1/metacog/training_validation", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CheckpointVersion uint64  `json:"checkpoint_version"`
			HeldOutMSE        float64 `json:"held_out_mse"`
			TrainError        float64 `json:"train_error"`
			SamplesTrained    uint64  `json:"samples_trained"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		v := s.Monitor.EvaluateTrainingQuality(req.CheckpointVersion, req.HeldOutMSE, req.TrainError, req.SamplesTrained)
		writeJSON(w, map[string]any{
			"checkpoint_version": v.CheckpointVersion,
			"held_out_mse":       v.HeldOutMSE,
			"quality":            v.Quality,
			"passed":             v.Passed,
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
