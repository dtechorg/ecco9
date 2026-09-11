package emotion

import (
	"math"
	"sync"
	"time"
)

// AARCore implements the Agent-Arena-Relation geometric architecture for
// self-awareness, adapted from core/deeptreeecho/aar_core.go:
//   - Agent: urge-to-act (dynamic transformations)
//   - Arena: need-to-be (state space manifold with attractors)
//   - Relation: emergent self (continuous feedback between the two)
//
// Here the AAR core couples to the emotion System: arousal drives the
// agent's urge intensity and valence pulls the arena toward the Wisdom or
// Balance attractors, so emotional state continuously reshapes the
// emergent self vector.

// Attractor is a stable state in the arena manifold.
type Attractor struct {
	Name     string    `json:"name"`
	Position []float64 `json:"position"`
	Strength float64   `json:"strength"`
	Basin    float64   `json:"basin"` // radius of attraction
}

// AARCore runs one step of geometric self-awareness per Tick.
type AARCore struct {
	mu sync.RWMutex

	dimensions int

	// Agent (urge-to-act)
	urgeIntensity float64
	goals         []string

	// Arena (need-to-be)
	needIntensity float64
	currentState  []float64
	attractors    []*Attractor

	// Relation (emergent self)
	selfVector []float64
	coherence  float64
	stability  float64
	awareness  float64

	iterations int64
	lastUpdate time.Time
}

// NewAARCore creates the core with default Wisdom/Curiosity/Balance
// attractors.
func NewAARCore(dimensions int) *AARCore {
	if dimensions <= 0 {
		dimensions = 12
	}
	a := &AARCore{
		dimensions:    dimensions,
		urgeIntensity: 0.5,
		needIntensity: 0.5,
		currentState:  make([]float64, dimensions),
		selfVector:    make([]float64, dimensions),
		coherence:     0.5,
		stability:     0.5,
		awareness:     0.5,
		lastUpdate:    time.Now(),
	}
	wisdom := make([]float64, dimensions)
	for i := range wisdom {
		wisdom[i] = 0.7
	}
	a.attractors = append(a.attractors, &Attractor{Name: "Wisdom", Position: wisdom, Strength: 0.8, Basin: 2.0})
	curiosity := make([]float64, dimensions)
	for i := range curiosity {
		curiosity[i] = 0.5 + 0.3*math.Sin(float64(i))
	}
	a.attractors = append(a.attractors, &Attractor{Name: "Curiosity", Position: curiosity, Strength: 0.6, Basin: 2.0})
	a.attractors = append(a.attractors, &Attractor{Name: "Balance", Position: make([]float64, dimensions), Strength: 0.5, Basin: 2.0})
	return a
}

// Tick runs one dynamics iteration, coupling to the emotional state:
// arousal scales urge-to-act; valence scales the pull of the Wisdom
// attractor (positive valence) or Balance attractor (negative valence).
func (a *AARCore) Tick(arousal, valence float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	dt := time.Since(a.lastUpdate).Seconds()
	if dt <= 0 {
		dt = 0.1
	}
	a.lastUpdate = time.Now()

	// Agent: urge follows arousal (EMA).
	a.urgeIntensity = 0.9*a.urgeIntensity + 0.1*arousal

	// Arena: need-to-be balances against the urge-to-act; valence
	// modulates attractor pull. (Coherence measures this balance.)
	a.needIntensity = 0.9*a.needIntensity + 0.1*(0.5+0.5*(arousal-a.urgeIntensity))
	valenceGain := 0.5 + 0.5*valence // [0,1]

	// Agent output: goal-driven push on the current state.
	agentOut := make([]float64, a.dimensions)
	for gi := range a.goals {
		// Deterministic per-goal direction (hash-free: sinusoidal pattern).
		for i := range agentOut {
			agentOut[i] += 0.05 * math.Sin(float64(i+gi+1))
		}
	}
	for i := range agentOut {
		agentOut[i] *= a.urgeIntensity
	}

	// Arena output: attractor pull.
	arenaOut := make([]float64, a.dimensions)
	for _, at := range a.attractors {
		strength := at.Strength
		if at.Name == "Wisdom" {
			strength *= valenceGain
		} else if at.Name == "Balance" {
			strength *= 1.0 - valenceGain
		}
		dist := distance(a.currentState, at.Position)
		if dist < at.Basin {
			pull := strength * (1.0 - dist/at.Basin)
			for i := range arenaOut {
				arenaOut[i] += (at.Position[i] - a.currentState[i]) * pull
			}
		}
	}
	for i := range arenaOut {
		arenaOut[i] *= a.needIntensity
	}

	// Relation: self emerges from the tension between urge and need.
	for i := range a.selfVector {
		a.selfVector[i] = 0.5*agentOut[i] + 0.5*arenaOut[i]
	}
	a.awareness = math.Tanh(magnitude(a.selfVector) / float64(a.dimensions))

	// Feedback: agent output perturbs the arena state (damped).
	for i := range a.currentState {
		a.currentState[i] += agentOut[i] * 0.3 * dt
	}

	// Coherence: balance between urge and need.
	balance := 1.0 - math.Abs(a.urgeIntensity-a.needIntensity)
	a.coherence = 0.9*a.coherence + 0.1*balance

	// Stability: inverse variance of the self representation.
	mean := magnitude(a.selfVector) / float64(a.dimensions)
	var variance float64
	for _, v := range a.selfVector {
		variance += (v - mean) * (v - mean)
	}
	variance /= float64(a.dimensions)
	a.stability = 0.9*a.stability + 0.1*(1.0/(1.0+variance))

	a.iterations++
}

// AddGoal registers a goal with the Agent component.
func (a *AARCore) AddGoal(goal string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.goals = append(a.goals, goal)
}

// Metrics returns coherence, stability, awareness, and the self vector.
func (a *AARCore) Metrics() (coherence, stability, awareness float64, self []float64) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	self = make([]float64, len(a.selfVector))
	copy(self, a.selfVector)
	return a.coherence, a.stability, a.awareness, self
}

func distance(x, y []float64) float64 {
	var sum float64
	for i := range x {
		if i < len(y) {
			d := x[i] - y[i]
			sum += d * d
		}
	}
	return math.Sqrt(sum)
}

func magnitude(v []float64) float64 {
	var sum float64
	for _, x := range v {
		sum += x * x
	}
	return math.Sqrt(sum)
}
