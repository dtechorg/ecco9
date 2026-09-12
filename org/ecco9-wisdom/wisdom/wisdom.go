// Package wisdom implements seven-dimensional wisdom cultivation metrics,
// autonomous goal orchestration, interest-driven goal generation, and the
// wisdom application engine.
//
// Adapted from core/wisdom/{metrics,seven_dimensional}.go,
// core/goals/{goal_orchestrator,interest_driven_generator}.go, and
// core/deeptreeecho/wisdom_application.go in the ecco9 monorepo.
package wisdom

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// WisdomDimension is one of the seven wisdom dimensions.
type WisdomDimension int

const (
	DimKnowledgeDepth WisdomDimension = iota
	DimKnowledgeBreadth
	DimIntegrationLevel
	DimPracticalApplication
	DimReflectiveInsight
	DimEthicalConsideration
	DimTemporalPerspective
)

// DimensionNames mirrors WisdomDimension.String() in the monorepo.
var DimensionNames = []string{
	"Knowledge Depth", "Knowledge Breadth", "Integration Level",
	"Practical Application", "Reflective Insight", "Ethical Consideration",
	"Temporal Perspective",
}

// dimensionWeights are the weighted-average coefficients per Echo9 spec.
var dimensionWeights = []float64{0.15, 0.15, 0.20, 0.15, 0.15, 0.10, 0.10}

// DimensionState tracks a single wisdom dimension.
type DimensionState struct {
	Value       float64
	Trend       float64
	LastUpdate  time.Time
	UpdateCount int64
	History     []float64
	TargetValue float64
}

// SevenDimensionalWisdom is the complete seven-dimensional wisdom
// cultivation tracker.
type SevenDimensionalWisdom struct {
	mu sync.RWMutex

	dimensions map[WisdomDimension]*DimensionState

	overallWisdom  float64
	coherenceScore float64
	evolutionRate  float64

	snapshots  []map[WisdomDimension]float64
	lastUpdate time.Time
}

// NewSevenDimensionalWisdom creates the tracker with modest baselines.
func NewSevenDimensionalWisdom() *SevenDimensionalWisdom {
	sdw := &SevenDimensionalWisdom{
		dimensions: make(map[WisdomDimension]*DimensionState),
		snapshots:  make([]map[WisdomDimension]float64, 0, 1000),
		lastUpdate: time.Now(),
	}
	for dim := DimKnowledgeDepth; dim <= DimTemporalPerspective; dim++ {
		sdw.dimensions[dim] = &DimensionState{
			Value:       0.3, // start with modest baseline
			History:     make([]float64, 0, 100),
			TargetValue: 0.8, // aim high
			LastUpdate:  time.Now(),
		}
	}
	return sdw
}

// Update updates wisdom dimensions from cognitive state signals and
// recomputes overall wisdom, coherence, and evolution rate.
func (sdw *SevenDimensionalWisdom) Update(graphDepth, graphBreadth, edgeDensity, skillProficiency, aarCoherence, moralityScore, goalTimeHorizon float64) {
	sdw.mu.Lock()
	defer sdw.mu.Unlock()

	now := time.Now()
	sdw.updateDimension(DimKnowledgeDepth, graphDepth, now)
	sdw.updateDimension(DimKnowledgeBreadth, graphBreadth, now)
	sdw.updateDimension(DimIntegrationLevel, edgeDensity, now)
	sdw.updateDimension(DimPracticalApplication, skillProficiency, now)
	sdw.updateDimension(DimReflectiveInsight, aarCoherence, now)
	sdw.updateDimension(DimEthicalConsideration, moralityScore, now)
	sdw.updateDimension(DimTemporalPerspective, goalTimeHorizon, now)

	// Overall wisdom (weighted average per Echo9 spec).
	sdw.overallWisdom = 0
	for dim := DimKnowledgeDepth; dim <= DimTemporalPerspective; dim++ {
		sdw.overallWisdom += sdw.dimensions[dim].Value * dimensionWeights[dim]
	}

	sdw.coherenceScore = sdw.calculateCoherence()
	sdw.evolutionRate = sdw.calculateEvolutionRate()
	sdw.takeSnapshot()
	sdw.lastUpdate = now
}

func (sdw *SevenDimensionalWisdom) updateDimension(dim WisdomDimension, value float64, now time.Time) {
	state := sdw.dimensions[dim]
	value = math.Max(0.0, math.Min(1.0, value))
	if len(state.History) > 0 {
		state.Trend = value - state.Value
	}
	state.History = append(state.History, value)
	if len(state.History) > 100 {
		state.History = state.History[1:]
	}
	state.Value = value
	state.LastUpdate = now
	state.UpdateCount++
}

