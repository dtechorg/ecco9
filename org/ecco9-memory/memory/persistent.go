package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PersistentStore provides file-backed durable snapshots of the hypergraph,
// adapted from core/memory/persistent.go. The production deployment swaps
// this for Supabase/PostgreSQL; the interface is identical (Store/Load of
// full snapshots) so the swap is transparent to the service.

// SnapshotData is the serializable form of the hypergraph.
type SnapshotData struct {
	Nodes      map[string]*MemoryNode `json:"nodes"`
	Edges      map[string]*MemoryEdge `json:"edges"`
	HyperEdges map[string]*HyperEdge  `json:"hyperedges"`
	SavedAt    time.Time              `json:"saved_at"`
}

// PersistentStore writes periodic JSON snapshots to a state directory
// (mounted from the StatefulSet volumeClaimTemplate in production).
type PersistentStore struct {
	mu        sync.Mutex
	stateFile string
	saveCount uint64
	lastSave  time.Time
}

// NewPersistentStore creates a store rooted at stateDir. An empty stateDir
// disables persistence (pure in-memory mode for local dev).
func NewPersistentStore(stateDir string) (*PersistentStore, error) {
	if stateDir == "" {
		return &PersistentStore{}, nil
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return nil, fmt.Errorf("create state dir: %w", err)
	}
	return &PersistentStore{stateFile: filepath.Join(stateDir, "hypergraph_snapshot.json")}, nil
}

// Enabled reports whether persistence is active.
func (ps *PersistentStore) Enabled() bool { return ps.stateFile != "" }

// Save writes a snapshot of the hypergraph to disk atomically
// (write-then-rename) so a crash cannot corrupt the previous snapshot.
func (ps *PersistentStore) Save(hg *HypergraphMemory) error {
	if !ps.Enabled() {
		return nil
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()

	nodes, edges, hyper := hg.Snapshot()
	data := SnapshotData{Nodes: nodes, Edges: edges, HyperEdges: hyper, SavedAt: time.Now()}

	buf, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}
	tmp := ps.stateFile + ".tmp"
	if err := os.WriteFile(tmp, buf, 0o644); err != nil {
		return fmt.Errorf("write snapshot: %w", err)
	}
	if err := os.Rename(tmp, ps.stateFile); err != nil {
		return fmt.Errorf("rename snapshot: %w", err)
	}
	ps.saveCount++
	ps.lastSave = time.Now()
	return nil
}

// Load restores a snapshot into the hypergraph, if one exists.
func (ps *PersistentStore) Load(hg *HypergraphMemory) error {
	if !ps.Enabled() {
		return nil
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()

	buf, err := os.ReadFile(ps.stateFile)
	if os.IsNotExist(err) {
		return nil // fresh start
	}
	if err != nil {
		return fmt.Errorf("read snapshot: %w", err)
	}
	var data SnapshotData
	if err := json.Unmarshal(buf, &data); err != nil {
		return fmt.Errorf("unmarshal snapshot: %w", err)
	}
	for _, n := range data.Nodes {
		if _, err := hg.AddNode(n); err != nil {
			return err
		}
	}
	for _, e := range data.Edges {
		if _, err := hg.AddEdge(e); err != nil {
			return err
		}
	}
	for _, h := range data.HyperEdges {
		if _, err := hg.AddHyperEdge(h); err != nil {
			return err
		}
	}
	return nil
}

// Stats returns persistence counters.
func (ps *PersistentStore) Stats() (saves uint64, lastSave time.Time) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	return ps.saveCount, ps.lastSave
}
