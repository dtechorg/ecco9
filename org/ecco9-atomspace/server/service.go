// Package server wires the atomspace core into a REST service exposing the
// AtomSpaceService contract (AddAtom, AddLink, PatternMatch, UpdateAttention).
package server

import (
	"encoding/json"
	"net/http"

	"github.com/dtechorg/ecco9-atomspace/atomspace"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the AtomSpace and health reporter.
type Service struct {
	Space  *atomspace.AtomSpace
	Health *health.Reporter
}

// New constructs the AtomSpace service.
func New() *Service {
	return &Service{
		Space:  atomspace.New(),
		Health: health.NewReporter(),
	}
}

// Routes returns the REST adapter for the AtomSpaceService gRPC contract.
// The gRPC server is generated from ecco9-proto; this REST surface keeps the
// service runnable in the offline sandbox and for simple health checks.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// AddAtom
	mux.HandleFunc("POST /v1/atomspace/atoms", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Atom struct {
				Type  atomspace.AtomType    `json:"type"`
				Name  string                `json:"name"`
				Truth *atomspace.TruthValue `json:"truth"`
			} `json:"atom"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		atom, err := s.Space.AddAtom(req.Atom.Type, req.Atom.Name, req.Atom.Truth)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.updateLoad()
		writeJSON(w, map[string]any{"id": atom.ID})
	})

	// AddLink
	mux.HandleFunc("POST /v1/atomspace/links", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Link struct {
				Type     atomspace.LinkType    `json:"type"`
				Outgoing []string              `json:"outgoing_atom_ids"`
				Truth    *atomspace.TruthValue `json:"truth"`
			} `json:"link"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		link, err := s.Space.AddLink(req.Link.Type, req.Link.Outgoing, req.Link.Truth)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		s.updateLoad()
		writeJSON(w, map[string]any{"id": link.ID})
	})

	// PatternMatch
	mux.HandleFunc("POST /v1/atomspace/pattern-match", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Pattern    string `json:"pattern"`
			MaxResults int    `json:"max_results"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		pattern := atomspace.ParsePattern(req.Pattern)
		result := s.Space.Match(pattern)
		if req.MaxResults > 0 && len(result.Bindings) > req.MaxResults {
			result.Bindings = result.Bindings[:req.MaxResults]
			result.Count = req.MaxResults
		}
		// Resolve bindings to atoms for the response.
		atoms := make([]map[string]any, 0, result.Count)
		for _, binding := range result.Bindings {
			resolved := make(map[string]any, len(binding))
			for variable, atomID := range binding {
				if atom, ok := s.Space.GetAtom(atomID); ok {
					resolved[variable] = atom
				} else if link, ok := s.Space.GetLink(atomID); ok {
					resolved[variable] = link
				} else {
					resolved[variable] = atomID
				}
			}
			atoms = append(atoms, resolved)
		}
		writeJSON(w, map[string]any{"atoms": atoms, "count": result.Count})
	})

	// UpdateAttention
	mux.HandleFunc("POST /v1/atomspace/attention", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			AtomID    string `json:"atom_id"`
			Attention struct {
				STI float64 `json:"sti"`
				LTI float64 `json:"lti"`
			} `json:"attention"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		av := &atomspace.AttentionValue{
			STI: int16(req.Attention.STI),
			LTI: int16(req.Attention.LTI),
		}
		applied := s.Space.UpdateAttention(req.AtomID, av)
		if !applied {
			http.Error(w, "atom or link not found", http.StatusNotFound)
			return
		}
		writeJSON(w, map[string]any{"applied": applied})
	})

	// Atom lookup.
	mux.HandleFunc("GET /v1/atomspace/atoms/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if atom, ok := s.Space.GetAtom(id); ok {
			writeJSON(w, atom)
			return
		}
		if link, ok := s.Space.GetLink(id); ok {
			writeJSON(w, link)
			return
		}
		http.Error(w, "not found", http.StatusNotFound)
	})

	// Status / hypergraph statistics.
	mux.HandleFunc("GET /v1/atomspace/status", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, s.Space.Status())
	})

	return mux
}

// updateLoad reports hypergraph occupancy as the cognitive load signal.
func (s *Service) updateLoad() {
	status := s.Space.Status()
	atoms, _ := status["atoms"].(int)
	links, _ := status["links"].(int)
	s.Health.SetLoad(clamp01(float64(atoms+links) / 10000.0))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
