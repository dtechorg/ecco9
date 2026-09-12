package identity

import (
	"math"
	"sync"
	"time"
)

// AgentModel is our theory-of-mind model of another agent's mental state,
// adapted from core/deeptreeecho/theory_of_mind.go.
type AgentModel struct {
	AgentID   string             `json:"agent_id"`
	AgentType string             `json:"agent_type"` // "human", "ai", "system"
	Goals     []AgentGoal        `json:"goals"`
	Beliefs   map[string]float64 `json:"beliefs"` // belief -> confidence [0,1]

	// Behavioral calibration
	Predictability   float64 `json:"predictability"`
	TrustScore       float64 `json:"trust_score"`
	ReliabilityScore float64 `json:"reliability_score"`

	// Cognitive style
	Style CognitiveStyle `json:"cognitive_style"`

	InteractionCount int       `json:"interaction_count"`
	LastInteraction  time.Time `json:"last_interaction"`

	// actionCounts feeds the entropy-based predictability estimate.
	actionCounts map[string]int
	totalActions int
}

// AgentGoal is an inferred or declared objective.
type AgentGoal struct {
	Description string  `json:"description"`
	Priority    float64 `json:"priority"`
	Progress    float64 `json:"progress"`
}

// CognitiveStyle describes how an agent thinks.
type CognitiveStyle struct {
	Analytical    float64 `json:"analytical"`
	Intuitive     float64 `json:"intuitive"`
	Cautious      float64 `json:"cautious"`
	Exploratory   float64 `json:"exploratory"`
	Collaborative float64 `json:"collaborative"`
}

// TheoryOfMind manages agent models, trust calibration, and recursive
// social reasoning ("I think they think I think...", max depth 3).
type TheoryOfMind struct {
	mu           sync.RWMutex
	models       map[string]*AgentModel
	maxRecursion int
	learningRate float64
}

// NewTheoryOfMind creates an empty ToM module.
func NewTheoryOfMind() *TheoryOfMind {
	return &TheoryOfMind{
		models:       make(map[string]*AgentModel),
		maxRecursion: 3,
		learningRate: 0.1,
	}
}

// ensureModel gets or lazily creates a model (call with lock held).
func (tom *TheoryOfMind) ensureModel(agentID string) *AgentModel {
	if m, ok := tom.models[agentID]; ok {
		return m
	}
	m := &AgentModel{
		AgentID:          agentID,
		AgentType:        "unknown",
		Beliefs:          make(map[string]float64),
		Goals:            []AgentGoal{},
		Predictability:   0.5,
		TrustScore:       0.7, // start with moderate trust
		ReliabilityScore: 0.5,
		Style: CognitiveStyle{
			Analytical: 0.5, Intuitive: 0.5, Cautious: 0.5,
			Exploratory: 0.5, Collaborative: 0.7, // assume collaborative
		},
		actionCounts: make(map[string]int),
	}
	tom.models[agentID] = m
	return m
}

// GetAgentModel returns the model for an agent (creating a neutral default
// on first sight, as in the original module).
func (tom *TheoryOfMind) GetAgentModel(agentID string) *AgentModel {
	tom.mu.Lock()
	defer tom.mu.Unlock()
	m := tom.ensureModel(agentID)
	// Return a copy without internal counters.
	cp := *m
	cp.actionCounts = nil
	return &cp
}

// UpdateBelief records what we think an agent believes.
func (tom *TheoryOfMind) UpdateBelief(agentID, belief string, confidence float64) {
	tom.mu.Lock()
	defer tom.mu.Unlock()
	tom.ensureModel(agentID).Beliefs[belief] = clamp01(confidence)
}

// RecordAction observes an action, updating the predictability estimate
// via Shannon entropy over the action distribution (lower diversity =
// higher predictability), as in the original updatePredictability.
func (tom *TheoryOfMind) RecordAction(agentID, action string) {
	tom.mu.Lock()
	defer tom.mu.Unlock()
	m := tom.ensureModel(agentID)
	m.actionCounts[action]++
	m.totalActions++
	if m.totalActions < 5 {
		return
	}
	var entropy float64
	total := float64(m.totalActions)
	for _, c := range m.actionCounts {
		p := float64(c) / total
		if p > 0 {
			entropy -= p * math.Log2(p)
		}
	}
	const maxEntropy = 2.0 // approximate max for typical action sets
	m.Predictability = clamp01(1.0 - entropy/maxEntropy)
}

// UpdateTrust folds an interaction outcome into trust and reliability with
// a fixed learning rate (EMA), clamped to [0,1].
func (tom *TheoryOfMind) UpdateTrust(agentID string, outcome float64) {
	tom.mu.Lock()
	defer tom.mu.Unlock()
	m := tom.ensureModel(agentID)
	outcome = clamp01(outcome)
	m.TrustScore = clamp01(m.TrustScore*(1.0-tom.learningRate) + outcome*tom.learningRate)
	m.ReliabilityScore = clamp01(m.ReliabilityScore*(1.0-tom.learningRate) + outcome*tom.learningRate)
	m.InteractionCount++
	m.LastInteraction = time.Now()
}

// InferGoal infers an agent's goal from observed actions and records it.
func (tom *TheoryOfMind) InferGoal(agentID string, observedActions []string) AgentGoal {
	tom.mu.Lock()
	defer tom.mu.Unlock()
	m := tom.ensureModel(agentID)
	goal := AgentGoal{
		Description: "Inferred from actions",
		Priority:    0.6,
		Progress:    0.3,
	}
	// Prefer a recurring action theme when one dominates.
	best, bestN := "", 0
	for a, n := range m.actionCounts {
		if n > bestN {
			best, bestN = a, n
		}
	}
	if best != "" && bestN >= 3 {
		goal.Description = "Sustained pattern: " + best
		goal.Priority = clamp01(0.4 + float64(bestN)/float64(m.totalActions))
	}
	_ = observedActions
	m.Goals = append(m.Goals, goal)
	if len(m.Goals) > 20 {
		m.Goals = m.Goals[1:]
	}
	return goal
}

// PredictAction predicts the agent's next action from their cognitive
// style and most common past action.
func (tom *TheoryOfMind) PredictAction(agentID string) string {
	tom.mu.RLock()
	defer tom.mu.RUnlock()
	m, ok := tom.models[agentID]
	if !ok {
		return "unknown_action"
	}
	if m.Style.Cautious > 0.7 {
		return "cautious_action"
	}
	if m.Style.Exploratory > 0.7 {
		return "exploratory_action"
	}
	best, bestN := "", 0
	for a, n := range m.actionCounts {
		if n > bestN {
			best, bestN = a, n
		}
	}
	if best != "" {
		return best
	}
	return "unknown_action"
}

// DetectDeception estimates the probability that a statement is deceptive
// from belief consistency, behavioral consistency, and motivation.
func (tom *TheoryOfMind) DetectDeception(agentID string) float64 {
	tom.mu.RLock()
	defer tom.mu.RUnlock()
	m, ok := tom.models[agentID]
	if !ok {
		return 0.5
	}
	behaviorScore := m.ReliabilityScore
	motivation := 1.0 - m.Style.Collaborative
	return clamp01((1.0-0.7)*0.4 + (1.0-behaviorScore)*0.3 + motivation*0.3)
}

// ModelCount returns the number of tracked agent models.
func (tom *TheoryOfMind) ModelCount() int {
	tom.mu.RLock()
	defer tom.mu.RUnlock()
	return len(tom.models)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
