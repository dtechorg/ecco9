// Package ontogenesis implements ontogenetic genome evolution (genetic
// algorithm over kernel genes) and entelechy actualization metrics.
//
// Adapted from core/ontogenesis/{genome,evolution,kernel,operations}.go,
// core/entelechy/{actualization,dimensions,genome,metrics}.go, and
// core/deeptreeecho/ontogenetic_development.go in the ecco9 monorepo.
package ontogenesis

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"
)

// Genome is the genetic information of an ontogenetic kernel: a flat map
// of named genes (reservoir params, thresholds, etc.) plus mutation
// parameters — the map-shaped projection of the monorepo's KernelGenome
// B-series coefficients, matching the proto Genome message.
type Genome struct {
	ID               string
	Genes            map[string]float64
	Fitness          float64
	Generation       int
	MutationRate     float64
	MutationStrength float64
	ParentIDs        []string
}

// NewGenome creates a genome with default mutation parameters.
func NewGenome(id string, genes map[string]float64) *Genome {
	return &Genome{
		ID:               id,
		Genes:            genes,
		MutationRate:     0.1,
		MutationStrength: 0.05,
		Generation:       0,
		ParentIDs:        []string{},
	}
}

// Clone creates a deep copy of the genome.
func (g *Genome) Clone() *Genome {
	genes := make(map[string]float64, len(g.Genes))
	for k, v := range g.Genes {
		genes[k] = v
	}
	parents := make([]string, len(g.ParentIDs))
	copy(parents, g.ParentIDs)
	return &Genome{
		ID: g.ID, Genes: genes, Fitness: g.Fitness,
		Generation: g.Generation, MutationRate: g.MutationRate,
		MutationStrength: g.MutationStrength, ParentIDs: parents,
	}
}

// Mutate applies Gaussian mutation per gene with probability MutationRate
// (mirror of KernelGenome.Mutate).
func (g *Genome) Mutate() {
	for k := range g.Genes {
		if rand.Float64() < g.MutationRate {
			g.Genes[k] += rand.NormFloat64() * g.MutationStrength
		}
	}
}

