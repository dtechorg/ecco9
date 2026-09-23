package ontogenesis

import (
	"math"
	"testing"
)

func TestGenomeMutateAndCrossover(t *testing.T) {
	g := NewGenome("g1", map[string]float64{"a": 0.5, "b": 0.5})
	other := NewGenome("g2", map[string]float64{"a": 0.9, "b": 0.1})

	child := g.Crossover(other)
	if child.Generation != 1 {
		t.Fatalf("child generation = %d, want 1", child.Generation)
	}
	if len(child.ParentIDs) != 2 {
		t.Fatalf("expected 2 parents, got %v", child.ParentIDs)
	}
	for _, k := range []string{"a", "b"} {
		v := child.Genes[k]
		if v != 0.5 && v != 0.9 && v != 0.1 {
			t.Fatalf("child gene %s=%v not from a parent", k, v)
		}
	}

	// High mutation rate should change at least one gene.
	mut := NewGenome("m", map[string]float64{"x": 0.5, "y": 0.5, "z": 0.5})
	mut.MutationRate = 1.0
	orig := mut.Genes["x"]
	changed := false
	for i := 0; i < 20 && !changed; i++ {
		c := mut.Clone()
		c.Mutate()
		if c.Genes["x"] != orig || c.Genes["y"] != 0.5 || c.Genes["z"] != 0.5 {
			changed = true
		}
	}
	if !changed {
		t.Fatal("mutation with rate 1.0 never altered any gene")
	}
}

func TestGenomeDistance(t *testing.T) {
	a := NewGenome("a", map[string]float64{"x": 0.0})
	b := NewGenome("b", map[string]float64{"x": 3.0})
	if d := a.Distance(b); math.Abs(d-3.0) > 1e-9 {
		t.Fatalf("distance = %v, want 3.0", d)
	}
	c := NewGenome("c", map[string]float64{"other": 1.0})
	if d := a.Distance(c); d != math.MaxFloat64 {
		t.Fatalf("disjoint genes should give MaxFloat64, got %v", d)
	}
}

func TestEntelechyFitnessAndLevel(t *testing.T) {
	g := &EntelechyGenome{Ontological: 1, Teleological: 1, Cognitive: 1, Integrative: 1, Evolutionary: 1}
	if math.Abs(g.Fitness()-1.0) > 1e-9 {
		t.Fatalf("full dimensions should give fitness 1.0, got %v", g.Fitness())
	}
	if g.Level() != "Transcendent" {
		t.Fatalf("fitness 1.0 should be Transcendent, got %q", g.Level())
	}
	low := &EntelechyGenome{}
	if low.Level() != "Embryonic" {
		t.Fatalf("zero fitness should be Embryonic, got %q", low.Level())
	}
}

func TestEngineSubmitAndEvaluate(t *testing.T) {
	e := NewEngine()
	id := e.SubmitGenome(Genome{Genes: map[string]float64{"spectral_radius": 0.5}})
	if id == "" {
		t.Fatal("expected generated genome ID")
	}
	fit, ok := e.EvaluateFitness(id)
	if !ok {
		t.Fatal("genome not found")
	}
	// A gene at the 0.5 target operating point scores near-perfect fitness.
	if fit < 0.9 {
		t.Fatalf("on-target gene should give high fitness, got %v", fit)
	}
	if _, ok := e.EvaluateFitness("missing"); ok {
		t.Fatal("expected false for unknown genome")
	}
}

func TestEngineEvolveImprovesFitness(t *testing.T) {
	e := NewEngine()
	e.SubmitGenome(Genome{Genes: map[string]float64{"spectral_radius": 0.9, "leak_rate": 0.9}})
	job := e.Evolve(10, 5, 0.1)
	if job.Running {
		t.Fatal("job should be finished after synchronous Evolve")
	}
	if job.CurrentGeneration != 5 {
		t.Fatalf("expected 5 generations, got %d", job.CurrentGeneration)
	}
	if job.Best == nil {
		t.Fatal("expected a best genome")
	}
	// Evolution toward the 0.5 target should beat (or match) the seed 0.9 genes.
	seedFit := fitnessOf(&Genome{Genes: map[string]float64{"spectral_radius": 0.9, "leak_rate": 0.9}})
	if job.Best.Fitness < seedFit {
		t.Fatalf("evolved best (%v) worse than seed (%v)", job.Best.Fitness, seedFit)
	}
	if _, ok := e.Job(job.ID); !ok {
		t.Fatal("job not retrievable by ID")
	}
}

