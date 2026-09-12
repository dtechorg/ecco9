// Package server wires the identity registry into a REST service exposing
// the IdentityService contract (ecco9.identity.v1).
package server

import (
	"encoding/json"
	"net/http"
	"sync"

	"github.com/dtechorg/ecco9-identity/identity"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the embedding registry, persona manager, theory-of-mind
// module, consciousness state store, and health reporter.
type Service struct {
	stateMu  sync.Mutex
	Registry *identity.Registry
	ToM      *identity.TheoryOfMind
	Store    *identity.StateStore
	State    *identity.ConsciousnessState
	Health   *health.Reporter
}

// New constructs the service, restoring consciousness state if persisted.
func New(stateDir, identityName string) *Service {
	if identityName == "" {
		identityName = "deep-tree-echo"
	}
	store, err := identity.NewStateStore(stateDir)
	if err != nil {
		store, _ = identity.NewStateStore("")
	}
	h := health.NewReporter()
	svc := &Service{
		Registry: identity.NewRegistry(),
		ToM:      identity.NewTheoryOfMind(),
		Store:    store,
		State:    store.Load(identityName),
		Health:   h,
	}
	// Genesis embedding for this identity from its kernel seed.
	_, _ = svc.Registry.Register(identityName, identity.Embed(identityName+"/kernel"))
	return svc
}

// Routes exposes the REST adapter for ecco9.identity.v1.IdentityService.
// The gRPC server is generated from ecco9-proto; this REST surface keeps
// the service runnable in the offline sandbox and for simple clients.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// rpc VerifyIdentity
	mux.HandleFunc("POST /v1/identity/verify", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Embedding *identity.IdentityEmbedding `json:"embedding"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Embedding == nil {
			http.Error(w, "invalid VerifyIdentityRequest", http.StatusBadRequest)
			return
		}
		verified, coherence, err := s.Registry.VerifyIdentity(req.Embedding)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		s.Health.SetCoherence(coherence)
		s.stateMu.Lock()
		s.State.AwarenessLevel = coherence
		err = s.Store.Save(s.State)
		s.stateMu.Unlock()
		if err != nil {
			http.Error(w, "persist identity state: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"verified": verified, "coherence": coherence})
	})

	// rpc SimilarityMatch
	mux.HandleFunc("POST /v1/identity/similarity", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Vector []float64 `json:"vector"`
			TopK   int       `json:"top_k"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ids, scores := s.Registry.SimilarityMatch(req.Vector, req.TopK)
		writeJSON(w, map[string]any{"identity_ids": ids, "scores": scores})
	})

	// rpc SwitchPersona
	mux.HandleFunc("POST /v1/identity/persona", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IdentityID string `json:"identity_id"`
			Persona    string `json:"persona"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		applied, err := s.Registry.SwitchPersona(req.IdentityID, req.Persona)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		s.stateMu.Lock()
		s.State.ActivePersona = applied
		err = s.Store.Save(s.State)
		s.stateMu.Unlock()
		if err != nil {
			http.Error(w, "persist identity state: "+err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"applied_persona": applied})
	})

	// rpc GetAgentModel
	mux.HandleFunc("POST /v1/identity/agent-model", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			AgentID string `json:"agent_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentID == "" {
			http.Error(w, "invalid GetAgentModelRequest", http.StatusBadRequest)
			return
		}
		model := s.ToM.GetAgentModel(req.AgentID)
		writeJSON(w, map[string]any{"model": model})
	})

	// rpc Register (embed helper): derive + register an embedding from a seed.
	mux.HandleFunc("POST /v1/identity/embed", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			IdentityID string `json:"identity_id"`
			Seed       string `json:"seed"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IdentityID == "" {
			http.Error(w, "identity_id required", http.StatusBadRequest)
			return
		}
		e, err := s.Registry.Register(req.IdentityID, identity.Embed(req.Seed))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"identity_id": e.IdentityID, "updated_at": e.UpdatedAt})
	})

	// ToM extras: trust/belief updates and deception detection.
	mux.HandleFunc("POST /v1/identity/agent-model/trust", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			AgentID string  `json:"agent_id"`
			Outcome float64 `json:"outcome"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AgentID == "" {
			http.Error(w, "agent_id required", http.StatusBadRequest)
			return
		}
		s.ToM.UpdateTrust(req.AgentID, req.Outcome)
		writeJSON(w, map[string]any{"model": s.ToM.GetAgentModel(req.AgentID)})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
