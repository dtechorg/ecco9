// Package server wires the reservoir core into a network service exposing
// the ReservoirService and ReservoirControlService contracts.
package server

import (
	"encoding/json"
	"math"
	"net/http"

	"github.com/dtechorg/ecco9-reservoir/reservoir"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the reservoir, its online trainer, and the thread pool.
type Service struct {
	Reservoir *reservoir.EchoStateReservoir
	Trainer   *reservoir.OnlineTrainer
	Pipeline  *reservoir.TrainingPipeline
	Pool      *reservoir.ThreadPoolController
	Health    *health.Reporter
}

// New constructs the service with a persona-configured reservoir.
func New(size int, persona reservoir.PersonaType, outDim int) *Service {
	r := reservoir.New(size, persona)
	trainer := reservoir.NewOnlineTrainer(r, outDim, 0.99)
	return &Service{
		Reservoir: r,
		Trainer:   trainer,
		Pipeline:  reservoir.NewTrainingPipeline(trainer),
		Pool:      reservoir.NewThreadPoolController(2, 64),
		Health:    health.NewReporter(),
	}
}

// HTTP handlers (REST adapter for the gRPC contracts defined in the proto).
// The gRPC server is generated from ecco9-proto; this REST surface keeps the
// service runnable in the offline sandbox and for simple health checks.

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	mux.HandleFunc("/v1/reservoir/state", func(w http.ResponseWriter, _ *http.Request) {
		ep, cx := s.Reservoir.Metrics()
		writeJSON(w, map[string]any{
			"state":         s.Reservoir.State(),
			"persona":       s.Reservoir.Persona(),
			"echo_property": ep,
			"complexity":    cx,
		})
	})

	mux.HandleFunc("/v1/reservoir/process", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input []float64 `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		state := s.Reservoir.Update(req.Input)
		writeJSON(w, map[string]any{"state": state})
	})

	mux.HandleFunc("/v1/reservoir/train", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Input  []float64 `json:"input"`
			Target []float64 `json:"target"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		errVal := s.Trainer.TrainSample(req.Input, req.Target)
		if math.IsNaN(errVal) {
			http.Error(w, "target dimension does not match trainer output dimension", http.StatusBadRequest)
			return
		}
		samples, lastErr, version := s.Trainer.Stats()
		writeJSON(w, map[string]any{
			"prediction_error": errVal,
			"samples_trained":  samples,
			"last_error":       lastErr,
			"checkpoint":       version,
		})
	})

	mux.HandleFunc("/v1/reservoir/persona", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Persona reservoir.PersonaType `json:"persona"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Reservoir.SetPersona(req.Persona)
		writeJSON(w, map[string]any{"applied": s.Reservoir.Persona()})
	})

	mux.HandleFunc("/v1/reservoir/threadpool", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Workers int    `json:"workers"`
			Reason  string `json:"reason"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		prev, cur := s.Pool.SetWorkers(req.Workers)
		writeJSON(w, map[string]any{"previous_workers": prev, "current_workers": cur})
	})

	// Online training pipeline endpoints: versioned checkpoints, held-out
	// validation, and hot reload (pipeline steps 4-5).

	// Capture a versioned checkpoint of the current readout weights.
	mux.HandleFunc("POST /v1/reservoir/checkpoints", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		cp := s.Pipeline.Checkpoint()
		writeJSON(w, map[string]any{
			"version":      cp.Version,
			"samples":      cp.Samples,
			"train_error":  cp.TrainError,
			"deployed":     s.Pipeline.Deployed(),
			"checkpointed": true,
		})
	})

	// Validate a checkpoint against held-out samples and promote it when the
	// prediction error is within the promotion gate.
	mux.HandleFunc("POST /v1/reservoir/checkpoints/validate", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Version uint64                       `json:"version"`
			HeldOut []reservoir.ValidationSample `json:"held_out"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		cp, ok := s.Pipeline.Validate(req.Version, req.HeldOut)
		if !ok {
			http.Error(w, "checkpoint not found", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{
			"version":      cp.Version,
			"held_out_mse": cp.HeldOutMSE,
			"promoted":     s.Pipeline.Deployed() == cp.Version,
			"gate":         s.Pipeline.MaxValidationMSE,
		})
	})

	// Hot-reload a versioned checkpoint into the live trainer.
	mux.HandleFunc("POST /v1/reservoir/checkpoints/reload", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Version uint64 `json:"version"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !s.Pipeline.Reload(req.Version) {
			http.Error(w, "checkpoint not found", http.StatusNotFound)
			return
		}
		samples, lastErr, version := s.Trainer.Stats()
		writeJSON(w, map[string]any{
			"reloaded":        true,
			"version":         version,
			"samples_trained": samples,
			"last_error":      lastErr,
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
