// Package metacog implements metacognitive monitoring, decision quality
// assessment, strategy selection, self-directed learning gap tracking, and
// recursive self-improvement.
//
// Adapted from core/deeptreeecho/{metacognitive_monitor,
// self_directed_learning}.go and core/improvement/recursive.go in the
// ecco9 monorepo.
package metacog

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// CognitiveProcess is an ongoing cognitive process under monitoring.
type CognitiveProcess struct {
	ID           string
	Name         string
	Type         string
	Service      string
	StartTime    time.Time
	Progress     float64
	QualityScore float64
	Completed    bool
	Success      bool
}

// Decision is a logged decision awaiting quality assessment.
type Decision struct {
	ID           string
	Timestamp    time.Time
	Context      string
	Chosen       string
	Rationale    string
	Confidence   float64
	UrgencyLevel float64
}

// QualityAssessment evaluates decision quality along four axes.
type QualityAssessment struct {
	DecisionID             string
	Appropriateness        float64
	Timeliness             float64
	Effectiveness          float64
	Efficiency             float64
	OverallQuality         float64
	ImprovementSuggestions []string
}

// CognitiveStrategy is a high-level thinking strategy.
type CognitiveStrategy struct {
	ID           string
	Name         string
	Description  string
	ApplicableTo []string
	Reliability  float64
	Speed        float64
	Accuracy     float64
	SuccessRate  float64
}

// KnowledgeGap is an identified self-directed learning gap.
type KnowledgeGap struct {
	ID           string
	Domain       string
	Description  string
	Severity     float64
	IdentifiedAt time.Time
	Addressed    bool
}

// Monitor is the metacognitive monitoring system (adapted from
// deeptreeecho.MetaCognitiveMonitor).
type Monitor struct {
	mu sync.RWMutex

	activeProcesses map[string]*CognitiveProcess
	recentDecisions []*Decision
	decisionQuality map[string]*QualityAssessment

	strategies map[string]*CognitiveStrategy

	learningGaps map[string]*KnowledgeGap

	confidenceLevel float64
	awarenessLevel  float64

	totalProcesses uint64
	totalDecisions uint64
	strategyShifts uint64

	counter uint64
}

// NewMonitor creates a monitor with the default strategy repertoire.
func NewMonitor() *Monitor {
	m := &Monitor{
		activeProcesses: make(map[string]*CognitiveProcess),
		recentDecisions: make([]*Decision, 0),
		decisionQuality: make(map[string]*QualityAssessment),
		strategies:      make(map[string]*CognitiveStrategy),
		learningGaps:    make(map[string]*KnowledgeGap),
		confidenceLevel: 0.7,
		awarenessLevel:  0.8,
	}
	// Default strategies from the monorepo's initializeStrategies.
	m.strategies["deliberate"] = &CognitiveStrategy{
		ID: "deliberate", Name: "Deliberate Reasoning",
		Description:  "Slow, careful, step-by-step analysis",
		ApplicableTo: []string{"reasoning", "problem_solving", "decision_making"},
		Reliability:  0.9, Speed: 0.3, Accuracy: 0.95, SuccessRate: 0.7,
	}
	m.strategies["intuitive"] = &CognitiveStrategy{
		ID: "intuitive", Name: "Intuitive Processing",
		Description:  "Fast, pattern-based, heuristic thinking",
		ApplicableTo: []string{"thinking", "pattern_recognition", "creative_synthesis"},
		Reliability:  0.7, Speed: 0.9, Accuracy: 0.75, SuccessRate: 0.7,
	}
	m.strategies["analytical"] = &CognitiveStrategy{
		ID: "analytical", Name: "Analytical Decomposition",
		Description:  "Break down complex problems into components",
		ApplicableTo: []string{"problem_solving", "reasoning"},
		Reliability:  0.85, Speed: 0.5, Accuracy: 0.9, SuccessRate: 0.7,
	}
	return m
}

// Track registers a cognitive process for monitoring. If the process ID
// already exists, its progress/quality are updated (mirroring
// UpdateProcess); otherwise a new record is created.
func (m *Monitor) Track(p CognitiveProcess) (string, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if p.ID != "" {
		if existing, ok := m.activeProcesses[p.ID]; ok {
			existing.Progress = clamp01(p.Progress)
			existing.QualityScore = clamp01(p.QualityScore)
			if p.Completed {
				existing.Completed = true
				existing.Success = p.Success
				// Completed processes leave the active set (finalizeProcess).
				delete(m.activeProcesses, p.ID)
				m.updateSelfModel(existing)
			}
			return p.ID, true
		}
	}

	m.counter++
	if p.ID == "" {
		p.ID = fmt.Sprintf("process-%d-%d", time.Now().Unix(), m.counter)
	}
	if p.StartTime.IsZero() {
		p.StartTime = time.Now()
	}
	proc := p
	m.activeProcesses[proc.ID] = &proc
	m.totalProcesses++
	return proc.ID, true
}

