package orchestrator

import (
	"testing"
	"time"

	"github.com/dtechorg/ecco9-sdk-go/contracts"
)

func metric(service, instance string, queueDepth float64) contracts.CognitiveMetrics {
	return contracts.CognitiveMetrics{
		ServiceName:       service,
		InstanceID:        instance,
		ThoughtQueueDepth: queueDepth,
	}
}

// saturatingMetrics maxes every load term so the global load index is 1.
func saturatingMetrics() contracts.CognitiveMetrics {
	return contracts.CognitiveMetrics{
		ServiceName:                "ecco9-reservoir",
		InstanceID:                 "hot",
		ThoughtQueueDepth:          1000,
		MemoryConsolidationBacklog: 1000,
		DreamCycleFreshness:        24 * time.Hour,
		IdentityCoherenceDelta:     1,
	}
}

func directivesByService(ds []contracts.ThreadPoolDirective) map[string]contracts.ThreadPoolDirective {
	out := make(map[string]contracts.ThreadPoolDirective, len(ds))
	for _, d := range ds {
		out[d.ServiceName] = d
	}
	return out
}

func TestGlobalLoadFoldsReportedMetrics(t *testing.T) {
	h := NewHomeostaticController()
	if got := h.GlobalLoad(); got != 0 {
		t.Fatalf("GlobalLoad with no metrics = %v, want 0", got)
	}

	// Queue depth 100 saturates the thought-queue term (weight 0.35).
	h.ReportMetrics(metric("ecco9-reservoir", "r1", 100))
	if got := h.GlobalLoad(); got <= 0 {
		t.Fatalf("GlobalLoad after reporting = %v, want > 0", got)
	}

	// Idle instances dilute the batch average.
	h.ReportMetrics(metric("ecco9-runner", "run1", 0))
	hot := h.GlobalLoad()
	h.ReportMetrics(metric("ecco9-memory", "m1", 0))
	if diluted := h.GlobalLoad(); diluted >= hot {
		t.Fatalf("diluted load %v should be below %v", diluted, hot)
	}
}

func TestDirectivesCoverLoadBearingServices(t *testing.T) {
	h := NewHomeostaticController()
	ds := directivesByService(h.Directives())
	for _, svc := range []string{"ecco9-reservoir", "ecco9-echobeats", "ecco9-runner"} {
		if _, ok := ds[svc]; !ok {
			t.Fatalf("missing directive for %s: %v", svc, ds)
		}
	}
}

func TestDirectivesIdleSystemUsesLowerBounds(t *testing.T) {
	h := NewHomeostaticController()
	ds := directivesByService(h.Directives())
	if w := ds["ecco9-reservoir"].TargetWorkers; w != 2 {
		t.Fatalf("idle reservoir workers = %d, want lower bound 2", w)
	}
	if hz := ds["ecco9-echobeats"].CycleFrequencyHz; hz != 0.5 {
		t.Fatalf("idle echobeats cadence = %v, want 0.5Hz", hz)
	}
	if w := ds["ecco9-runner"].TargetWorkers; w != 1 {
		t.Fatalf("idle runner workers = %d, want lower bound 1", w)
	}
}

func TestDirectivesSaturatedSystemUsesUpperBounds(t *testing.T) {
	h := NewHomeostaticController()
	h.ReportMetrics(saturatingMetrics())
	ds := directivesByService(h.Directives())
	if w := ds["ecco9-reservoir"].TargetWorkers; w != 64 {
		t.Fatalf("hot reservoir workers = %d, want upper bound 64", w)
	}
	if hz := ds["ecco9-echobeats"].CycleFrequencyHz; hz != 2.0 {
		t.Fatalf("hot echobeats cadence = %v, want 2Hz", hz)
	}
	if w := ds["ecco9-runner"].TargetWorkers; w != 16 {
		t.Fatalf("hot runner workers = %d, want upper bound 16", w)
	}
}

func TestDirectivesHysteresisReturnsCachedSnapshot(t *testing.T) {
	h := NewHomeostaticController()
	first := h.Directives()

	// Within minInterval, new metrics must not trigger recomputation.
	h.ReportMetrics(saturatingMetrics())
	second := h.Directives()

	if len(first) != len(second) {
		t.Fatalf("hysteresis changed directive count: %d -> %d", len(first), len(second))
	}
	a, b := directivesByService(first), directivesByService(second)
	for svc, d1 := range a {
		d2 := b[svc]
		if d1 != d2 {
			t.Fatalf("hysteresis recomputed %s directive: %+v -> %+v", svc, d1, d2)
		}
	}
}

func TestScaleIntClampsLoad(t *testing.T) {
	if got := scaleInt(-1, 2, 64); got != 2 {
		t.Fatalf("scaleInt(-1) = %d, want lower bound 2", got)
	}
	if got := scaleInt(2, 2, 64); got != 64 {
		t.Fatalf("scaleInt(2) = %d, want upper bound 64", got)
	}
}
