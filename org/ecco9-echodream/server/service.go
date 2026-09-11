// Package server wires the EchoDream system into a network service
// exposing the EchoDreamService contract.
package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/dtechorg/ecco9-echodream/echodream"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the dream system and health reporter.
type Service struct {
	Dream  *echodream.System
	Health *health.Reporter
}

// New constructs the echodream service.
func New() *Service {
	return &Service{
		Dream:  echodream.New(),
		Health: health.NewReporter(),
	}
}

// HTTP handlers (REST adapter for the gRPC contracts defined in the proto).
// The gRPC server is generated from ecco9-proto; this REST surface keeps the
// service runnable in the offline sandbox and for simple health checks.

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// EchoDreamService.StartDreamCycle
	mux.HandleFunc("POST /v1/echodream/cycles", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			MaxMemories int32 `json:"max_memories"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cycleID := s.Dream.StartDreamCycle(int(req.MaxMemories))
		writeJSON(w, map[string]any{"cycle_id": cycleID})
	})

	// EchoDreamService.GetDreamState
	mux.HandleFunc("GET /v1/echodream/state", func(w http.ResponseWriter, _ *http.Request) {
		st := s.Dream.GetState()
		s.Health.SetLoad(boolLoad(st.Dreaming))
		writeJSON(w, map[string]any{
			"dreaming":           st.Dreaming,
			"phase":              int(st.Phase),
			"phase_name":         st.Phase.String(),
			"cycles_completed":   st.CyclesCompleted,
			"memories_processed": st.MemoriesProcessed,
			"wisdom_extracted":   st.WisdomExtracted,
		})
	})

	// EchoDreamService.SubmitMemory
	mux.HandleFunc("POST /v1/echodream/memories", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Memory struct {
				ID             string  `json:"id"`
				TimestampMs    int64   `json:"timestamp_unix_ms"`
				Content        string  `json:"content"`
				Importance     float64 `json:"importance"`
				Consolidated   bool    `json:"consolidated"`
			} `json:"memory"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ts := time.Now()
		if req.Memory.TimestampMs > 0 {
			ts = time.UnixMilli(req.Memory.TimestampMs)
		}
		_, queued := s.Dream.SubmitMemory(echodream.EpisodicMemory{
			ID: req.Memory.ID, Timestamp: ts, Content: req.Memory.Content,
			Importance: req.Memory.Importance, Consolidated: req.Memory.Consolidated,
		})
		writeJSON(w, map[string]any{"queued": queued})
	})

	// Auxiliary endpoints (not part of the proto): extracted wisdom,
	// consolidated knowledge, and dream history.
	mux.HandleFunc("GET /v1/echodream/wisdom", func(w http.ResponseWriter, _ *http.Request) {
		insights := s.Dream.Wisdom()
		out := make([]map[string]any, 0, len(insights))
		for _, wi := range insights {
			out = append(out, map[string]any{
				"id": wi.ID, "insight": wi.Insight,
				"depth": wi.Depth, "applicability": wi.Applicability,
			})
		}
		writeJSON(w, map[string]any{"wisdom_insights": out})
	})

	mux.HandleFunc("GET /v1/echodream/knowledge", func(w http.ResponseWriter, _ *http.Request) {
		items := s.Dream.Knowledge()
		out := make([]map[string]any, 0, len(items))
		for _, k := range items {
			out = append(out, map[string]any{
				"id": k.ID, "content": k.Content,
				"source_memory_ids": k.SourceMemoryIDs, "confidence": k.Confidence,
			})
		}
		writeJSON(w, map[string]any{"knowledge_items": out})
	})

	mux.HandleFunc("GET /v1/echodream/dreams", func(w http.ResponseWriter, _ *http.Request) {
		dreams := s.Dream.Dreams()
		out := make([]map[string]any, 0, len(dreams))
		for _, d := range dreams {
			out = append(out, map[string]any{
				"cycle_id": d.CycleID, "consolidations": d.Consolidations,
				"patterns": d.Patterns, "wisdom_count": len(d.Wisdom),
				"narrative": d.Narrative,
			})
		}
		writeJSON(w, map[string]any{"dreams": out})
	})

	return mux
}

func boolLoad(dreaming bool) float64 {
	if dreaming {
		return 1.0
	}
	return 0.0
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