// updateSelfModel nudges confidence/awareness from process outcomes.
func (m *Monitor) updateSelfModel(p *CognitiveProcess) {
	if p.Success {
		m.confidenceLevel = math.Min(1.0, m.confidenceLevel+0.02)
	} else {
		m.confidenceLevel = math.Max(0.0, m.confidenceLevel-0.05)
		// Failures reveal potential learning gaps in the process's domain.
		if p.QualityScore < 0.5 {
			m.counter++
			gapID := fmt.Sprintf("gap-%d-%d", time.Now().Unix(), m.counter)
			m.learningGaps[gapID] = &KnowledgeGap{
				ID: gapID, Domain: p.Type,
				Description:  fmt.Sprintf("Low-quality outcome in %s process on %s", p.Type, p.Service),
				Severity:     0.5 + (0.5-p.QualityScore)*0.5,
				IdentifiedAt: time.Now(),
			}
		}
	}
}

// RecordDecision logs a decision for later quality assessment.
func (m *Monitor) RecordDecision(context, chosen, rationale string, confidence float64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counter++
	d := &Decision{
		ID:        fmt.Sprintf("decision-%d-%d", time.Now().Unix(), m.counter),
		Timestamp: time.Now(), Context: context, Chosen: chosen,
		Rationale: rationale, Confidence: clamp01(confidence), UrgencyLevel: 0.5,
	}
	m.recentDecisions = append(m.recentDecisions, d)
	if len(m.recentDecisions) > 100 {
		m.recentDecisions = m.recentDecisions[1:]
	}
	m.totalDecisions++
	return d.ID
}

// AssessDecision scores a recorded decision against its outcome, as in
// the monorepo's AssessDecisionQuality.
func (m *Monitor) AssessDecision(decisionID string, success bool, expectedBenefit, actualBenefit float64, sideEffects int) *QualityAssessment {
	m.mu.Lock()
	defer m.mu.Unlock()

	var decision *Decision
	for _, d := range m.recentDecisions {
		if d.ID == decisionID {
			decision = d
			break
		}
	}
	if decision == nil {
		return nil
	}

	assessment := &QualityAssessment{
		DecisionID:      decisionID,
		Appropriateness: decision.Confidence * 0.8,
		Timeliness:      1.0 - decision.UrgencyLevel*0.3,
		Effectiveness:   actualBenefit / math.Max(expectedBenefit, 0.01),
		Efficiency:      1.0,
	}
	if sideEffects > 0 {
		assessment.Efficiency = 1.0 / (1.0 + float64(sideEffects)*0.2)
	}
	assessment.Effectiveness = math.Min(assessment.Effectiveness, 2.0) / 2.0
	assessment.OverallQuality = (assessment.Appropriateness + assessment.Timeliness +
		assessment.Effectiveness + assessment.Efficiency) / 4.0

	if assessment.OverallQuality < 0.7 {
		if assessment.Appropriateness < 0.7 {
			assessment.ImprovementSuggestions = append(assessment.ImprovementSuggestions,
				"Consider gathering more context before deciding")
		}
		if assessment.Timeliness < 0.7 {
			assessment.ImprovementSuggestions = append(assessment.ImprovementSuggestions,
				"Balance urgency with decision quality")
		}
		if assessment.Effectiveness < 0.7 {
			assessment.ImprovementSuggestions = append(assessment.ImprovementSuggestions,
				"Improve outcome prediction accuracy")
		}
	}
	if !success {
		assessment.OverallQuality *= 0.8
	}

	m.decisionQuality[decisionID] = assessment
	return assessment
}

// SelectStrategy chooses the best applicable strategy for a problem
// context, using the monorepo's calculateStrategyScore weighting
// (reliability .3, success .3, speed/accuracy .4 when constrained).
func (m *Monitor) SelectStrategy(problemContext string, candidates []string) (string, float64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	allowed := make(map[string]bool, len(candidates))
	for _, c := range candidates {
		allowed[c] = true
	}

	// Speed vs. accuracy pressure inferred from the problem context.
	speedRequired := containsAny(problemContext, "urgent", "fast", "realtime", "deadline")
	accuracyRequired := containsAny(problemContext, "critical", "precise", "exact", "safety")

	var best *CognitiveStrategy
	bestScore := 0.0
	for _, s := range m.strategies {
		if len(allowed) > 0 && !allowed[s.ID] && !allowed[s.Name] {
			continue
		}
		// Applicability: process-type keywords from the context.
		applicable := len(s.ApplicableTo) == 0
		for _, pt := range s.ApplicableTo {
			if containsAny(problemContext, pt) {
				applicable = true
				break
			}
		}
		if !applicable && len(allowed) == 0 {
			continue
		}

		score := s.Reliability*0.3 + s.SuccessRate*0.3
		switch {
		case speedRequired:
			score += s.Speed * 0.4
		case accuracyRequired:
			score += s.Accuracy * 0.4
		default:
			score += (s.Speed + s.Accuracy) * 0.2
		}
		if score > bestScore {
			bestScore = score
			best = s
		}
	}

	if best == nil {
		// Fall back to deliberate reasoning.
		best = m.strategies["deliberate"]
		bestScore = best.Reliability * 0.6
	}

	best.TimesUsedInc()
	m.strategyShifts++
	return best.ID, clamp01(bestScore)
}

