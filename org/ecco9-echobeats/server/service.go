// Package server wires the echobeats core into a REST service exposing the
// EchoBeatsService contract (GetLoopState, StartLoop, StopLoop, StreamSteps).
package server

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/dtechorg/ecco9-echobeats/echobeats"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the 12-step loop, scheduler, and health reporter.
type Service struct {
	Loop      *echobeats.ThreePhase
	Scheduler *echobeats.Scheduler
	Health    *health.Reporter
}

// New constructs the EchoBeats service.
func New() *Service {
	return &Service{
		Loop:      echobeats.NewThreePhase(),
		Scheduler: echobeats.NewScheduler(),
		Health:    health.NewReporter(),
	}
}

// Routes returns the REST adapter for the EchoBeatsService gRPC contract.
// The gRPC server is generated from ecco9-proto; this REST surface keeps the
// service runnable in the offline sandbox and for simple health checks.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// GetLoopState
	mux.HandleFunc("GET /v1/echobeats/loop", func(w http.ResponseWriter, _ *http.Request) {
		step, phase, cycles, steps, running := s.Loop.State()
		s.Health.SetLoad(float64(steps%120) / 120.0)
		writeJSON(w, map[string]any{
			"current_step":     step,
			"current_phase":    phase.String(),
			"cycles_completed": cycles,
			"steps_executed":   steps,
			"running":          running,
		})
	})

	// StartLoop
	mux.HandleFunc("POST /v1/echobeats/loop/start", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CycleFrequencyHz float64 `json:"cycle_frequency_hz"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req) // empty body is valid
		}
		started := s.Loop.Start(req.CycleFrequencyHz)
		s.Health.SetLoad(0.1)
		writeJSON(w, map[string]any{"started": started})
	})

	// StopLoop
	mux.HandleFunc("POST /v1/echobeats/loop/stop", func(w http.ResponseWriter, _ *http.Request) {
		stopped := s.Loop.Stop()
		s.Health.SetLoad(0)
		writeJSON(w, map[string]any{"stopped": stopped})
	})

	// StreamSteps as Server-Sent Events (REST form of the server stream).
	mux.HandleFunc("GET /v1/echobeats/steps", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		ch := s.Loop.Subscribe()
		defer s.Loop.Unsubscribe(ch)

		for {
			select {
			case <-r.Context().Done():
				return
			case exec := <-ch:
				data, _ := json.Marshal(map[string]any{"step": exec})
				fmt.Fprintf(w, "data: %s\n\n", data)
				flusher.Flush()
			}
		}
	})

	// Loop metrics (three concurrent engines).
	mux.HandleFunc("GET /v1/echobeats/metrics", func(w http.ResponseWriter, _ *http.Request) {
		eng := s.Loop.EngineStats()
		scheduled, processed, pending := s.Scheduler.Stats()
		writeJSON(w, map[string]any{
			"engine1_tasks":       eng[0],
			"engine2_tasks":       eng[1],
			"engine3_tasks":       eng[2],
			"events_scheduled":    scheduled,
			"events_processed":    processed,
			"events_pending":      pending,
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