// calculateCoherence maps variance across dimensions to [0,1] via
// exponential decay (0 variance = 1.0 coherence).
func (sdw *SevenDimensionalWisdom) calculateCoherence() float64 {
	mean := 0.0
	for dim := DimKnowledgeDepth; dim <= DimTemporalPerspective; dim++ {
		mean += sdw.dimensions[dim].Value
	}
	mean /= 7.0
	variance := 0.0
	for dim := DimKnowledgeDepth; dim <= DimTemporalPerspective; dim++ {
		diff := sdw.dimensions[dim].Value - mean
		variance += diff * diff
	}
	variance /= 7.0
	return math.Exp(-variance * 10.0)
}

// calculateEvolutionRate measures wisdom growth between the last two
// snapshots (per hour).
func (sdw *SevenDimensionalWisdom) calculateEvolutionRate() float64 {
	n := len(sdw.snapshots)
	if n < 1 {
		return 0.0
	}
	prev := sdw.snapshots[n-1]
	current := make(map[WisdomDimension]float64, 7)
	for dim := DimKnowledgeDepth; dim <= DimTemporalPerspective; dim++ {
		current[dim] = sdw.dimensions[dim].Value
	}
	dt := time.Since(sdw.lastUpdate).Hours()
	if dt <= 0 {
		return 0.0
	}
	delta := 0.0
	for dim := DimKnowledgeDepth; dim <= DimTemporalPerspective; dim++ {
		delta += current[dim] - prev[dim]
	}
	return delta / 7.0 / dt
}

func (sdw *SevenDimensionalWisdom) takeSnapshot() {
	snap := make(map[WisdomDimension]float64, 7)
	for dim := DimKnowledgeDepth; dim <= DimTemporalPerspective; dim++ {
		snap[dim] = sdw.dimensions[dim].Value
	}
	sdw.snapshots = append(sdw.snapshots, snap)
	if len(sdw.snapshots) > 1000 {
		sdw.snapshots = sdw.snapshots[1:]
	}
}

// Assessment is the seven-dimensional wisdom snapshot for GetWisdom.
type Assessment struct {
	Dimensions []float64
	Overall    float64
	Coherence  float64
	Evolution  float64
	AssessedAt time.Time
}

// Assess returns the current seven-dimensional assessment.
func (sdw *SevenDimensionalWisdom) Assess() *Assessment {
	sdw.mu.RLock()
	defer sdw.mu.RUnlock()
	dims := make([]float64, 7)
	for dim := DimKnowledgeDepth; dim <= DimTemporalPerspective; dim++ {
		dims[dim] = sdw.dimensions[dim].Value
	}
	return &Assessment{
		Dimensions: dims,
		Overall:    sdw.overallWisdom,
		Coherence:  sdw.coherenceScore,
		Evolution:  sdw.evolutionRate,
		AssessedAt: sdw.lastUpdate,
	}
}

// GoalType mirrors the GoalType enum in wisdom/v1/wisdom.proto.
type GoalType int

const (
	GoalTypeUnspecified GoalType = iota
	GoalTypeExploration
	GoalTypeConsolidation
	GoalTypeRepair
	GoalTypeGrowth
)

// Goal is an autonomous goal with progress tracking (adapted from
// goals.Goal; Category maps onto the proto GoalType).
type Goal struct {
	ID          string
	Type        GoalType
	Description string
	Priority    float64
	Progress    float64
	CreatedAt   time.Time
	CompletedAt *time.Time
	Completed   bool
}

// GoalOrchestrator manages autonomous goal generation and pursuit.
type GoalOrchestrator struct {
	mu           sync.RWMutex
	activeGoals  []*Goal
	completed    []*Goal
	maxActive    int
	goalsCreated uint64
	counter      uint64
}

// NewGoalOrchestrator creates the orchestrator (max 5 active goals, as in
// the monorepo's generateGoalsFromIdentity guard).
func NewGoalOrchestrator() *GoalOrchestrator {
	return &GoalOrchestrator{maxActive: 5}
}

