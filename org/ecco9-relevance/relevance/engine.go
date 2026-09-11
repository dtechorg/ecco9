// Package relevance implements the Relevance Realization Ennead — a
// triad-of-triads meta-framework for relevance realization integrating
// ways of knowing (epistemological), orders of understanding
// (ontological), and practices of wisdom (axiological).
//
// Adapted from core/relevance/{engine,knowing_triad,understanding_triad,
// wisdom_triad,realization_process}.go in the ecco9 monorepo.
package relevance

import (
	"math"
	"sync"
	"time"
)

// KnowingTriad implements Triad I: the four ways of knowing.
type KnowingTriad struct {
	Propositional float64 // Knowing-that (facts, beliefs)
	Procedural    float64 // Knowing-how (skills, abilities)
	Perspectival  float64 // Knowing-as (framing, salience)
	Participatory float64 // Knowing-by-being (transformation)

	GnosticIntegration float64

	PropositionalLearningRate float64
	ProceduralLearningRate    float64
	PerspectivalLearningRate  float64
	ParticipatoryLearningRate float64
}

// NewKnowingTriad creates a Ways of Knowing triad with Vervaeke's
// learning rates (participatory is slowest — transformation takes time).
func NewKnowingTriad() *KnowingTriad {
	return &KnowingTriad{
		Propositional: 0.5, Procedural: 0.5, Perspectival: 0.5, Participatory: 0.5,
		GnosticIntegration:        0.5,
		PropositionalLearningRate: 0.1,
		ProceduralLearningRate:    0.08,
		PerspectivalLearningRate:  0.12,
		ParticipatoryLearningRate: 0.05,
	}
}

// KnowingAnalysis is input analyzed through the four ways of knowing.
type KnowingAnalysis struct {
	PropositionalScore float64
	ProceduralScore    float64
	PerspectivalScore  float64
	ParticipatoryScore float64
	OverallScore       float64
}

// Analyze scores input through all four ways of knowing.
func (kt *KnowingTriad) Analyze() *KnowingAnalysis {
	a := &KnowingAnalysis{
		PropositionalScore: kt.Propositional * 0.8,
		ProceduralScore:    kt.Procedural * 0.85,
		PerspectivalScore:  kt.Perspectival * 0.9,
		ParticipatoryScore: kt.Participatory * 0.7,
	}
	a.OverallScore = a.PropositionalScore*0.25 + a.ProceduralScore*0.25 +
		a.PerspectivalScore*0.3 + a.ParticipatoryScore*0.2
	return a
}

// UnderstandingTriad implements Triad II: the three orders of understanding.
type UnderstandingTriad struct {
	Nomological float64 // How things work (causal-scientific)
	Normative   float64 // What matters (evaluative-ethical)
	Narrative   float64 // How things develop (temporal-historical)

	MeaningIntegration float64
}

// NewUnderstandingTriad creates an Orders of Understanding triad.
func NewUnderstandingTriad() *UnderstandingTriad {
	return &UnderstandingTriad{
		Nomological: 0.5, Normative: 0.5, Narrative: 0.5,
		MeaningIntegration: 0.5,
	}
}

// UnderstandingAnalysis is input analyzed through the three orders.
type UnderstandingAnalysis struct {
	NomologicalScore float64
	NormativeScore   float64
	NarrativeScore   float64
	OverallScore     float64
}

// Analyze scores input through all three orders of understanding.
func (ut *UnderstandingTriad) Analyze() *UnderstandingAnalysis {
	a := &UnderstandingAnalysis{
		NomologicalScore: ut.Nomological * 0.85,
		NormativeScore:   ut.Normative * 0.9,
		NarrativeScore:   ut.Narrative * 0.8,
	}
	a.OverallScore = a.NomologicalScore*0.3 + a.NormativeScore*0.35 + a.NarrativeScore*0.35
	return a
}

// updateMeaningIntegration uses a geometric mean — all three orders must
// be present for meaning.
func (ut *UnderstandingTriad) updateMeaningIntegration() {
	ut.MeaningIntegration = math.Pow(ut.Nomological*ut.Normative*ut.Narrative, 1.0/3.0)
}

// WisdomTriad implements Triad III: the three Ms of wisdom.
type WisdomTriad struct {
	Morality float64 // Virtue and character excellence
	Meaning  float64 // Coherence and purpose
	Mastery  float64 // Excellence and flow

	Eudaimonia float64 // Flourishing — integration of the three Ms
}