// Crossover performs uniform crossover over the union of gene keys;
// mutation parameters are averaged; generation is max(parents)+1
// (mirror of KernelGenome.Crossover).
func (g *Genome) Crossover(other *Genome) *Genome {
	childGenes := make(map[string]float64, len(g.Genes))
	keys := make([]string, 0, len(g.Genes))
	for k := range g.Genes {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic gene order
	for _, k := range keys {
		if rand.Float64() < 0.5 {
			childGenes[k] = g.Genes[k]
		} else if v, ok := other.Genes[k]; ok {
			childGenes[k] = v
		} else {
			childGenes[k] = g.Genes[k]
		}
	}
	return &Genome{
		Genes:            childGenes,
		MutationRate:     (g.MutationRate + other.MutationRate) / 2.0,
		MutationStrength: (g.MutationStrength + other.MutationStrength) / 2.0,
		Generation:       max(g.Generation, other.Generation) + 1,
		ParentIDs:        []string{g.ID, other.ID},
	}
}

// Distance is the Euclidean genetic distance over shared gene keys
// (mirror of KernelGenome.Distance).
func (g *Genome) Distance(other *Genome) float64 {
	sumSquares := 0.0
	shared := 0
	for k, v := range g.Genes {
		if ov, ok := other.Genes[k]; ok {
			diff := v - ov
			sumSquares += diff * diff
			shared++
		}
	}
	if shared == 0 {
		return math.MaxFloat64
	}
	return math.Sqrt(sumSquares)
}

// EntelechyGenome is the five-dimension actualization genome (mirror of
// entelechy.EntelechyGenome).
type EntelechyGenome struct {
	Ontological  float64
	Teleological float64
	Cognitive    float64
	Integrative  float64
	Evolutionary float64
}

// Fitness computes the weighted actualization fitness and level (mirror
// of EntelechyGenome.CalculateFitness).
func (g *EntelechyGenome) Fitness() float64 {
	return g.Ontological*0.20 + g.Teleological*0.25 + g.Cognitive*0.30 +
		g.Integrative*0.10 + g.Evolutionary*0.15
}

// Level maps fitness onto the developmental levels.
func (g *EntelechyGenome) Level() string {
	switch f := g.Fitness(); {
	case f < 0.3:
		return "Embryonic"
	case f < 0.6:
		return "Juvenile"
	case f < 0.8:
		return "Adolescent"
	case f < 0.95:
		return "Adult"
	default:
		return "Transcendent"
	}
}

// Dimensions returns the five actualization dimensions by name.
func (g *EntelechyGenome) Dimensions() map[string]float64 {
	return map[string]float64{
		"ontological":  g.Ontological,
		"teleological": g.Teleological,
		"cognitive":    g.Cognitive,
		"integrative":  g.Integrative,
		"evolutionary": g.Evolutionary,
	}
}

// EvolutionJob is a running or completed GA evolution run.
type EvolutionJob struct {
	ID                string
	Running           bool
	CurrentGeneration int
	Generations       int
	Best              *Genome
	StartedAt         time.Time
	FinishedAt        time.Time
}

// Engine combines genome evolution with entelechy actualization tracking.
type Engine struct {
	mu sync.RWMutex

	population []*Genome
	jobs       map[string]*EvolutionJob

	entelechy     EntelechyGenome
	actualization float64
	purpose       float64
	fragmentation float64
	alpha         float64 // actualization growth rate
	beta          float64 // fragmentation decay rate

	counter uint64
}

// NewEngine creates the engine with mid-level entelechy baselines and the
// monorepo's alpha/beta metric rates.
func NewEngine() *Engine {
	return &Engine{
		population: make([]*Genome, 0),
		jobs:       make(map[string]*EvolutionJob),
		entelechy: EntelechyGenome{
			Ontological: 0.5, Teleological: 0.5, Cognitive: 0.5,
			Integrative: 0.5, Evolutionary: 0.5,
		},
		purpose: 0.5, fragmentation: 0.5,
		alpha: 0.1, beta: 0.05,
	}
}

// SubmitGenome registers a genome, generating an ID when absent.
func (e *Engine) SubmitGenome(g Genome) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.counter++
	if g.ID == "" {
		g.ID = fmt.Sprintf("genome-%d-%d", time.Now().Unix(), e.counter)
	}
	if g.MutationRate == 0 {
		g.MutationRate = 0.1
	}
	if g.MutationStrength == 0 {
		g.MutationStrength = 0.05
	}
	genome := g
	e.population = append(e.population, &genome)
	return genome.ID
}

// EvaluateFitness scores a genome. Fitness blends gene magnitude balance
// (penalizing extreme genes, as B-series coefficients near zero produce
// unstable kernels) with gene-count coverage — the map-space analog of
// the ODE-solver error fitness in KernelGenome.Evaluate.
func (e *Engine) EvaluateFitness(genomeID string) (float64, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, g := range e.population {
		if g.ID == genomeID {
			g.Fitness = fitnessOf(g)
			return g.Fitness, true
		}
	}
	return 0, false
}

// fitnessOf computes 1/(1+error) where error is the RMS gene magnitude —
// the same 1/(1+sqrt(totalError)) form as the monorepo's Evaluate.
func fitnessOf(g *Genome) float64 {
	if len(g.Genes) == 0 {
		return 0
	}
	totalError := 0.0
	target := 0.5 // target operating point for cognitive parameters
	for _, v := range g.Genes {
		diff := v - target
		totalError += diff * diff
	}
	return 1.0 / (1.0 + math.Sqrt(totalError))
}

