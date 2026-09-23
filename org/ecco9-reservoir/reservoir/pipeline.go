package reservoir

import (
	"math"
	"sync"
	"time"
)

// Checkpoint is a versioned, immutable snapshot of the RLS readout weights.
// Checkpoints are promoted only after held-out validation passes and can be
// hot-reloaded into the live trainer without downtime (pipeline step 5).
type Checkpoint struct {
	Version    uint64
	Weights    [][]float64
	Samples    uint64
	TrainError float64
	HeldOutMSE float64
	HasHeldOut bool
	CreatedAt  time.Time
}

// ValidationSample is a held-out input/target pair used to evaluate
// training quality before a checkpoint is promoted (pipeline step 4).
type ValidationSample struct {
	Input  []float64 `json:"input"`
	Target []float64 `json:"target"`
}

// TrainingPipeline wraps the OnlineTrainer with the deployment side of the
// online reservoir training pipeline: versioned checkpoints, held-out
// validation, and hot-reload.
type TrainingPipeline struct {
	mu sync.RWMutex

	trainer *OnlineTrainer

	checkpoints map[uint64]*Checkpoint
	latest      uint64
	deployed    uint64

	// MaxValidationMSE is the promotion gate: checkpoints whose held-out
	// error exceeds the threshold are kept versioned but not promoted.
	MaxValidationMSE float64
}

// NewTrainingPipeline binds a pipeline to an online trainer.
func NewTrainingPipeline(t *OnlineTrainer) *TrainingPipeline {
	return &TrainingPipeline{
		trainer:          t,
		checkpoints:      make(map[uint64]*Checkpoint),
		MaxValidationMSE: 0.5,
	}
}

// Checkpoint captures the current readout weights as a new version.
func (p *TrainingPipeline) Checkpoint() *Checkpoint {
	weights, samples, trainErr, version := p.trainer.Snapshot()
	cp := &Checkpoint{
		Version:    version,
		Weights:    weights,
		Samples:    samples,
		TrainError: trainErr,
		CreatedAt:  time.Now().UTC(),
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	first := len(p.checkpoints) == 0
	p.checkpoints[version] = cp
	p.latest = version
	if first {
		// First checkpoint becomes the deployed baseline.
		p.deployed = version
	}
	return copyCheckpoint(cp)
}

// Validate scores a checkpoint against held-out samples (mean squared
// prediction error) and promotes it to the deployed version when the error
// is within MaxValidationMSE. Returns the recorded checkpoint.
func (p *TrainingPipeline) Validate(version uint64, heldOut []ValidationSample) (*Checkpoint, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()

	cp, ok := p.checkpoints[version]
	if !ok {
		return nil, false
	}

	mse := heldOutMSE(cp.Weights, p.trainer, heldOut)
	cp.HeldOutMSE = mse
	cp.HasHeldOut = true
	if len(heldOut) > 0 && !math.IsNaN(mse) && mse <= p.MaxValidationMSE {
		p.deployed = version
	}
	return copyCheckpoint(cp), true
}

// Deployed returns the currently deployed checkpoint version.
func (p *TrainingPipeline) Deployed() uint64 {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.deployed
}

// Reload hot-swaps the trainer's readout weights with a versioned
// checkpoint without interrupting inference (pipeline step 5).
func (p *TrainingPipeline) Reload(version uint64) bool {
	p.mu.RLock()
	cp, ok := p.checkpoints[version]
	p.mu.RUnlock()
	if !ok {
		return false
	}
	p.trainer.LoadWeights(cp.Weights, cp.Samples, cp.TrainError, cp.Version)
	return true
}

// heldOutMSE runs the reservoir forward for each held-out pair and scores
// the checkpoint's readout weights without mutating trainer state. The
// reservoir state advance is inherent to ESN evaluation.
func heldOutMSE(weights [][]float64, t *OnlineTrainer, heldOut []ValidationSample) float64 {
	if len(heldOut) == 0 {
		return math.NaN()
	}
	var sum float64
	var n int
	for _, s := range heldOut {
		pred := t.PredictWith(weights, s.Input)
		if len(pred) != len(s.Target) {
			continue
		}
		for o := range pred {
			d := s.Target[o] - pred[o]
			sum += d * d
			n++
		}
	}
	if n == 0 {
		return math.NaN()
	}
	return sum / float64(n)
}

func copyCheckpoint(cp *Checkpoint) *Checkpoint {
	out := *cp
	out.Weights = copyWeights(cp.Weights)
	return &out
}

func copyWeights(w [][]float64) [][]float64 {
	out := make([][]float64, len(w))
	for i := range w {
		out[i] = make([]float64, len(w[i]))
		copy(out[i], w[i])
	}
	return out
}
