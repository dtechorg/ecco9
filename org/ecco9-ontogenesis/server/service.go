// Package server wires the ontogenesis engine into a network service
// exposing the OntogenesisService contract.
package server

import (
	"encoding/json"
	"net/http"

	"github.com/dtechorg/ecco9-ontogenesis/ontogenesis"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the ontogenesis engine and health reporter.
type Service struct {
	Engine *ontogenesis.Engine
	Health *health.Reporter
}

// New constructs the ontogenesis service.
func New() *Service {
	return &Service{
		Engine: ontogenesis.NewEngine(),
		Health: health.NewReporter(),
	}
}

// HTTP handlers (REST adapter for the gRPC contracts defined in the proto).
// The gRPC server is generated from ecco9-proto; this REST surface keeps the
// service runnable in the offline sandbox and for simple health checks.

func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// OntogenesisService.SubmitGenome
	mux.HandleFunc("POST /v1/ontogenesis/genomes", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Genome genomeJSON `json:"genome"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id := s.Engine.SubmitGenome(ontogenesis.Genome{
			ID: req.Genome.ID, Genes: req.Genome.Genes,
			Fitness: req.Genome.Fitness, Generation: req.Genome.Generation,
		})
		writeJSON(w, map[string]any{"id": id})
	})

	// OntogenesisService.EvaluateFitness
	mux.HandleFunc("POST /v1/ontogenesis/fitness", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			GenomeID string `json:"genome_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		fitness, ok := s.Engine.EvaluateFitness(req.GenomeID)
		if !ok {
			http.Error(w, "genome not found", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"fitness": fitness})
	})

	// OntogenesisService.Evolve
	mux.HandleFunc("POST /v1/ontogenesis/evolve", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			PopulationSize int     `json:"population_size"`
			Generations    int     `json:"generations"`
			MutationRate   float64 `json:"mutation_rate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if req.PopulationSize <= 0 {
			req.PopulationSize = 16
		}
		if req.Generations <= 0 {
			req.Generations = 10
		}
		job := s.Engine.Evolve(req.PopulationSize, req.Generations, req.MutationRate)
		_, actualization, _ := s.Engine.Actualization()
		s.Health.SetCoherence(actualization)
		writeJSON(w, map[string]any{"job_id": job.ID})
	})

	// OntogenesisService.GetEvolutionStatus
	mux.HandleFunc("GET /v1/ontogenesis/evolve/{job_id}", func(w http.ResponseWriter, r *http.Request) {
		job, ok := s.Engine.Job(r.PathValue("job_id"))
		if !ok {
			http.Error(w, "job not found", http.StatusNotFound)
			return
		}
		resp := map[string]any{
			"running":            job.Running,
			"current_generation": job.CurrentGeneration,
		}
		if job.Best != nil {
			resp["best_genome"] = toJSONGenome(job.Best)
		}
		writeJSON(w, resp)
	})

	// OntogenesisService.GetActualization
	mux.HandleFunc("GET /v1/ontogenesis/actualization", func(w http.ResponseWriter, _ *http.Request) {
		dims, overall, level := s.Engine.Actualization()
		s.Health.SetCoherence(overall)
		writeJSON(w, map[string]any{
			"dimensions":            dims,
			"overall_actualization": overall,
			"level":                 level,
		})
	})

	// Auxiliary endpoints (not part of the proto): entelechy dimension
	// updates and engine introspection.
	mux.HandleFunc("POST /v1/ontogenesis/dimensions", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Ontological  float64 `json:"ontological"`
			Teleological float64 `json:"teleological"`
			Cognitive    float64 `json:"cognitive"`
			Integrative  float64 `json:"integrative"`
			Evolutionary float64 `json:"evolutionary"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Engine.SetDimensions(req.Ontological, req.Teleological, req.Cognitive, req.Integrative, req.Evolutionary)
		_, overall, level := s.Engine.Actualization()
		s.Health.SetCoherence(overall)
		writeJSON(w, map[string]any{"overall_actualization": overall, "level": level})
	})

	mux.HandleFunc("GET /v1/ontogenesis/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, s.Engine.Status())
	})

	// Training metrics feedback (pipeline step 6): reservoir training
	// outcomes seed the population for evolutionary parameter search.
	mux.HandleFunc("POST /v1/ontogenesis/training_metrics", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			CheckpointVersion uint64  `json:"checkpoint_version"`
			SamplesTrained    uint64  `json:"samples_trained"`
			LastError         float64 `json:"last_error"`
			HeldOutMSE        float64 `json:"held_out_mse"`
			SpectralRadius    float64 `json:"spectral_radius"`
			InputScaling      float64 `json:"input_scaling"`
			LeakRate          float64 `json:"leak_rate"`
			ForgettingFactor  float64 `json:"forgetting_factor"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		id := s.Engine.ObserveTrainingMetrics(ontogenesis.TrainingMetrics{
			CheckpointVersion: req.CheckpointVersion,
			SamplesTrained:    req.SamplesTrained,
			LastError:         req.LastError,
			HeldOutMSE:        req.HeldOutMSE,
			SpectralRadius:    req.SpectralRadius,
			InputScaling:      req.InputScaling,
			LeakRate:          req.LeakRate,
			ForgettingFactor:  req.ForgettingFactor,
		})
		_, overall, _ := s.Engine.Actualization()
		s.Health.SetCoherence(overall)
		writeJSON(w, map[string]any{"genome_id": id})
	})

	return mux
}

type genomeJSON struct {
	ID         string             `json:"id"`
	Genes      map[string]float64 `json:"genes"`
	Fitness    float64            `json:"fitness"`
	Generation int                `json:"generation"`
}

func toJSONGenome(g *ontogenesis.Genome) genomeJSON {
	return genomeJSON{ID: g.ID, Genes: g.Genes, Fitness: g.Fitness, Generation: g.Generation}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
