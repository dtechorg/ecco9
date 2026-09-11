// Package orchestrator implements the central nervous system: agent, task,
// and workflow lifecycle plus the echo state thread pool controller.
package orchestrator

import (
	"sync"
	"time"

	"github.com/dtechorg/ecco9-sdk-go/cognitive"
	"github.com/dtechorg/ecco9-sdk-go/contracts"
)

// HomeostaticController computes the global cognitive load index and emits
// adaptive ThreadPoolDirectives to each service. This is the echo state
// thread pool controller described in the architecture.
type HomeostaticController struct {
	mu sync.RWMutex

	load *cognitive.LoadIndex

	// Latest metrics per service instance.
	metrics map[string]contracts.CognitiveMetrics

	// Current directive targets per service.
	targets map[string]contracts.ThreadPoolDirective

	// Hysteresis to avoid oscillation.
	lastAdjust time.Time
	minInterval time.Duration
}

// NewHomeostaticController builds the controller with default load weights.
func NewHomeostaticController() *HomeostaticController {
	return &HomeostaticController{
		load:        cognitive.DefaultLoadIndex(),
		metrics:     make(map[string]contracts.CognitiveMetrics),
		targets:     make(map[string]contracts.ThreadPoolDirective),
		minInterval: 5 * time.Second,
	}
}

// ReportMetrics ingests a CognitiveMetrics sample from a service.
func (h *HomeostaticController) ReportMetrics(m contracts.CognitiveMetrics) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.metrics[m.ServiceName+"/"+m.InstanceID] = m
}

// GlobalLoad folds all reported metrics into the global load index.
func (h *HomeostaticController) GlobalLoad() float64 {
	h.mu.RLock()
	batch := make([]contracts.CognitiveMetrics, 0, len(h.metrics))
	for _, m := range h.metrics {
		batch = append(batch, m)
	}
	h.mu.RUnlock()
	return h.load.Compute(batch)
}

// Directives computes (or returns cached) thread pool directives for each
// known service based on the current global load. Higher load → more
// reservoir workers, faster EchoBeats cadence, scaled inference.
func (h *HomeostaticController) Directives() []contracts.ThreadPoolDirective {
	h.mu.Lock()
	defer h.mu.Unlock()

	if time.Since(h.lastAdjust) < h.minInterval && len(h.targets) > 0 {
		return h.snapshotLocked()
	}
	h.lastAdjust = time.Now()

	batch := make([]contracts.CognitiveMetrics, 0, len(h.metrics))
	for _, m := range h.metrics {
		batch = append(batch, m)
	}
	load := h.load.Compute(batch)

	// Homeostatic control law: scale concurrency with load, with smooth
	// bounds so the system neither starves nor thrashes.
	reservoirWorkers := scaleInt(load, 2, 64)
	cycleHz := 0.5 + 1.5*load // 0.5Hz at rest → 2Hz under load
	runnerWorkers := scaleInt(load, 1, 16)

	h.targets["ecco9-reservoir"] = contracts.ThreadPoolDirective{
		ServiceName:   "ecco9-reservoir",
		TargetWorkers: int32(reservoirWorkers),
		Reason:        "global cognitive load",
	}
	h.targets["ecco9-echobeats"] = contracts.ThreadPoolDirective{
		ServiceName:      "ecco9-echobeats",
		TargetWorkers:    int32(scaleInt(load, 1, 8)),
		CycleFrequencyHz: cycleHz,
		Reason:           "adaptive cognitive cadence",
	}
	h.targets["ecco9-runner"] = contracts.ThreadPoolDirective{
		ServiceName:   "ecco9-runner",
		TargetWorkers: int32(runnerWorkers),
		Reason:        "inference parallelism",
	}
	return h.snapshotLocked()
}

func (h *HomeostaticController) snapshotLocked() []contracts.ThreadPoolDirective {
	out := make([]contracts.ThreadPoolDirective, 0, len(h.targets))
	for _, d := range h.targets {
		out = append(out, d)
	}
	return out
}

func scaleInt(load float64, lo, hi int) int {
	if load < 0 {
		load = 0
	}
	if load > 1 {
		load = 1
	}
	return lo + int(float64(hi-lo)*load)
}
