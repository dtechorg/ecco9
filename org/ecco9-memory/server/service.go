// Package server wires the hypergraph memory core into a REST service
// exposing the MemoryService contract (ecco9.memory.v1).
package server

import (
	"encoding/json"
	"net/http"

	"github.com/dtechorg/ecco9-memory/memory"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// Service bundles the hypergraph, weaver, persistence, and health reporter.
type Service struct {
	Graph  *memory.HypergraphMemory
	Weaver *memory.MemoryWeaver
	Store  *memory.PersistentStore
	Health *health.Reporter
}

// New constructs the service and restores any persisted snapshot.
func New(stateDir string) *Service {
	g := memory.NewHypergraphMemory()
	store, err := memory.NewPersistentStore(stateDir)
	if err != nil {
		store, _ = memory.NewPersistentStore("")
	}
	_ = store.Load(g)
	return &Service{
		Graph:  g,
		Weaver: memory.NewMemoryWeaver(),
		Store:  store,
		Health: health.NewReporter(),
	}
}

// Routes exposes the REST adapter for ecco9.memory.v1.MemoryService.
// The gRPC server is generated from ecco9-proto; this REST surface keeps
// the service runnable in the offline sandbox and for simple clients.
func (s *Service) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/healthz", s.Health.Handler())

	// rpc AddNode
	mux.HandleFunc("POST /v1/memory/nodes", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Node *memory.MemoryNode `json:"node"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Node == nil {
			http.Error(w, "invalid AddNodeRequest", http.StatusBadRequest)
			return
		}
		id, err := s.Graph.AddNode(req.Node)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = s.Store.Save(s.Graph)
		writeJSON(w, map[string]any{"id": id})
	})

	// rpc AddEdge
	mux.HandleFunc("POST /v1/memory/edges", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Edge *memory.MemoryEdge `json:"edge"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Edge == nil {
			http.Error(w, "invalid AddEdgeRequest", http.StatusBadRequest)
			return
		}
		id, err := s.Graph.AddEdge(req.Edge)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		s.Weaver.RecordAccess(req.Edge.SourceID, req.Edge.TargetID)
		_ = s.Store.Save(s.Graph)
		writeJSON(w, map[string]any{"id": id})
	})

	// rpc AddHyperEdge
	mux.HandleFunc("POST /v1/memory/hyperedges", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			HyperEdge *memory.HyperEdge `json:"hyperedge"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.HyperEdge == nil {
			http.Error(w, "invalid AddHyperEdgeRequest", http.StatusBadRequest)
			return
		}
		id, err := s.Graph.AddHyperEdge(req.HyperEdge)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		_ = s.Store.Save(s.Graph)
		writeJSON(w, map[string]any{"id": id})
	})

	// rpc Traverse
	mux.HandleFunc("POST /v1/memory/traverse", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			StartNodeID    string `json:"start_node_id"`
			MaxDepth       int    `json:"max_depth"`
			RelationFilter string `json:"relation_filter"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		nodes, edges, err := s.Graph.Traverse(req.StartNodeID, req.MaxDepth, req.RelationFilter)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		for _, e := range edges {
			s.Weaver.RecordAccess(e.SourceID, e.TargetID)
		}
		writeJSON(w, map[string]any{"nodes": nodes, "edges": edges})
	})

	// rpc SimilaritySearch
	mux.HandleFunc("POST /v1/memory/similarity", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Embedding []float64 `json:"embedding"`
			TopK      int       `json:"top_k"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]any{"nodes": s.Graph.SimilaritySearch(req.Embedding, req.TopK)})
	})

	// Weaver introspection: run one adaptation cycle and return stats.
	mux.HandleFunc("POST /v1/memory/weave", func(w http.ResponseWriter, _ *http.Request) {
		cycle := s.Weaver.WeaveConnections()
		nodes, edges, hyper := s.Graph.Stats()
		writeJSON(w, map[string]any{
			"cycle":           cycle,
			"connections":     s.Weaver.ConnectionCount(),
			"node_count":      nodes,
			"edge_count":      edges,
			"hyperedge_count": hyper,
		})
	})

	return mux
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