// CreateGoal registers a new goal, generating an ID when absent.
func (o *GoalOrchestrator) CreateGoal(g Goal) (string, error) {
	o.mu.Lock()
	defer o.mu.Unlock()

	active := 0
	for _, ag := range o.activeGoals {
		if !ag.Completed {
			active++
		}
	}
	if active >= o.maxActive {
		return "", fmt.Errorf("active goal limit (%d) reached", o.maxActive)
	}

	o.counter++
	if g.ID == "" {
		g.ID = fmt.Sprintf("goal-%d-%d", time.Now().Unix(), o.counter)
	}
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now()
	}
	g.Progress = math.Max(0, math.Min(1, g.Progress))
	goal := g
	o.activeGoals = append(o.activeGoals, &goal)
	o.goalsCreated++
	return goal.ID, nil
}

// ListGoals returns goals, optionally filtered by type (unspecified = all).
func (o *GoalOrchestrator) ListGoals(typeFilter GoalType) []*Goal {
	o.mu.RLock()
	defer o.mu.RUnlock()
	out := make([]*Goal, 0, len(o.activeGoals))
	for _, g := range o.activeGoals {
		if typeFilter == GoalTypeUnspecified || g.Type == typeFilter {
			out = append(out, g)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Priority > out[j].Priority })
	return out
}

// AdvanceGoal updates progress, completing the goal at 1.0.
func (o *GoalOrchestrator) AdvanceGoal(id string, delta float64) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	for _, g := range o.activeGoals {
		if g.ID == id && !g.Completed {
			g.Progress = math.Max(0, math.Min(1, g.Progress+delta))
			if g.Progress >= 1.0 {
				now := time.Now()
				g.Completed = true
				g.CompletedAt = &now
				o.completed = append(o.completed, g)
			}
			return true
		}
	}
	return false
}

// InterestPattern tracks interest in a topic for goal generation.
type InterestPattern struct {
	Topic           string
	Strength        float64
	Recency         float64
	Depth           float64
	Novelty         float64
	Utility         float64
	LastEngaged     time.Time
	EngagementCount int
}

// InterestDrivenGenerator creates goals from curiosity and interest
// patterns (adapted from goals.InterestDrivenGoalGenerator).
type InterestDrivenGenerator struct {
	mu                   sync.RWMutex
	interestPatterns     map[string]*InterestPattern
	curiosityLevel       float64
	minInterestThreshold float64
}

// NewInterestDrivenGenerator seeds the interest landscape from Deep Tree
// Echo's canonical curiosity topics.
func NewInterestDrivenGenerator() *InterestDrivenGenerator {
	g := &InterestDrivenGenerator{
		interestPatterns:     make(map[string]*InterestPattern),
		curiosityLevel:       0.7,
		minInterestThreshold: 0.4,
	}
	seedTopics := []string{
		"pattern recognition", "wisdom cultivation", "cognitive architectures",
		"consciousness studies", "knowledge integration", "autonomous learning",
		"creative problem solving", "meta-cognition", "temporal reasoning",
		"social understanding",
	}
	for _, topic := range seedTopics {
		g.interestPatterns[topic] = &InterestPattern{
			Topic: topic, Strength: 0.6, Recency: 0.5, Depth: 0.2,
			Novelty: 0.8, Utility: 0.6, LastEngaged: time.Now(),
		}
	}
	return g
}

// interestScore is the composite: strength .4, novelty .3, utility .2,
// recency .1 (as in the monorepo).
func (g *InterestDrivenGenerator) interestScore(p *InterestPattern) float64 {
	return p.Strength*0.4 + p.Novelty*0.3 + p.Utility*0.2 + p.Recency*0.1
}

// StrongestInterests returns up to count topics above the interest
// threshold, strongest first.
func (g *InterestDrivenGenerator) StrongestInterests(count int) []*InterestPattern {
	g.mu.RLock()
	defer g.mu.RUnlock()
	scored := make([]*InterestPattern, 0)
	for _, p := range g.interestPatterns {
		if g.interestScore(p) > g.minInterestThreshold {
			scored = append(scored, p)
		}
	}
	sort.Slice(scored, func(i, j int) bool {
		return g.interestScore(scored[i]) > g.interestScore(scored[j])
	})
	if len(scored) > count {
		scored = scored[:count]
	}
	return scored
}

// RecordEngagement strengthens an interest on engagement and decays
// novelty, mirroring the monorepo's interest dynamics.
func (g *InterestDrivenGenerator) RecordEngagement(topic string, strength float64) {
	g.mu.Lock()
	defer g.mu.Unlock()
	p, ok := g.interestPatterns[topic]
	if !ok {
		p = &InterestPattern{Topic: topic, Novelty: 1.0, Recency: 0.5, Depth: 0.1, Utility: 0.5}
		g.interestPatterns[topic] = p
	}
	p.EngagementCount++
	p.LastEngaged = time.Now()
	p.Recency = 1.0
	p.Strength = math.Min(1.0, p.Strength+0.1*strength)
	p.Depth = math.Min(1.0, p.Depth+0.05*strength)
	p.Novelty = math.Max(0.0, p.Novelty-0.05)
}

