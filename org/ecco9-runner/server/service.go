// Package server wires the runner core into a REST service mirroring the
// RunnerService proto contract (LoadModel, UnloadModel, Generate).
package server

import (
	"encoding/json"
	"net/http"

	"github.com/dtechorg/ecco9-runner/runner"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the registry, backend, and job queue with health.
type Service struct {
	Registry *runner.Registry
	Backend  runner.Backend
	Queue    *runner.Queue
	Health   *health.Reporter
}

// New constructs the service with the stub inference backend.
func New() *Service {
	reg := runner.NewRegistry()
	backend := &runner.StubBackend{}
	return &Service{
		Registry: reg,
		Backend:  backend,
		Queue:    runner.NewQueue(4, 64, backend, reg),
		Health:   health.NewReporter(),
	}
}

// Routes returns the REST adapter for the gRPC contracts defined in
// ecco9/llm/v1/llm.proto (RunnerService).
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// RunnerService.LoadModel
	mux.HandleFunc("POST /v1/runner/models", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ModelPath   string `json:"model_path"`
			GPULayers   int32  `json:"gpu_layers"`
			ContextSize int32  `json:"context_size"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Health.SetLoad(0.9)
		m, err := s.Registry.Load(runner.LoadParams{
			ModelPath:   req.ModelPath,
			GPULayers:   req.GPULayers,
			ContextSize: req.ContextSize,
		}, s.Backend)
		s.Health.SetLoad(0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusConflict)
			return
		}
		writeJSON(w, map[string]any{"model_id": m.ID, "status": m.Status.String()})
	})

	// RunnerService.UnloadModel
	mux.HandleFunc("DELETE /v1/runner/models/{id}", func(w http.ResponseWriter, r *http.Request) {
		ok := s.Registry.Unload(r.PathValue("id"), s.Backend)
		writeJSON(w, map[string]any{"unloaded": ok})
	})

	// Model inventory (control-plane introspection).
	mux.HandleFunc("GET /v1/runner/models", func(w http.ResponseWriter, _ *http.Request) {
		models := s.Registry.List()
		out := make([]map[string]any, 0, len(models))
		for _, m := range models {
			out = append(out, map[string]any{
				"model_id":     m.ID,
				"path":         m.Path,
				"gpu_layers":   m.GPULayers,
				"context_size": m.ContextSize,
				"status":       m.Status.String(),
				"loaded_at":    m.LoadedAt,
				"kv_util":      m.Cache.Utilization(),
			})
		}
		writeJSON(w, map[string]any{"models": out})
	})

	// RunnerService.Generate (streamed as NDJSON, one token per line).
	mux.HandleFunc("POST /v1/runner/generate", func(w http.ResponseWriter, r *http.Request) {
		var req runner.GenerateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		job := &runner.Job{
			Req:     &req,
			Results: make(chan runner.GenerateResponse, 64),
			Err:     make(chan error, 1),
		}
		if err := s.Queue.Submit(r.Context(), job); err != nil {
			http.Error(w, err.Error(), http.StatusServiceUnavailable)
			return
		}
		s.Health.SetLoad(0.5)
		defer s.Health.SetLoad(0)

		flusher, _ := w.(http.Flusher)
		w.Header().Set("Content-Type", "application/x-ndjson")
		// Drain until both channels close. process sends every token
		// (including the terminal done:true) before its deferred closes, so
		// a closed Err must not end the loop while Results still holds
		// buffered tokens — otherwise the final done frame is lost.
		var jobErr error
		for job.Results != nil {
			select {
			case resp, ok := <-job.Results:
				if !ok {
					job.Results = nil
					continue
				}
				_ = json.NewEncoder(w).Encode(resp)
				if flusher != nil {
					flusher.Flush()
				}
			case err, ok := <-job.Err:
				if !ok {
					// Err closed: keep draining any buffered Results.
					job.Err = nil
					continue
				}
				if err != nil {
					jobErr = err
					job.Err = nil
				}
			}
		}
		if jobErr != nil {
			http.Error(w, jobErr.Error(), http.StatusInternalServerError)
		}
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
