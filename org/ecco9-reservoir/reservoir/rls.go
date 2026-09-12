package reservoir

import (
	"math"
	"sync"
)

// OnlineTrainer implements Recursive Least Squares (RLS) for continuous
// training of the reservoir readout weights without retraining the fixed
// internal reservoir. This is the "online reservoir training" pipeline step 3.
type OnlineTrainer struct {
	mu sync.Mutex

	reservoir *EchoStateReservoir

	// RLS state
	forgettingFactor float64     // lambda, e.g. 0.99
	p                [][]float64 // inverse correlation matrix
	w                [][]float64 // readout weights [outDim][stateDim+1] (+bias)
	outDim           int
	stateDim         int

	samplesTrained uint64
	lastError      float64
	version        uint64
}

// NewOnlineTrainer binds a trainer to a reservoir with the given output
// dimension and RLS forgetting factor.
func NewOnlineTrainer(r *EchoStateReservoir, outDim int, forgettingFactor float64) *OnlineTrainer {
	if forgettingFactor <= 0 || forgettingFactor > 1 {
		forgettingFactor = 0.99
	}
	stateDim := r.size
	// Initialize P as delta*I (large initial uncertainty).
	p := make([][]float64, stateDim+1)
	for i := range p {
		p[i] = make([]float64, stateDim+1)
		p[i][i] = 1000.0
	}
	w := make([][]float64, outDim)
	for i := range w {
		w[i] = make([]float64, stateDim+1)
	}
	return &OnlineTrainer{
		reservoir:        r,
		forgettingFactor: forgettingFactor,
		p:                p,
		w:                w,
		outDim:           outDim,
		stateDim:         stateDim,
	}
}

// TrainSample performs one RLS update given an input/target pair.
// Returns the a priori prediction error.
func (t *OnlineTrainer) TrainSample(input, target []float64) float64 {
	if len(target) != t.outDim {
		return math.NaN()
	}
	// Advance reservoir and augment state with bias.
	state := t.reservoir.Update(input)
	phi := make([]float64, t.stateDim+1)
	copy(phi, state)
	phi[t.stateDim] = 1.0

	t.mu.Lock()
	defer t.mu.Unlock()

	lambda := t.forgettingFactor

	// k = P·phi / (lambda + phi'·P·phi)
	pPhi := matVec(t.p, phi)
	denom := lambda + dot(phi, pPhi)
	if denom == 0 {
		return math.NaN()
	}
	k := make([]float64, len(pPhi))
	for i := range pPhi {
		k[i] = pPhi[i] / denom
	}

	// Prediction error for each output: e = y - w·phi
	var errSum float64
	for o := 0; o < t.outDim; o++ {
		pred := dot(t.w[o], phi)
		e := target[o] - pred
		errSum += e * e
		// w = w + e·k
		for i := range t.w[o] {
			t.w[o][i] += e * k[i]
		}
	}

	// P = (P - k·phi'·P) / lambda
	phiTP := vecMat(phi, t.p)
	for i := range t.p {
		for j := range t.p[i] {
			t.p[i][j] = (t.p[i][j] - k[i]*phiTP[j]) / lambda
		}
	}

	t.samplesTrained++
	t.version++
	t.lastError = math.Sqrt(errSum / float64(t.outDim))
	return t.lastError
}

// Predict runs the readout on the current reservoir state.
func (t *OnlineTrainer) Predict(input []float64) []float64 {
	state := t.reservoir.Update(input)
	phi := make([]float64, t.stateDim+1)
	copy(phi, state)
	phi[t.stateDim] = 1.0

	t.mu.Lock()
	defer t.mu.Unlock()
	out := make([]float64, t.outDim)
	for o := 0; o < t.outDim; o++ {
		out[o] = dot(t.w[o], phi)
	}
	return out
}

// Stats returns training statistics and the checkpoint version.
func (t *OnlineTrainer) Stats() (samples uint64, lastError float64, version uint64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.samplesTrained, t.lastError, t.version
}

func dot(a, b []float64) float64 {
	var s float64
	for i := range a {
		s += a[i] * b[i]
	}
	return s
}

func matVec(m [][]float64, v []float64) []float64 {
	out := make([]float64, len(m))
	for i := range m {
		out[i] = dot(m[i], v)
	}
	return out
}

func vecMat(v []float64, m [][]float64) []float64 {
	cols := len(m[0])
	out := make([]float64, cols)
	for j := 0; j < cols; j++ {
		var s float64
		for i := range v {
			s += v[i] * m[i][j]
		}
		out[j] = s
	}
	return out
}