// WisdomEntry is a piece of applicable wisdom.
type WisdomEntry struct {
	ID            string
	Content       string
	Category      string
	Applicability []string
	Confidence    float64
	Usefulness    float64
	Generality    float64
	Specificity   float64
	TimesApplied  uint64
	CreatedAt     time.Time
	LastApplied   time.Time
}

// ApplicationEngine matches contexts to relevant wisdom (adapted from
// deeptreeecho.WisdomApplicationEngine).
type ApplicationEngine struct {
	mu         sync.RWMutex
	wisdomBase map[string]*WisdomEntry
	counter    uint64
}

// NewApplicationEngine creates the engine seeded with foundational wisdom.
func NewApplicationEngine() *ApplicationEngine {
	e := &ApplicationEngine{wisdomBase: make(map[string]*WisdomEntry)}
	foundational := []struct {
		content, category string
		applicability     []string
	}{
		{"Learn from experience and adapt continuously", "learning", []string{"growth", "development", "improvement"}},
		{"Understand context before applying solutions", "problem_solving", []string{"decision_making", "analysis", "planning"}},
		{"Balance exploration with exploitation", "strategy", []string{"learning", "optimization", "discovery"}},
		{"Recognize patterns but remain open to novelty", "perception", []string{"learning", "creativity", "adaptation"}},
	}
	for _, f := range foundational {
		e.counter++
		e.wisdomBase[fmt.Sprintf("wisdom-%d", e.counter)] = &WisdomEntry{
			ID: fmt.Sprintf("wisdom-%d", e.counter), Content: f.content,
			Category: f.category, Applicability: f.applicability,
			Confidence: 0.9, Usefulness: 0.8, Generality: 0.9, Specificity: 0.3,
			CreatedAt: time.Now(), LastApplied: time.Now(),
		}
	}
	return e
}

// AddWisdom stores a new wisdom entry and returns its ID.
func (e *ApplicationEngine) AddWisdom(entry WisdomEntry) string {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.counter++
	if entry.ID == "" {
		entry.ID = fmt.Sprintf("wisdom-%d", e.counter)
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	w := entry
	e.wisdomBase[w.ID] = &w
	return w.ID
}

// WisdomMatch pairs an entry with its applicability score for a context.
type WisdomMatch struct {
	Entry         *WisdomEntry
	Applicability float64
}

// FindRelevantWisdom returns the top-K entries whose applicability to the
// context exceeds 0.3, scored as in the monorepo's calculateRelevance:
// applicability .4, timeliness .2, specificity .2, novelty .1,
// usefulness .1.
func (e *ApplicationEngine) FindRelevantWisdom(context string, topK int) []*WisdomMatch {
	e.mu.RLock()
	defer e.mu.RUnlock()

	matches := make([]*WisdomMatch, 0)
	for _, w := range e.wisdomBase {
		applicability := calculateApplicability(context, w)
		timeliness := 1.0 / (1.0 + math.Log(1.0+time.Since(w.LastApplied).Hours()/24.0))
		novelty := 1.0 - float64(w.TimesApplied)/100.0
		overall := applicability*0.4 + timeliness*0.2 + w.Specificity*0.2 +
			novelty*0.1 + w.Usefulness*0.1
		if overall > 0.3 {
			matches = append(matches, &WisdomMatch{Entry: w, Applicability: overall})
		}
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Applicability > matches[j].Applicability })
	if len(matches) > topK {
		matches = matches[:topK]
	}

	// Record the application (last-applied recency and use count).
	e.mu.RUnlock()
	e.mu.Lock()
	for _, m := range matches {
		m.Entry.TimesApplied++
		m.Entry.LastApplied = time.Now()
	}
	e.mu.Unlock()
	e.mu.RLock()

	return matches
}

// calculateApplicability scores term overlap between context and the
// entry's applicability contexts (+0.3 per hit, capped at 1.0).
func calculateApplicability(context string, w *WisdomEntry) float64 {
	score := 0.0
	lower := strings.ToLower(context)
	for _, applicable := range w.Applicability {
		if strings.Contains(lower, strings.ToLower(applicable)) {
			score += 0.3
		}
	}
	return math.Min(1.0, score)
}
