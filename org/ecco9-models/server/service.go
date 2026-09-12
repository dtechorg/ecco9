// Package server wires the models core into a REST service mirroring the
// ModelRegistryService proto contract (PullModel, PushModel, ListModels,
// DeleteModel, ConvertModel).
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/dtechorg/ecco9-models/models"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the registry, puller, and converter with health.
type Service struct {
	Registry  *models.Registry
	Puller    *models.Puller
	Converter *models.Converter
	Health    *health.Reporter
}

// dirSource is the offline RemoteSource: serves model blobs from a seed
// directory (ECCO9_MODEL_SEED_DIR), standing in for the HTTP registry.
type dirSource struct{ root string }

func (d *dirSource) Fetch(_ context.Context, name string) (io.ReadCloser, int64, error) {
	p := filepath.Join(d.root, filepath.Base(name))
	fi, err := os.Stat(p)
	if err != nil {
		return nil, 0, fmt.Errorf("remote model not found: %s", name)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, 0, err
	}
	return f, fi.Size(), nil
}

// New constructs the service with a filesystem blob store at root.
func New(root string) (*Service, error) {
	store, err := models.NewFSStore(root)
	if err != nil {
		return nil, err
	}
	reg := models.NewRegistry(store)
	return &Service{
		Registry:  reg,
		Puller:    &models.Puller{Reg: reg, Remote: &dirSource{root: os.Getenv("ECCO9_MODEL_SEED_DIR")}},
		Converter: models.NewConverter(),
		Health:    health.NewReporter(),
	}, nil
}

// Routes returns the REST adapter for the gRPC contracts defined in
// ecco9/models/v1/models.proto.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// ModelRegistryService.PullModel (streamed as NDJSON progress).
	mux.HandleFunc("POST /v1/models/pull", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.streamProgress(w, r, func(fn func(models.Progress)) error {
			return s.Puller.Pull(r.Context(), req.Name, fn)
		})
	})

	// ModelRegistryService.PushModel (streamed as NDJSON progress).
	mux.HandleFunc("POST /v1/models/push", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.streamProgress(w, r, func(fn func(models.Progress)) error {
			return s.Puller.Push(r.Context(), req.Name, fn)
		})
	})

	// ModelRegistryService.ListModels
	mux.HandleFunc("GET /v1/models", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]any{"models": s.Registry.List()})
	})

	// ModelRegistryService.DeleteModel
	mux.HandleFunc("DELETE /v1/models/{name}", func(w http.ResponseWriter, r *http.Request) {
		ok := s.Registry.Delete(r.PathValue("name"))
		writeJSON(w, map[string]any{"deleted": ok})
	})

	// ModelRegistryService.ConvertModel
	mux.HandleFunc("POST /v1/models/convert", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			SourcePath   string `json:"source_path"`
			TargetFormat string `json:"target_format"`
			Quantization string `json:"quantization"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id, err := s.Converter.Start(req.SourcePath, req.TargetFormat, req.Quantization)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"job_id": id})
	})

	// Conversion job introspection (control plane).
	mux.HandleFunc("GET /v1/models/convert/{id}", func(w http.ResponseWriter, r *http.Request) {
		job, err := s.Converter.Status(r.PathValue("id"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, job)
	})

	// Direct blob upload (Register path from server/create.go).
	mux.HandleFunc("POST /v1/models/register/{name}", func(w http.ResponseWriter, r *http.Request) {
		s.Health.SetLoad(0.5)
		info, err := s.Registry.Register(r.PathValue("name"), r.Body, map[string]string{"format": "gguf"})
		s.Health.SetLoad(0)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, info)
	})

	return mux
}

// streamProgress encodes each Progress as one NDJSON line, ending with the
// error or a done record — the REST shape of the streaming Pull/Push RPCs.
func (s *Service) streamProgress(w http.ResponseWriter, r *http.Request, run func(func(models.Progress)) error) {
	s.Health.SetLoad(0.5)
	defer s.Health.SetLoad(0)
	flusher, _ := w.(http.Flusher)
	w.Header().Set("Content-Type", "application/x-ndjson")
	err := run(func(p models.Progress) {
		_ = json.NewEncoder(w).Encode(p)
		if flusher != nil {
			flusher.Flush()
		}
	})
	if err != nil {
		_ = json.NewEncoder(w).Encode(models.Progress{Status: "error: " + err.Error(), Done: true})
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
