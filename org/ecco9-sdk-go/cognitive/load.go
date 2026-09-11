// Package cognitive implements the cognitive load computation that drives
// the echo state thread pool controller and HPA scaling decisions.
package cognitive

import (
	"math"
	"sync"

	"github.com/dtechorg/ecco9-sdk-go/contracts"
)

// LoadIndex computes the global cognitive load index in [0,1] from a set of
// service metrics. This is the autoscaling signal used instead of CPU.
type LoadIndex struct {
	mu sync.RWMutex

	// Weights for each contributing metric (tuned by ontogenesis evolution).
	WThoughtQueue   float64
	WMemoryBacklog  float64
	WDreamStaleness float64
	WCoherenceDelta float64
}

// DefaultLoadIndex returns weights from the platform defaults.
func DefaultLoadIndex() *LoadIndex {
	return &LoadIndex{
		WThoughtQueue:   0.35,
		WMemoryBacklog:  0.25,
		WDreamStaleness: 0.15,
		WCoherenceDelta: 0.25,
	}
}

// Compute folds a batch of per-service metrics into a single load index.
func (li *LoadIndex) Compute(batch []contracts.CognitiveMetrics) float64 {
	li.mu.RLock()
	defer li.mu.RUnlock()
	if len(batch) == 0 {
		return 0
	}
	var sum float64
	for _, m := range batch {
		sum += li.WThoughtQueue * clamp01(m.ThoughtQueueDepth/100.0)
		sum += li.WMemoryBacklog * clamp01(m.MemoryConsolidationBacklog/100.0)
		sum += li.WDreamStaleness * clamp01(m.DreamCycleFreshness.Hours()/24.0)
		sum += li.WCoherenceDelta * clamp01(math.Abs(m.IdentityCoherenceDelta))
	}
	return clamp01(sum / float64(len(batch)))
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
