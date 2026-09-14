package reservoir

import (
	"math"
	"testing"
	"time"
)

func TestUpdateProducesBoundedState(t *testing.T) {
	r := New(50, PersonaContemplativeScholar)
	input := []float64{0.5, -0.3, 0.8, 0.1}
	state := r.Update(input)
	if len(state) != 50 {
		t.Fatalf("expected state of size 50, got %d", len(state))
	}
	for i, v := range state {
		if math.IsNaN(v) || math.IsInf(v, 0) {
			t.Fatalf("state[%d] is not finite: %v", i, v)
		}
		if v < -1.0 || v > 1.0 {
			t.Fatalf("state[%d]=%v out of tanh bounds [-1,1]", i, v)
		}
	}
}

func TestStateIsIsolatedCopy(t *testing.T) {
	r := New(20, PersonaDynamicExplorer)
	r.Update([]float64{1.0, 0.0})
	s1 := r.State()
	s1[0] = 999.0
	if r.State()[0] == 999.0 {
		t.Fatal("State() leaked the internal buffer")
	}
}

func TestSetPersonaReconfiguresDynamics(t *testing.T) {
	r := New(30, PersonaContemplativeScholar)
	if got := r.Persona(); got != PersonaContemplativeScholar {
		t.Fatalf("expected scholar persona, got %v", got)
	}
	r.SetPersona(PersonaCautiousAnalyst)
	if got := r.Persona(); got != PersonaCautiousAnalyst {
		t.Fatalf("persona not switched, got %v", got)
	}
	// CautiousAnalyst has spectral radius 0.99 — weights should be re-scaled.
	cfg := PersonaConfigs[PersonaCautiousAnalyst]
	if cfg.SpectralRadius != 0.99 {
		t.Fatalf("unexpected analyst config: %+v", cfg)
	}
}

func TestMetricsInRange(t *testing.T) {
	r := New(40, PersonaCreativeVisionary)
	for i := 0; i < 20; i++ {
		r.Update([]float64{float64(i%3) * 0.3, 0.1})
	}
	ep, cx := r.Metrics()
	if ep < 0 || ep > 1 {
		t.Fatalf("echo property out of range: %v", ep)
	}
	if cx < 0 || cx > 1 {
		t.Fatalf("complexity out of range: %v", cx)
	}
}

func TestOnlineTrainerLearnsMapping(t *testing.T) {
	r := New(64, PersonaContemplativeScholar)
	tr := NewOnlineTrainer(r, 1, 0.99)

	input := []float64{0.4, 0.4}
	target := []float64{0.9}
	// Drive the reservoir to a steady state, then train repeatedly on the
	// same (input -> target) pair; RLS readout error should shrink.
	var first, last float64
	for i := 0; i < 60; i++ {
		e := tr.TrainSample(input, target)
		if i == 0 {
			first = e
		}
		last = e
	}
	if math.IsNaN(last) {
		t.Fatal("trainer returned NaN error")
	}
	if last > first {
		t.Fatalf("expected error to decrease: first=%v last=%v", first, last)
	}
	samples, _, version := tr.Stats()
	if samples != 60 || version != 60 {
		t.Fatalf("expected 60 samples/version, got %d/%d", samples, version)
	}
}

func TestOnlineTrainerRejectsBadTargetDim(t *testing.T) {
	r := New(16, PersonaDynamicExplorer)
	tr := NewOnlineTrainer(r, 2, 0.99)
	if e := tr.TrainSample([]float64{1}, []float64{1, 2, 3}); !math.IsNaN(e) {
		t.Fatalf("expected NaN for mismatched target dim, got %v", e)
	}
}

func TestThreadPoolScalesWithinBounds(t *testing.T) {
	c := NewThreadPoolController(2, 8)
	defer c.Close()

	if _, cur := c.SetWorkers(6); cur != 6 {
		t.Fatalf("expected 6 workers, got %d", cur)
	}
	if _, cur := c.SetWorkers(100); cur != 8 {
		t.Fatalf("expected clamp to max 8, got %d", cur)
	}
	if _, cur := c.SetWorkers(0); cur != 2 {
		t.Fatalf("expected clamp to min 2, got %d", cur)
	}
}

func TestThreadPoolExecutesJobs(t *testing.T) {
	c := NewThreadPoolController(2, 4)
	defer c.Close()
	done := make(chan struct{}, 1)
	c.Submit(func() { done <- struct{}{} })
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("submitted job was not executed")
	}
}
