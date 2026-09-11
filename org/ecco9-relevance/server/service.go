// Package server wires the relevance engine into a network service
// exposing the RelevanceService contract.
package server

import (
	"encoding/json"
	"net/http"

	"github.com/dtechorg/ecco9-relevance/relevance"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the relevance engine and health reporter.
type Service struct {
	Engine *relevance.Engine
	Health *health.Reporter
}

// New constructs the relevance service.
func New() *Service {
	return &Service{
		Engine: relevance.NewEngine(),
		Health: health.NewReporter(),
	}
}

// HTTP handlers (REST adapter for the gRPC contracts defined in the proto).
// The gRPC server is generated from ecco9-proto; this REST surface keeps the
// service runnable in the offline sandbox and for simple health checks.

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// RelevanceService.Score
	mux.HandleFunc("POST /v1/relevance/score", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			EventID    string            `json:"event_id"`
			EventTopic string            `json:"event_topic"`
			Payload    string            `json:"payload"`
			Context    map[string]string `json:"context"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		score := s.Engine.RealizeRelevance(req.EventTopic, req.Context)
		s.Health.SetCoherence(s.Engine.Coherence())
		writeJSON(w, map[string]any{
			"event_id":   req.EventID,
			"relevance":  score.Relevance,
			"salience":   score.Salience,
			"affordance": score.Affordance,
			"facets":     score.Facets,
		})
	})

	// RelevanceService.ScoreStream (batched REST analog of the bidi stream:
	// scores each request in order and returns the ordered responses).
	mux.HandleFunc("POST /v1/relevance/score_stream", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Events []struct {
				EventID    string            `json:"event_id"`
				EventTopic string            `json:"event_topic"`
				Payload    string            `json:"payload"`
				Context    map[string]string `json:"context"`
			} `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		responses := make([]map[string]any, 0, len(req.Events))
		for _, ev := range req.Events {
			score := s.Engine.RealizeRelevance(ev.EventTopic, ev.Context)
			responses = append(responses, map[string]any{
				"event_id":   ev.EventID,
				"relevance":  score.Relevance,
				"salience":   score.Salience,
				"affordance": score.Affordance,
				"facets":     score.Facets,
			})
		}
		s.Health.SetCoherence(s.Engine.Coherence())
		writeJSON(w, map[string]any{"responses": responses})
	})

	// Engine introspection and learning endpoints (not part of the proto).
	mux.HandleFunc("GET /v1/relevance/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, s.Engine.Status())
	})

	mux.HandleFunc("POST /v1/relevance/experience", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Feedback float64 `json:"feedback"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Engine.UpdateFromExperience(req.Feedback)
		s.Health.SetCoherence(s.Engine.Coherence())
		writeJSON(w, map[string]any{"coherence": s.Engine.Coherence()})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