// NewWisdomTriad creates a Practices of Wisdom triad.
func NewWisdomTriad() *WisdomTriad {
	return &WisdomTriad{Morality: 0.5, Meaning: 0.5, Mastery: 0.5, Eudaimonia: 0.5}
}

// WisdomAnalysis is input analyzed through the three Ms.
type WisdomAnalysis struct {
	MoralityScore float64
	MeaningScore  float64
	MasteryScore  float64
	OverallScore  float64
}

// Analyze scores input through the three Ms of wisdom.
func (wt *WisdomTriad) Analyze() *WisdomAnalysis {
	a := &WisdomAnalysis{
		MoralityScore: wt.Morality * 0.85,
		MeaningScore:  wt.Meaning * 0.9,
		MasteryScore:  wt.Mastery * 0.8,
	}
	a.OverallScore = a.MoralityScore*0.35 + a.MeaningScore*0.35 + a.MasteryScore*0.3
	return a
}

// updateEudaimonia uses a weighted geometric mean — all three Ms must be
// present, weighted slightly toward morality and meaning.
func (wt *WisdomTriad) updateEudaimonia() {
	wt.Eudaimonia = math.Pow(
		math.Pow(wt.Morality, 1.2)*math.Pow(wt.Meaning, 1.2)*wt.Mastery,
		1.0/3.4,
	)
}

// RealizationProcess is the meta-process of relevance realization across
// all nine dimensions of the Ennead.
type RealizationProcess struct {
	contextWeights    map[string]float64
	salienceLandscape map[string]float64
	relevanceHistory  []float64
	maxHistorySize    int
}

// NewRealizationProcess creates a realization process with balanced
// triad weights.
func NewRealizationProcess() *RealizationProcess {
	return &RealizationProcess{
		contextWeights:    map[string]float64{"knowing": 0.33, "understanding": 0.33, "wisdom": 0.34},
		salienceLandscape: make(map[string]float64),
		relevanceHistory:  make([]float64, 0, 100),
		maxHistorySize:    100,
	}
}

// CalculateRelevance integrates the three triad analyses and modulates
// with the current salience landscape.
func (rp *RealizationProcess) CalculateRelevance(ka *KnowingAnalysis, ua *UnderstandingAnalysis, wa *WisdomAnalysis) float64 {
	relevance := ka.OverallScore*rp.contextWeights["knowing"] +
		ua.OverallScore*rp.contextWeights["understanding"] +
		wa.OverallScore*rp.contextWeights["wisdom"]

	relevance = rp.modulateWithSalience(relevance)
	rp.storeRelevance(relevance)
	return relevance
}

// modulateWithSalience amplifies/dampens relevance by average salience.
func (rp *RealizationProcess) modulateWithSalience(base float64) float64 {
	avgSalience := 0.0
	count := 0
	for _, s := range rp.salienceLandscape {
		avgSalience += s
		count++
	}
	if count > 0 {
		avgSalience /= float64(count)
		return base * (0.7 + 0.3*avgSalience)
	}
	return base
}

func (rp *RealizationProcess) storeRelevance(r float64) {
	if len(rp.relevanceHistory) >= rp.maxHistorySize {
		rp.relevanceHistory = rp.relevanceHistory[1:]
	}
	rp.relevanceHistory = append(rp.relevanceHistory, r)
}

// UpdateSalience refreshes the salience landscape from the ennead state,
// boosting underdeveloped dimensions (< 0.4) that need attention.
func (rp *RealizationProcess) UpdateSalience(dims map[string]float64) {
	for dim, value := range dims {
		if value < 0.4 {
			value += (0.4 - value) * 0.5
		}
		rp.salienceLandscape[dim] = value
	}
}

// Salience returns a copy of the current salience landscape.
func (rp *RealizationProcess) Salience() map[string]float64 {
	out := make(map[string]float64, len(rp.salienceLandscape))
	for k, v := range rp.salienceLandscape {
		out[k] = v
	}
	return out
}

// History returns the most recent n relevance scores.
func (rp *RealizationProcess) History(n int) []float64 {
	l := len(rp.relevanceHistory)
	if n > l {
		n = l
	}
	out := make([]float64, n)
	copy(out, rp.relevanceHistory[l-n:])
	return out
}

