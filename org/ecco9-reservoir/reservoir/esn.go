// Package reservoir implements the Echo State Network temporal core,
// extracted from core/deeptreeecho/echo_state_reservoir.go and extended with
// online training (recursive least squares) and an adaptive thread pool
// controller.
package reservoir

import (
	"math"
	"math/rand"
	"sync"
)

// PersonaType selects a reservoir configuration.
type PersonaType int

const (
	PersonaContemplativeScholar PersonaType = iota
	PersonaDynamicExplorer
	PersonaCautiousAnalyst
	PersonaCreativeVisionary
)

// PersonaConfig holds spectral/leak parameters for a persona.
type PersonaConfig struct {
	SpectralRadius float64
	InputScaling   float64
	LeakRate       float64
	Description    string
}

// PersonaConfigs mirrors the source monolith's persona table.
var PersonaConfigs = map[PersonaType]PersonaConfig{
	PersonaContemplativeScholar: {0.95, 0.3, 0.2, "Deep memory, slow deliberation"},
	PersonaDynamicExplorer:      {0.7, 0.8, 0.8, "Low memory, rapid adaptation"},
	PersonaCautiousAnalyst:      {0.99, 0.2, 0.3, "Maximal stability, conservative"},
	PersonaCreativeVisionary:    {0.85, 0.7, 0.6, "Edge of chaos, transformation-seeking"},
}

// EchoStateReservoir is the fixed-weights temporal reservoir.
type EchoStateReservoir struct {
	mu sync.RWMutex

	spectralRadius float64
	inputScaling   float64
	leakRate       float64

	size         int
	state        []float64
	weights      [][]float64
	inputWeights [][]float64

	// Online readout (trained via RLS).
	readout [][]float64
	outDim  int

	persona PersonaType

	history    [][]float64
	maxHistory int

	echoProperty float64
	complexity   float64
}

// New creates a reservoir of the given size for a persona.
func New(size int, persona PersonaType) *EchoStateReservoir {
	cfg := PersonaConfigs[persona]
	r := &EchoStateReservoir{
		spectralRadius: cfg.SpectralRadius,
		inputScaling:   cfg.InputScaling,
		leakRate:       cfg.LeakRate,
		size:           size,
		state:          make([]float64, size),
		persona:        persona,
		maxHistory:     100,
	}
	r.initWeights()
	return r
}

func (r *EchoStateReservoir) initWeights() {
	r.weights = make([][]float64, r.size)
	for i := range r.weights {
		r.weights[i] = make([]float64, r.size)
		for j := range r.weights[i] {
			r.weights[i][j] = (rand.Float64()*2 - 1) * 0.5 * r.spectralRadius
		}
	}
}

// Update advances the reservoir with leaky tanh integration.
// x(t+1) = (1-α)x(t) + α·tanh(W_in·u + W·x(t))
func (r *EchoStateReservoir) Update(input []float64) []float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	if len(r.inputWeights) == 0 {
		r.initInputWeights(len(input))
	}

	next := make([]float64, r.size)
	for i := 0; i < r.size; i++ {
		var inSum float64
		for j := 0; j < len(input) && j < len(r.inputWeights[i]); j++ {
			inSum += r.inputWeights[i][j] * input[j]
		}
		inSum *= r.inputScaling

		var resSum float64
		row := r.weights[i]
		for j := 0; j < r.size; j++ {
			resSum += row[j] * r.state[j]
		}
		next[i] = (1-r.leakRate)*r.state[i] + r.leakRate*math.Tanh(inSum+resSum)
	}
	r.state = next
	r.record(next)
	r.updateMetrics()
	return next
}

func (r *EchoStateReservoir) initInputWeights(dim int) {
	r.inputWeights = make([][]float64, r.size)
	for i := range r.inputWeights {
		r.inputWeights[i] = make([]float64, dim)
		for j := range r.inputWeights[i] {
			r.inputWeights[i][j] = (rand.Float64()*2 - 1) * 0.5
		}
	}
}

func (r *EchoStateReservoir) record(s []float64) {
	cp := make([]float64, len(s))
	copy(cp, s)
	r.history = append(r.history, cp)
	if len(r.history) > r.maxHistory {
		r.history = r.history[1:]
	}
}

func (r *EchoStateReservoir) updateMetrics() {
	r.echoProperty = r.calcEchoProperty()
	r.complexity = r.calcComplexity()
}

func (r *EchoStateReservoir) calcEchoProperty() float64 {
	if len(r.history) < 2 {
		return 1.0
	}
	recent := r.history[len(r.history)-1]
	prev := r.history[len(r.history)-2]
	var change float64
	for i := range recent {
		d := recent[i] - prev[i]
		change += d * d
	}
	change = math.Sqrt(change / float64(len(recent)))
	optimal := 0.2
	ep := 1 - math.Abs(change-optimal)/optimal
	return math.Max(0, math.Min(1, ep))
}

func (r *EchoStateReservoir) calcComplexity() float64 {
	if len(r.history) < 10 {
		return 0.5
	}
	means := make([]float64, r.size)
	for _, s := range r.history {
		for i, v := range s {
			means[i] += v
		}
	}
	n := float64(len(r.history))
	for i := range means {
		means[i] /= n
	}
	var avgVar float64
	for _, s := range r.history {
		for i, v := range s {
			d := v - means[i]
			avgVar += d * d
		}
	}
	avgVar /= n * float64(r.size)
	return math.Min(avgVar*2, 1)
}

// State returns a copy of the current reservoir state.
func (r *EchoStateReservoir) State() []float64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]float64, len(r.state))
	copy(out, r.state)
	return out
}

// Metrics returns echo property and complexity.
func (r *EchoStateReservoir) Metrics() (echoProperty, complexity float64) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.echoProperty, r.complexity
}

// SetPersona reconfigures spectral radius, input scaling, and leak rate.
func (r *EchoStateReservoir) SetPersona(p PersonaType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cfg := PersonaConfigs[p]
	r.persona = p
	r.spectralRadius = cfg.SpectralRadius
	r.inputScaling = cfg.InputScaling
	r.leakRate = cfg.LeakRate
	r.initWeights()
}

// Persona returns the current persona.
func (r *EchoStateReservoir) Persona() PersonaType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.persona
}
