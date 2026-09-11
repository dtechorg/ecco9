package memory

import (
	"sync"
	"time"
)

// MemoryWeaver dynamically adapts memory connection patterns based on
// usage, adapted from core/memory/weaver.go. It observes edge traversal
// frequency and periodically strengthens, weakens, creates, or removes
// weighted connections.

// Connection is an adaptive weighted link tracked by the weaver.
type Connection struct {
	FromNode   string    `json:"from_node"`
	ToNode     string    `json:"to_node"`
	Weight     float64   `json:"weight"`
	Usage      int64     `json:"usage"`
	LastAccess time.Time `json:"last_access"`
	Strength   float64   `json:"strength"`
}

// UsagePattern summarizes how a connection is being used.
type UsagePattern struct {
	ConnectionID string  `json:"connection_id"`
	AccessCount  int64   `json:"access_count"`
	Frequency    float64 `json:"frequency"`
	Trend        string  `json:"trend"` // "increasing", "decreasing", "stable"
}

// AdaptationCycle records one memory restructuring pass.
type AdaptationCycle struct {
	Timestamp          time.Time `json:"timestamp"`
	ConnectionsAdded   int       `json:"connections_added"`
	ConnectionsRemoved int       `json:"connections_removed"`
	WeightAdjustments  int       `json:"weight_adjustments"`
}

// Adaptation is a single suggested structural change.
type Adaptation struct {
	Type      string  // "strengthen", "weaken", "remove"
	FromNode  string
	ToNode    string
	NewWeight float64
}

// MemoryWeaver tracks connections and applies usage-based adaptation.
type MemoryWeaver struct {
	mu               sync.RWMutex
	connections      map[string][]Connection
	adaptationCycles []AdaptationCycle
	lastWeave        time.Time
}

// NewMemoryWeaver creates an empty weaver.
func NewMemoryWeaver() *MemoryWeaver {
	return &MemoryWeaver{
		connections:      make(map[string][]Connection),
		adaptationCycles: make([]AdaptationCycle, 0),
		lastWeave:        time.Now(),
	}
}

// RecordAccess registers a traversal between two nodes (creates the
// connection on first sight).
func (mw *MemoryWeaver) RecordAccess(from, to string) {
	mw.mu.Lock()
	defer mw.mu.Unlock()
	for nodeID, conns := range mw.connections {
		for i, c := range conns {
			if c.FromNode == from && c.ToNode == to {
				mw.connections[nodeID][i].Usage++
				mw.connections[nodeID][i].LastAccess = time.Now()
				return
			}
		}
	}
	mw.connections[from] = append(mw.connections[from], Connection{
		FromNode:   from,
		ToNode:     to,
		Weight:     0.5,
		Usage:      1,
		LastAccess: time.Now(),
		Strength:   0.5,
	})
}

// WeaveConnections runs one adaptation cycle: connections used heavily
// since the last weave are strengthened; stale ones are weakened and
// eventually pruned. Mirrors the strengthen/weaken/remove rules of the
// original PatternDetector adaptation loop.
func (mw *MemoryWeaver) WeaveConnections() AdaptationCycle {
	mw.mu.Lock()
	defer mw.mu.Unlock()

	cycle := AdaptationCycle{Timestamp: time.Now()}
	staleAfter := time.Since(mw.lastWeave) + time.Hour // grace window

	for nodeID, conns := range mw.connections {
		kept := conns[:0]
		for _, c := range conns {
			idle := time.Since(c.LastAccess)
			switch {
			case c.Usage > 10:
				c.Weight = min(1.0, c.Weight+0.1)
				c.Strength = min(1.0, c.Strength+0.1)
				cycle.WeightAdjustments++
			case idle > staleAfter && c.Usage < 2:
				c.Weight = c.Weight * 0.8
				c.Strength = c.Strength - 0.1
				cycle.WeightAdjustments++
			}
			if c.Strength <= 0 {
				cycle.ConnectionsRemoved++
				continue
			}
			kept = append(kept, c)
		}
		mw.connections[nodeID] = kept
	}

	mw.adaptationCycles = append(mw.adaptationCycles, cycle)
	if len(mw.adaptationCycles) > 100 {
		mw.adaptationCycles = mw.adaptationCycles[1:]
	}
	mw.lastWeave = time.Now()
	return cycle
}

// GetAdaptationHistory returns past weaving cycles.
func (mw *MemoryWeaver) GetAdaptationHistory() []AdaptationCycle {
	mw.mu.RLock()
	defer mw.mu.RUnlock()
	out := make([]AdaptationCycle, len(mw.adaptationCycles))
	copy(out, mw.adaptationCycles)
	return out
}

// ConnectionCount returns the number of tracked connections.
func (mw *MemoryWeaver) ConnectionCount() int {
	mw.mu.RLock()
	defer mw.mu.RUnlock()
	n := 0
	for _, conns := range mw.connections {
		n += len(conns)
	}
	return n
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
