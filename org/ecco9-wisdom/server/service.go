// Package server wires the wisdom core into a network service exposing
// the WisdomService contract.
package server

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/dtechorg/ecco9-sdk-go/health"
	"github.com/dtechorg/ecco9-wisdom/wisdom"
)

// Service bundles the wisdom tracker, goal orchestrator, interest
// generator, and wisdom application engine.
type Service struct {
	Metrics  *wisdom.SevenDimensionalWisdom
	Goals    *wisdom.GoalOrchestrator
	Interest *wisdom.InterestDrivenGenerator
	Engine   *wisdom.ApplicationEngine
	Health   *health.Reporter
}

// New constructs the wisdom service.
func New() *Service {
	return &Service{
		Metrics:  wisdom.NewSevenDimensionalWisdom(),
		Goals:    wisdom.NewGoalOrchestrator(),
		Interest: wisdom.NewInterestDrivenGenerator(),
		Engine:   wisdom.NewApplicationEngine(),
		Health:   health.NewReporter(),
	}
}

// HTTP handlers (REST adapter for the gRPC contracts defined in the proto).
// The gRPC server is generated from ecco9-proto; this REST surface keeps the
// service runnable in the offline sandbox and for simple health checks.

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// WisdomService.CreateGoal
	mux.HandleFunc("POST /v1/wisdom/goals", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Goal goalJSON `json:"goal"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := s.Goals.CreateGoal(fromJSONGoal(req.Goal))
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(w, map[string]any{"id": id})
	})

	// WisdomService.ListGoals (type_filter as query param, 0 = all)
	mux.HandleFunc("GET /v1/wisdom/goals", func(w http.ResponseWriter, r *http.Request) {
		typeFilter := wisdom.GoalType(0)
		if v := r.URL.Query().Get("type_filter"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				typeFilter = wisdom.GoalType(n)
			}
		}
		goals := s.Goals.ListGoals(typeFilter)
		out := make([]goalJSON, 0, len(goals))
		for _, g := range goals {
			out = append(out, toJSONGoal(g))
		}
		writeJSON(w, map[string]any{"goals": out})
	})

	// WisdomService.GetWisdom
	mux.HandleFunc("GET /v1/wisdom/assessment", func(w http.ResponseWriter, _ *http.Request) {
		a := s.Metrics.Assess()
		s.Health.SetCoherence(a.Coherence)
		writeJSON(w, map[string]any{
			"assessment": map[string]any{
				"dimensions":          a.Dimensions,
				"dimension_names":     wisdom.DimensionNames,
				"overall":             a.Overall,
				"coherence":           a.Coherence,
				"evolution_rate":      a.Evolution,
				"assessed_at_unix_ms": a.AssessedAt.UnixMilli(),
			},
		})
	})

	// WisdomService.ApplyWisdom
	mux.HandleFunc("POST /v1/wisdom/apply", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Context    string `json:"context"`
			MaxEntries int32  `json:"max_entries"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		topK := int(req.MaxEntries)
		if topK <= 0 {
			topK = 3
		}
		matches := s.Engine.FindRelevantWisdom(req.Context, topK)
		entries := make([]string, 0, len(matches))
		applicability := make([]float64, 0, len(matches))
		for _, m := range matches {
			entries = append(entries, m.Entry.Content)
			applicability = append(applicability, m.Applicability)
		}
		writeJSON(w, map[string]any{
			"wisdom_entries": entries,
			"applicability":  applicability,
		})
	})

	// Auxiliary endpoints (not part of the proto): dimension updates,
	// interest engagement, and goal progress.
	mux.HandleFunc("POST /v1/wisdom/update", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			GraphDepth       float64 `json:"graph_depth"`
			GraphBreadth     float64 `json:"graph_breadth"`
			EdgeDensity      float64 `json:"edge_density"`
			SkillProficiency float64 `json:"skill_proficiency"`
			AarCoherence     float64 `json:"aar_coherence"`
			MoralityScore    float64 `json:"morality_score"`
			GoalTimeHorizon  float64 `json:"goal_time_horizon"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Metrics.Update(req.GraphDepth, req.GraphBreadth, req.EdgeDensity,
			req.SkillProficiency, req.AarCoherence, req.MoralityScore, req.GoalTimeHorizon)
		a := s.Metrics.Assess()
		s.Health.SetCoherence(a.Coherence)
		writeJSON(w, map[string]any{"overall": a.Overall, "coherence": a.Coherence})
	})

	mux.HandleFunc("POST /v1/wisdom/interest", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Topic    string  `json:"topic"`
			Strength float64 `json:"strength"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Interest.RecordEngagement(req.Topic, req.Strength)
		strong := s.Interest.StrongestInterests(3)
		topics := make([]string, 0, len(strong))
		for _, p := range strong {
			topics = append(topics, p.Topic)
		}
		writeJSON(w, map[string]any{"strongest_interests": topics})
	})

	mux.HandleFunc("POST /v1/wisdom/goals/advance", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID    string  `json:"id"`
			Delta float64 `json:"delta"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"advanced": s.Goals.AdvanceGoal(req.ID, req.Delta)})
	})

	return mux
}

type goalJSON struct {
	ID          string  `json:"id"`
	Type        int     `json:"type"`
	Description string  `json:"description"`
	Priority    float64 `json:"priority"`
	Progress    float64 `json:"progress"`
	CreatedAtMs int64   `json:"created_at_unix_ms"`
}

func fromJSONGoal(g goalJSON) wisdom.Goal {
	created := time.Now()
	if g.CreatedAtMs > 0 {
		created = time.UnixMilli(g.CreatedAtMs)
	}
	return wisdom.Goal{
		ID: g.ID, Type: wisdom.GoalType(g.Type), Description: g.Description,
		Priority: g.Priority, Progress: g.Progress, CreatedAt: created,
	}
}

func toJSONGoal(g *wisdom.Goal) goalJSON {
	return goalJSON{
		ID: g.ID, Type: int(g.Type), Description: g.Description,
		Priority: g.Priority, Progress: g.Progress, CreatedAtMs: g.CreatedAt.UnixMilli(),
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