// Engine is the Relevance Realization Ennead scoring engine.
type Engine struct {
	mu sync.RWMutex

	knowing      *KnowingTriad
	understanding *UnderstandingTriad
	wisdom       *WisdomTriad
	realization  *RealizationProcess

	// Integration metrics
	OverallCoherence      float64
	RelevanceOptimization float64
	totalScored           uint64
	lastUpdate            time.Time
}

// NewEngine creates a relevance engine with all triads at baseline 0.5.
func NewEngine() *Engine {
	return &Engine{
		knowing:               NewKnowingTriad(),
		understanding:         NewUnderstandingTriad(),
		wisdom:                NewWisdomTriad(),
		realization:           NewRealizationProcess(),
		OverallCoherence:      0.5,
		RelevanceOptimization: 0.5,
		lastUpdate:            time.Now(),
	}
}

// Score is the result of scoring one event for relevance.
type Score struct {
	Relevance  float64
	Salience   float64
	Affordance float64
	Facets     []string
}

// RealizeRelevance scores an input through all nine dimensions and the
// realization meta-process. Mirrors Engine.RealizeRelevance in the
// monorepo, plus salience and affordance for the ScoreResponse contract.
func (e *Engine) RealizeRelevance(topic string, context map[string]string) *Score {
	e.mu.Lock()
	defer e.mu.Unlock()

	ka := e.knowing.Analyze()
	ua := e.understanding.Analyze()
	wa := e.wisdom.Analyze()

	relevance := e.realization.CalculateRelevance(ka, ua, wa)

	// Refresh salience from current dimension levels.
	e.realization.UpdateSalience(e.dimensionValues())

	// Salience: how strongly this event stands out given the salience
	// landscape, modulated by the perspectival way of knowing.
	salience := clamp01(relevance * (0.5 + 0.5*e.knowing.Perspectival))

	// Affordance: opportunities for action offered by the event, grounded
	// in procedural knowing and mastery.
	affordance := clamp01(0.5*e.knowing.Procedural + 0.5*e.wisdom.Mastery)

	// Facets: the dimensions that contributed above their baseline.
	facets := e.activeFacets()

	e.updateCoherence()
	e.totalScored++
	e.lastUpdate = time.Now()

	// Topic/context gently shape the perspectival framing (more context
	// keys = richer framing affordance).
	if n := len(context); n > 0 {
		boost := math.Min(0.05*float64(n), 0.2)
		salience = clamp01(salience + boost*0.5)
	}
	_ = topic

	return &Score{Relevance: relevance, Salience: salience, Affordance: affordance, Facets: facets}
}

// dimensionValues snapshots all nine dimension levels.
func (e *Engine) dimensionValues() map[string]float64 {
	return map[string]float64{
		"propositional": e.knowing.Propositional,
		"procedural":    e.knowing.Procedural,
		"perspectival":  e.knowing.Perspectival,
		"participatory": e.knowing.Participatory,
		"nomological":   e.understanding.Nomological,
		"normative":     e.understanding.Normative,
		"narrative":     e.understanding.Narrative,
		"morality":      e.wisdom.Morality,
		"meaning":       e.wisdom.Meaning,
		"mastery":       e.wisdom.Mastery,
	}
}

// activeFacets lists the triad facets scoring above the 0.5 baseline.
func (e *Engine) activeFacets() []string {
	facets := make([]string, 0, 9)
	for dim, v := range e.dimensionValues() {
		if v >= 0.5 {
			facets = append(facets, dim)
		}
	}
	return facets
}

// updateCoherence mirrors the monorepo: coherence is the mean of the
// three triad means; optimization is coherence scaled by a variance
// penalty around the optimal variance 0.05.
func (e *Engine) updateCoherence() {
	knowingAvg := (e.knowing.Propositional + e.knowing.Procedural +
		e.knowing.Perspectival + e.knowing.Participatory) / 4.0
	understandingAvg := (e.understanding.Nomological + e.understanding.Normative +
		e.understanding.Narrative) / 3.0
	wisdomAvg := (e.wisdom.Morality + e.wisdom.Meaning + e.wisdom.Mastery) / 3.0

	e.OverallCoherence = (knowingAvg + understandingAvg + wisdomAvg) / 3.0

	values := []float64{
		e.knowing.Propositional, e.knowing.Procedural, e.knowing.Perspectival, e.knowing.Participatory,
		e.understanding.Nomological, e.understanding.Normative, e.understanding.Narrative,
		e.wisdom.Morality, e.wisdom.Meaning, e.wisdom.Mastery,
	}
	mean := 0.0
	for _, v := range values {
		mean += v
	}
	mean /= float64(len(values))
	variance := 0.0
	for _, v := range values {
		variance += math.Pow(v-mean, 2)
	}
	variance /= float64(len(values))

	optimalVariance := 0.05
	variancePenalty := 1.0 - math.Abs(variance-optimalVariance)
	e.RelevanceOptimization = e.OverallCoherence * math.Max(0.5, variancePenalty)

	e.understanding.updateMeaningIntegration()
	e.wisdom.updateEudaimonia()
}