// AddGap registers an externally identified learning gap.
func (m *Monitor) AddGap(domain, description string, severity float64) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.counter++
	id := fmt.Sprintf("gap-%d-%d", time.Now().Unix(), m.counter)
	m.learningGaps[id] = &KnowledgeGap{
		ID: id, Domain: domain, Description: description,
		Severity: clamp01(severity), IdentifiedAt: time.Now(),
	}
	return id
}

// LearningGaps returns all identified gaps, most severe first.
func (m *Monitor) LearningGaps() []*KnowledgeGap {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]*KnowledgeGap, 0, len(m.learningGaps))
	for _, g := range m.learningGaps {
		out = append(out, g)
	}
	for i := 0; i < len(out)-1; i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j].Severity > out[i].Severity {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

// Assessment is the global metacognitive assessment for the orchestrator.
type Assessment struct {
	GlobalCognitiveLoad float64
	IdentityCoherence   float64
	Recommendations     []string
}

// AssessSystem computes global load, identity coherence, and
// recommendations — the metacog analog of improvement.RecursiveSelfImprover
// analysis feeding the orchestrator's adaptation loop.
func (m *Monitor) AssessSystem() *Assessment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Global cognitive load: active process pressure relative to capacity.
	const capacity = 16.0
	load := math.Min(1.0, float64(len(m.activeProcesses))/capacity)
	// Low-quality recent decisions add load (rework pressure).
	lowQuality := 0
	assessed := 0
	qualitySum := 0.0
	for _, a := range m.decisionQuality {
		assessed++
		qualitySum += a.OverallQuality
		if a.OverallQuality < 0.7 {
			lowQuality++
		}
	}
	if assessed > 0 {
		load = math.Min(1.0, load+0.2*float64(lowQuality)/float64(assessed))
	}

	// Identity coherence: confidence and awareness blended with mean
	// decision quality.
	coherence := (m.confidenceLevel + m.awarenessLevel) / 2.0
	if assessed > 0 {
		coherence = 0.5*coherence + 0.5*(qualitySum/float64(assessed))
	}

	recommendations := make([]string, 0)
	if load > 0.8 {
		recommendations = append(recommendations, "Scale out cognitive replicas or shed low-priority processes")
	}
	if m.confidenceLevel < 0.5 {
		recommendations = append(recommendations, "Trigger self-directed learning cycle to rebuild confidence")
	}
	if len(m.learningGaps) > 5 {
		recommendations = append(recommendations, "Prioritize top learning gaps into goal orchestration")
	}
	if assessed > 0 && qualitySum/float64(assessed) < 0.7 {
		recommendations = append(recommendations, "Run recursive self-improvement cycle on decision pipeline")
	}
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "System operating within metacognitive tolerances")
	}

	return &Assessment{
		GlobalCognitiveLoad: load,
		IdentityCoherence:   clamp01(coherence),
		Recommendations:     recommendations,
	}
}

// Status returns self-awareness metrics (mirror of GetSelfAwareness).
func (m *Monitor) Status() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return map[string]any{
		"confidence_level": m.confidenceLevel,
		"awareness_level":  m.awarenessLevel,
		"active_processes": len(m.activeProcesses),
		"total_processes":  m.totalProcesses,
		"total_decisions":  m.totalDecisions,
		"strategy_shifts":  m.strategyShifts,
		"learning_gaps":    len(m.learningGaps),
		"strategies":       len(m.strategies),
	}
}

// TimesUsedInc bumps a strategy's use count (success rate placeholder as
// in the monorepo's StrategyMetrics initialization).
func (s *CognitiveStrategy) TimesUsedInc() { s.SuccessRate = math.Min(1.0, s.SuccessRate+0.001) }

func containsAny(haystack string, needles ...string) bool {
	lower := strings.ToLower(haystack)
	for _, n := range needles {
		if strings.Contains(lower, strings.ToLower(n)) {
			return true
		}
	}
	return false
}

func clamp01(v float64) float64 {
	return math.Max(0.0, math.Min(1.0, v))
}