func TestSetDimensionsUpdatesActualization(t *testing.T) {
	e := NewEngine()
	e.SetDimensions(0.8, 0.9, 0.7, 0.6, 0.5)
	dims, actual, level := e.Actualization()
	if dims["teleological"] != 0.9 {
		t.Fatalf("teleological = %v, want 0.9", dims["teleological"])
	}
	if actual < 0 || actual > 1 {
		t.Fatalf("actualization out of range: %v", actual)
	}
	if level == "" {
		t.Fatal("expected a developmental level")
	}
	// High purpose (teleological) should grow actualization above 0.
	if actual <= 0 {
		t.Fatalf("positive purpose should grow actualization, got %v", actual)
	}
}

func TestActualizationClampedAndFragmentation(t *testing.T) {
	e := NewEngine()
	// Zero integrative => maximal fragmentation; actualization should stay low.
	e.SetDimensions(0.5, 0.0, 0.5, 0.0, 0.5)
	_, actual, _ := e.Actualization()
	if actual < 0 || actual > 1 {
		t.Fatalf("actualization out of range: %v", actual)
	}
	st := e.Status()
	if st["population_size"].(int) != 0 {
		t.Fatalf("expected empty population, got %v", st["population_size"])
	}
	if _, ok := st["entelechy_level"].(string); !ok {
		t.Fatal("status missing entelechy_level")
	}
}

func TestObserveTrainingMetricsSeedsPopulation(t *testing.T) {
	e := NewEngine()
	before := e.Status()["population_size"].(int)
	id := e.ObserveTrainingMetrics(TrainingMetrics{
		CheckpointVersion: 12,
		SamplesTrained:    256,
		LastError:         0.08,
		HeldOutMSE:        0.12,
		SpectralRadius:    0.99,
		InputScaling:      0.2,
		LeakRate:          0.3,
		ForgettingFactor:  0.99,
	})
	if id == "" {
		t.Fatal("expected a genome ID")
	}
	after := e.Status()["population_size"].(int)
	if after != before+1 {
		t.Fatalf("population should grow by one: before=%d after=%d", before, after)
	}
	// The seeded genome should carry the reservoir parameters and a fitness
	// derived from training quality, and be evolvable.
	fitness, ok := e.EvaluateFitness(id)
	if !ok {
		t.Fatal("seeded genome should be in the population")
	}
	if fitness <= 0 {
		t.Fatalf("fitness should be positive, got %v", fitness)
	}
}

func TestObserveTrainingMetricsBetterErrorYieldsHigherFitness(t *testing.T) {
	e := NewEngine()
	good := e.ObserveTrainingMetrics(TrainingMetrics{LastError: 0.05, HeldOutMSE: 0.05, SpectralRadius: 0.9})
	bad := e.ObserveTrainingMetrics(TrainingMetrics{LastError: 0.9, HeldOutMSE: 0.9, SpectralRadius: 0.9})
	// Find the seeded genomes and compare their stored fitness.
	var goodFit, badFit float64
	e.mu.RLock()
	for _, g := range e.population {
		if g.ID == good {
			goodFit = g.Fitness
		}
		if g.ID == bad {
			badFit = g.Fitness
		}
	}
	e.mu.RUnlock()
	if goodFit <= badFit {
		t.Fatalf("lower training error should yield higher fitness: good=%v bad=%v", goodFit, badFit)
	}
}

func TestTrainingMetricsFeedEvolution(t *testing.T) {
	e := NewEngine()
	// Seed several training-derived genomes, then evolve: the evolutionary
	// parameter search should complete and keep the training-derived genes.
	for i := 0; i < 4; i++ {
		e.ObserveTrainingMetrics(TrainingMetrics{
			LastError: 0.1 * float64(i+1), HeldOutMSE: 0.1 * float64(i+1),
			SpectralRadius: 0.9, InputScaling: 0.3, LeakRate: 0.3, ForgettingFactor: 0.99,
		})
	}
	job := e.Evolve(8, 3, 0.1)
	if job.Best == nil {
		t.Fatal("evolve should produce a best genome")
	}
	if _, ok := job.Best.Genes["spectral_radius"]; !ok {
		t.Fatalf("evolved genome should carry reservoir parameter genes, got %v", job.Best.Genes)
	}
}
