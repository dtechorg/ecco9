package cognitive

import (
	"testing"
	"time"

	"github.com/dtechorg/ecco9-sdk-go/contracts"
)

func TestLoadIndexEmptyBatch(t *testing.T) {
	if got := DefaultLoadIndex().Compute(nil); got != 0 {
		t.Fatalf("empty batch load = %v, want 0", got)
	}
}

func TestLoadIndexBounds(t *testing.T) {
	li := DefaultLoadIndex()

	// A fully idle system yields zero load.
	idle := contracts.CognitiveMetrics{ServiceName: "reservoir"}
	if got := li.Compute([]contracts.CognitiveMetrics{idle}); got != 0 {
		t.Fatalf("idle load = %v, want 0", got)
	}

	// A saturated system stays clamped to [0,1].
	hot := contracts.CognitiveMetrics{
		ServiceName:                "echobeats",
		ThoughtQueueDepth:          1e6,
		MemoryConsolidationBacklog: 1e6,
		DreamCycleFreshness:        720 * time.Hour,
		IdentityCoherenceDelta:     1.0,
	}
	got := li.Compute([]contracts.CognitiveMetrics{hot})
	if got <= 0 || got > 1 {
		t.Fatalf("saturated load = %v, want in (0,1]", got)
	}
}

func TestLoadIndexWeightsSumToOne(t *testing.T) {
	li := DefaultLoadIndex()
	total := li.WThoughtQueue + li.WMemoryBacklog + li.WDreamStaleness + li.WCoherenceDelta
	if total < 0.999 || total > 1.001 {
		t.Fatalf("default weights sum to %v, want ~1.0", total)
	}
}
