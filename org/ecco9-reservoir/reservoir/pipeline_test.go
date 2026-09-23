package reservoir

import (
	"math"
	"testing"
)

// newTrainedPipeline returns a pipeline whose trainer has learned the
// identity mapping on a 1-D signal well enough to pass the promotion gate.
func newTrainedPipeline() *TrainingPipeline {
	r := New(16, PersonaCautiousAnalyst)
	tr := NewOnlineTrainer(r, 1, 0.99)
	p := NewTrainingPipeline(tr)
	p.MaxValidationMSE = 1.0 // lenient gate for a few-shot trainer
	for i := 0; i < 30; i++ {
		x := float64(i%5) / 5.0
		tr.TrainSample([]float64{x}, []float64{x})
	}
	return p
}

func TestCheckpointVersionsAndPromotesFirst(t *testing.T) {
	p := newTrainedPipeline()
	cp := p.Checkpoint()
	if cp.Version == 0 {
		t.Fatal("expected non-zero checkpoint version")
	}
	if got := p.Deployed(); got != cp.Version {
		t.Fatalf("first checkpoint should become deployed baseline, got %d want %d", got, cp.Version)
	}
	if len(cp.Weights) != 1 {
		t.Fatalf("expected 1 output row of weights, got %d", len(cp.Weights))
	}
	// Mutating the returned weights must not affect the stored checkpoint.
	cp.Weights[0][0] = 999
	again := p.Checkpoint()
	if again.Weights[0][0] == 999 {
		t.Fatal("checkpoint weights must be deep-copied")
	}
}

func TestValidatePromotesWithinGate(t *testing.T) {
	p := newTrainedPipeline()
	cp := p.Checkpoint()
	heldOut := []ValidationSample{
		{Input: []float64{0.2}, Target: []float64{0.2}},
		{Input: []float64{0.6}, Target: []float64{0.6}},
	}
	got, ok := p.Validate(cp.Version, heldOut)
	if !ok {
		t.Fatal("validate should find the checkpoint")
	}
	if !got.HasHeldOut {
		t.Fatal("validate should record held-out evaluation")
	}
	if math.IsNaN(got.HeldOutMSE) {
		t.Fatal("held-out MSE should be a number")
	}
	if p.Deployed() != cp.Version {
		t.Fatalf("checkpoint within gate should be promoted, deployed=%d", p.Deployed())
	}
}

func TestValidateRejectsUnknownVersion(t *testing.T) {
	p := newTrainedPipeline()
	if _, ok := p.Validate(4242, nil); ok {
		t.Fatal("validate of unknown version should fail")
	}
}

func TestValidateDoesNotPromoteBeyondGate(t *testing.T) {
	p := newTrainedPipeline()
	p.MaxValidationMSE = 0.0 // impossible gate
	first := p.Checkpoint()
	baseline := p.Deployed()
	if baseline != first.Version {
		t.Fatalf("first checkpoint should be the deployed baseline: %d", baseline)
	}
	// A later checkpoint with held-out error beyond the gate must not displace
	// the baseline.
	for i := 0; i < 5; i++ {
		p.trainer.TrainSample([]float64{1.0}, []float64{-1.0})
	}
	later := p.Checkpoint()
	if later.Version == first.Version {
		t.Fatal("expected a later checkpoint version")
	}
	if _, ok := p.Validate(later.Version, []ValidationSample{
		{Input: []float64{0.5}, Target: []float64{-3.0}}, // deliberately wrong target
	}); !ok {
		t.Fatal("validate should find the checkpoint")
	}
	if p.Deployed() != baseline {
		t.Fatalf("checkpoint beyond gate must not be promoted: deployed moved %d -> %d", baseline, p.Deployed())
	}
}

func TestReloadRestoresVersionedWeights(t *testing.T) {
	p := newTrainedPipeline()
	cp := p.Checkpoint()

	// Corrupt the live weights via more training, then reload.
	for i := 0; i < 10; i++ {
		p.trainer.TrainSample([]float64{1.0}, []float64{-1.0})
	}
	if !p.Reload(cp.Version) {
		t.Fatal("reload should find the checkpoint")
	}
	w, samples, _, _ := p.trainer.Snapshot()
	if samples != cp.Samples {
		t.Fatalf("reload should restore sample count %d, got %d", cp.Samples, samples)
	}
	if math.Abs(w[0][0]-cp.Weights[0][0]) > 1e-12 {
		t.Fatal("reload should restore versioned weights")
	}
	if p.Reload(9999) {
		t.Fatal("reload of unknown version should fail")
	}
}