// UpdateFromExperience nudges the triads from feedback in [-1, 1], with
// per-way learning rates; wisdom only moves on significant feedback.
func (e *Engine) UpdateFromExperience(feedback float64) {
	e.mu.Lock()
	defer e.mu.Unlock()

	feedback = math.Max(-1.0, math.Min(1.0, feedback))

	k := e.knowing
	k.Propositional = clamp01(k.Propositional + k.PropositionalLearningRate*feedback)
	k.Procedural = clamp01(k.Procedural + k.ProceduralLearningRate*feedback)
	k.Perspectival = clamp01(k.Perspectival + k.PerspectivalLearningRate*feedback)
	if math.Abs(feedback) > 0.5 { // only significant feedback transforms
		k.Participatory = clamp01(k.Participatory + k.ParticipatoryLearningRate*feedback)
	}
	k.GnosticIntegration = 0.2*k.Propositional + 0.2*k.Procedural +
		0.3*k.Perspectival + 0.3*k.Participatory

	u := e.understanding
	u.Nomological = clamp01(u.Nomological + 0.1*feedback)
	u.Normative = clamp01(u.Normative + 0.08*feedback)
	u.Narrative = clamp01(u.Narrative + 0.12*feedback)

	if math.Abs(feedback) > 0.3 { // wisdom develops slowly
		w := e.wisdom
		w.Morality = clamp01(w.Morality + 0.06*feedback)
		w.Meaning = clamp01(w.Meaning + 0.08*feedback)
		w.Mastery = clamp01(w.Mastery + 0.1*feedback)
	}

	// Cross-triad integration (knowing → understanding → wisdom → knowing).
	u.Nomological = 0.95*u.Nomological + 0.05*k.Propositional
	u.Normative = 0.95*u.Normative + 0.05*k.Perspectival
	u.Narrative = 0.95*u.Narrative + 0.05*k.Participatory
	w := e.wisdom
	w.Morality = 0.95*w.Morality + 0.05*u.Normative
	w.Meaning = 0.95*w.Meaning + 0.05*u.Narrative
	w.Mastery = 0.95*w.Mastery + 0.05*u.Nomological
	k.Participatory = 0.95*k.Participatory + 0.05*w.Morality
	k.Perspectival = 0.95*k.Perspectival + 0.05*w.Meaning
	k.Procedural = 0.95*k.Procedural + 0.05*w.Mastery

	e.updateCoherence()
	e.lastUpdate = time.Now()
}

// Status returns a snapshot for health/metrics reporting.
func (e *Engine) Status() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return map[string]any{
		"knowing": map[string]float64{
			"propositional": e.knowing.Propositional, "procedural": e.knowing.Procedural,
			"perspectival": e.knowing.Perspectival, "participatory": e.knowing.Participatory,
			"gnostic_integration": e.knowing.GnosticIntegration,
		},
		"understanding": map[string]float64{
			"nomological": e.understanding.Nomological, "normative": e.understanding.Normative,
			"narrative": e.understanding.Narrative, "meaning_integration": e.understanding.MeaningIntegration,
		},
		"wisdom": map[string]float64{
			"morality": e.wisdom.Morality, "meaning": e.wisdom.Meaning,
			"mastery": e.wisdom.Mastery, "eudaimonia": e.wisdom.Eudaimonia,
		},
		"integration": map[string]float64{
			"coherence": e.OverallCoherence, "optimization": e.RelevanceOptimization,
		},
		"total_scored": e.totalScored,
		"last_update":  e.lastUpdate,
	}
}

// Coherence returns the overall coherence (used for identity coherence).
func (e *Engine) Coherence() float64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.OverallCoherence
}

func clamp01(v float64) float64 {
	return math.Max(0.0, math.Min(1.0, v))
}