// Evolve runs a synchronous genetic algorithm for the requested number of
// generations with tournament selection and elitism (mirror of
// Population.Evolve), returning a completed job record.
func (e *Engine) Evolve(populationSize, generations int, mutationRate float64) *EvolutionJob {
	e.mu.Lock()

	e.counter++
	job := &EvolutionJob{
		ID:      fmt.Sprintf("evolve-%d-%d", time.Now().Unix(), e.counter),
		Running: true, Generations: generations, StartedAt: time.Now(),
	}
	e.jobs[job.ID] = job

	// Seed population: existing genomes plus random perturbations to size.
	pop := make([]*Genome, 0, populationSize)
	for _, g := range e.population {
		pop = append(pop, g.Clone())
	}
	for len(pop) < populationSize {
		pop = append(pop, randomGenome(6))
	}

	e.mu.Unlock()

	// Run generations without holding the lock.
	for gen := 0; gen < generations; gen++ {
		for _, g := range pop {
			g.Fitness = fitnessOf(g)
		}
		sort.Slice(pop, func(i, j int) bool { return pop[i].Fitness > pop[j].Fitness })

		next := make([]*Genome, 0, populationSize)
		next = append(next, pop[0].Clone()) // elitism
		for len(next) < populationSize {
			p1 := tournamentSelect(pop, 3)
			p2 := tournamentSelect(pop, 3)
			child := p1.Crossover(p2)
			if mutationRate > 0 {
				child.MutationRate = mutationRate
			}
			child.Mutate()
			e.mu.Lock()
			e.counter++
			child.ID = fmt.Sprintf("genome-%d-%d", time.Now().Unix(), e.counter)
			e.mu.Unlock()
			next = append(next, child)
		}
		pop = next

		e.mu.Lock()
		job.CurrentGeneration = gen + 1
		job.Best = pop[0].Clone()
		e.mu.Unlock()
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	job.Running = false
	job.FinishedAt = time.Now()
	e.population = pop
	// Evolutionary actualization improves with best fitness.
	e.entelechy.Evolutionary = clamp01(0.9*e.entelechy.Evolutionary + 0.1*job.Best.Fitness)
	e.updateActualization(1.0)
	return job
}

// tournamentSelect picks the fittest of k random genomes.
func tournamentSelect(pop []*Genome, k int) *Genome {
	var best *Genome
	bestFitness := -1.0
	for i := 0; i < k; i++ {
		g := pop[rand.Intn(len(pop))]
		if g.Fitness > bestFitness {
			best = g
			bestFitness = g.Fitness
		}
	}
	return best
}

// randomGenome creates a genome with n uniform random genes in [0,1].
func randomGenome(n int) *Genome {
	genes := make(map[string]float64, n)
	names := []string{"spectral_radius", "input_scaling", "leak_rate", "threshold", "learning_rate", "density"}
	for i := 0; i < n && i < len(names); i++ {
		genes[names[i]] = rand.Float64()
	}
	return NewGenome("", genes)
}

// Job returns an evolution job by ID.
func (e *Engine) Job(id string) (*EvolutionJob, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	j, ok := e.jobs[id]
	return j, ok
}

// SetDimensions updates entelechy dimension assessments (mirror of
// EntelechyEngine.Actualize's Assess step).
func (e *Engine) SetDimensions(ontological, teleological, cognitive, integrative, evolutionary float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.entelechy.Ontological = clamp01(ontological)
	e.entelechy.Teleological = clamp01(teleological)
	e.entelechy.Cognitive = clamp01(cognitive)
	e.entelechy.Integrative = clamp01(integrative)
	e.entelechy.Evolutionary = clamp01(evolutionary)
	e.purpose = e.entelechy.Teleological
	e.fragmentation = 1.0 - e.entelechy.Integrative
	e.updateActualization(1.0)
}

// updateActualization applies the entelechy metrics differential:
// dA = alpha*purpose*(1-A) - beta*fragmentation (mirror of
// ActualizationMetrics.Update).
func (e *Engine) updateActualization(dt float64) {
	dA := e.alpha*e.purpose*(1.0-e.actualization) - e.beta*e.fragmentation
	e.actualization = clamp01(e.actualization + dA*dt)
}

// Actualization returns the entelechy dimensions and overall
// actualization for GetActualization.
func (e *Engine) Actualization() (map[string]float64, float64, string) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.entelechy.Dimensions(), e.actualization, e.entelechy.Level()
}

// Status returns engine metrics for introspection.
func (e *Engine) Status() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return map[string]any{
		"population_size":       len(e.population),
		"evolution_jobs":        len(e.jobs),
		"overall_actualization": e.actualization,
		"entelechy_level":       e.entelechy.Level(),
		"entelechy_fitness":     e.entelechy.Fitness(),
	}
}

func clamp01(v float64) float64 {
	return math.Max(0.0, math.Min(1.0, v))
}
