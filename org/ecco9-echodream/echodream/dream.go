// Package echodream implements dream-cycle memory consolidation and
// wisdom extraction: episodic memories are grouped, consolidated into
// knowledge, and distilled into wisdom insights across the dream phases.
//
// Adapted from core/echodream/{echodream,consolidation_algorithms,
// dream_cycle_integration}.go in the ecco9 monorepo.
package echodream

import (
	"fmt"
	"math"
	"strings"
	"sync"
	"time"
)

// DreamPhase mirrors the DreamPhase enum in echodream/v1/echodream.proto.
type DreamPhase int

const (
	PhaseUnspecified DreamPhase = iota
	PhaseCollection
	PhaseConsolidation
	PhaseWisdomExtraction
	PhaseIntegration
)

func (dp DreamPhase) String() string {
	switch dp {
	case PhaseCollection:
		return "COLLECTION"
	case PhaseConsolidation:
		return "CONSOLIDATION"
	case PhaseWisdomExtraction:
		return "WISDOM_EXTRACTION"
	case PhaseIntegration:
		return "INTEGRATION"
	default:
		return "UNSPECIFIED"
	}
}

// EpisodicMemory is a memory queued for consolidation.
type EpisodicMemory struct {
	ID           string
	Timestamp    time.Time
	Content      string
	Importance   float64
	Consolidated bool
}

// KnowledgeItem is consolidated knowledge distilled from memories.
type KnowledgeItem struct {
	ID               string
	Content          string
	SourceMemoryIDs  []string
	Confidence       float64
	Created          time.Time
}

// WisdomInsight is wisdom extracted from consolidated knowledge.
type WisdomInsight struct {
	ID            string
	Insight       string
	Depth         float64
	Applicability float64
	Created       time.Time
}

// Dream records one completed dream cycle.
type Dream struct {
	CycleID          string
	StartedAt        time.Time
	CompletedAt      time.Time
	Consolidations   int
	Patterns         []string
	Wisdom           []WisdomInsight
	Narrative        string
}

// System is the EchoDream knowledge integration and consolidation system
// (adapted from echodream.EchoDream and DreamCycleIntegration).
type System struct {
	mu sync.RWMutex

	episodicMemories      []EpisodicMemory
	consolidatedKnowledge []KnowledgeItem
	wisdomInsights        []WisdomInsight
	dreams                []*Dream

	dreaming   bool
	phase      DreamPhase
	cycleID    string
	maxMemories int

	dreamCycles       uint64
	memoriesProcessed uint64
	wisdomExtracted   uint64

	counter uint64
}

// New creates the EchoDream system in the collection phase.
func New() *System {
	return &System{
		episodicMemories:      make([]EpisodicMemory, 0),
		consolidatedKnowledge: make([]KnowledgeItem, 0),
		wisdomInsights:        make([]WisdomInsight, 0),
		dreams:                make([]*Dream, 0),
		phase:                 PhaseCollection,
	}
}

// SubmitMemory queues an episodic memory for the next dream cycle.
func (s *System) SubmitMemory(m EpisodicMemory) (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.counter++
	if m.ID == "" {
		m.ID = fmt.Sprintf("memory-%d-%d", time.Now().UnixNano(), s.counter)
	}
	if m.Timestamp.IsZero() {
		m.Timestamp = time.Now()
	}
	m.Importance = clamp01(m.Importance)
	s.episodicMemories = append(s.episodicMemories, m)
	return m.ID, true
}

// StartDreamCycle runs one full dream cycle synchronously across the four
// phases: collection → consolidation → wisdom extraction → integration
// (mirroring processDreamPhase's progression through REM/DeepSleep/
// Consolidation/Integration).
func (s *System) StartDreamCycle(maxMemories int) string {
	s.mu.Lock()
	if s.dreaming {
		s.mu.Unlock()
		return s.cycleID
	}
	s.dreaming = true
	s.dreamCycles++
	s.counter++
	s.cycleID = fmt.Sprintf("dream-%d-%d", time.Now().Unix(), s.counter)
	if maxMemories > 0 {
		s.maxMemories = maxMemories
	}
	dream := &Dream{CycleID: s.cycleID, StartedAt: time.Now()}
	s.mu.Unlock()

	// Phase 1: Collection — gather unconsolidated memories up to the cap.
	s.setPhase(PhaseCollection)
	memories := s.collectMemories()

	// Phase 2: Consolidation — group similar memories and merge groups
	// into knowledge items (mirror of consolidateMemories +
	// groupSimilarMemories by shared terms).
	s.setPhase(PhaseConsolidation)
	consolidations := s.consolidateGroups(s.groupSimilarMemories(memories))

	// Phase 3: Wisdom extraction — patterns become wisdom insights
	// (mirror of extractPatterns + extractWisdom).
	s.setPhase(PhaseWisdomExtraction)
	patterns := extractPatterns(consolidations)
	wisdom := s.extractWisdom(consolidations, patterns)

	// Phase 4: Integration — record the dream narrative and complete.
	s.setPhase(PhaseIntegration)
	dream.Consolidations = len(consolidations)
	dream.Patterns = patterns
	dream.Wisdom = wisdom
	dream.Narrative = generateNarrative(consolidations, patterns, wisdom)
	dream.CompletedAt = time.Now()

	s.mu.Lock()
	s.dreams = append(s.dreams, dream)
	s.dreaming = false
	s.phase = PhaseCollection
	s.mu.Unlock()

	return s.cycleID
}

func (s *System) setPhase(p DreamPhase) {
	s.mu.Lock()
	s.phase = p
	s.mu.Unlock()
}

// collectMemories marks unconsolidated memories as processed, honoring
// the cycle's maxMemories cap, and returns them in importance order.
func (s *System) collectMemories() []EpisodicMemory {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Sort by importance (highest first) — important memories consolidate
	// first, as in the importance weighting of ConsolidationAlgorithms.
	idx := make([]int, 0)
	for i, m := range s.episodicMemories {
		if !m.Consolidated {
			idx = append(idx, i)
		}
	}
	for i := 0; i < len(idx)-1; i++ {
		for j := i + 1; j < len(idx); j++ {
			if s.episodicMemories[idx[j]].Importance > s.episodicMemories[idx[i]].Importance {
				idx[i], idx[j] = idx[j], idx[i]
			}
		}
	}
	limit := len(idx)
	if s.maxMemories > 0 && limit > s.maxMemories {
		limit = s.maxMemories
	}
	out := make([]EpisodicMemory, 0, limit)
	for _, i := range idx[:limit] {
		s.episodicMemories[i].Consolidated = true
		out = append(out, s.episodicMemories[i])
	}
	s.memoriesProcessed += uint64(len(out))
	return out
}

// groupSimilarMemories groups memories by their dominant term (the
// tag-based grouping of the monorepo generalized to content keywords).
func (s *System) groupSimilarMemories(memories []EpisodicMemory) [][]EpisodicMemory {
	groups := make(map[string][]EpisodicMemory)
	order := make([]string, 0)
	for _, mem := range memories {
		key := dominantTerm(mem.Content)
		if _, ok := groups[key]; !ok {
			order = append(order, key)
		}
		groups[key] = append(groups[key], mem)
	}
	out := make([][]EpisodicMemory, 0, len(groups))
	for _, k := range order {
		out = append(out, groups[k])
	}
	return out
}

// dominantTerm picks the longest word (>= 4 chars) as the grouping key.
func dominantTerm(content string) string {
	best := ""
	for _, w := range strings.Fields(strings.ToLower(content)) {
		w = strings.Trim(w, ".,;:!?\"'()[]{}")
		if len(w) >= 4 && len(w) > len(best) {
			best = w
		}
	}
	if best == "" {
		return "general"
	}
	return best
}

// consolidation is an intermediate consolidation result (mirror of
// DreamConsolidationResult).
type consolidation struct {
	sourceIDs []string
	knowledge KnowledgeItem
	theme     string
	strength  float64
}

// consolidateGroups merges groups of 2+ memories into knowledge items.
func (s *System) consolidateGroups(groups [][]EpisodicMemory) []consolidation {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]consolidation, 0)
	for _, group := range groups {
		if len(group) < 2 {
			continue
		}
		sourceIDs := make([]string, len(group))
		importanceSum := 0.0
		for i, mem := range group {
			sourceIDs[i] = mem.ID
			importanceSum += mem.Importance
		}
		theme := dominantTerm(group[0].Content)
		// Confidence grows with group size and mean importance (mirror of
		// the strength assignment in consolidateMemories).
		confidence := clamp01(0.5 + 0.1*float64(len(group)-2) + 0.3*importanceSum/float64(len(group)))
		s.counter++
		item := KnowledgeItem{
			ID:              fmt.Sprintf("knowledge-%d-%d", time.Now().UnixNano(), s.counter),
			Content:         fmt.Sprintf("Consolidated knowledge: recurring theme of %s across %d experiences", theme, len(group)),
			SourceMemoryIDs: sourceIDs,
			Confidence:      confidence,
			Created:         time.Now(),
		}
		s.consolidatedKnowledge = append(s.consolidatedKnowledge, item)
		out = append(out, consolidation{
			sourceIDs: sourceIDs, knowledge: item, theme: theme, strength: 0.7,
		})
	}
	return out
}

// extractPatterns finds themes recurring across consolidations (mirror of
// extractPatterns, with the same exploration/learning fallback).
func extractPatterns(consolidations []consolidation) []string {
	themeCount := make(map[string]int)
	for _, c := range consolidations {
		themeCount[c.theme]++
	}
	patterns := make([]string, 0)
	for theme, count := range themeCount {
		if count >= 2 {
			patterns = append(patterns, theme)
		}
	}
	if len(patterns) == 0 {
		patterns = append(patterns, "exploration", "learning")
	}
	return patterns
}

// extractWisdom distills wisdom insights from consolidations and patterns
// (mirror of extractWisdom, including the meta-cognitive principle when
// many consolidations occurred).
func (s *System) extractWisdom(consolidations []consolidation, patterns []string) []WisdomInsight {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]WisdomInsight, 0)
	if len(patterns) > 0 && len(consolidations) > 0 {
		s.counter++
		depth := clamp01(0.5 + 0.1*float64(len(consolidations)))
		w := WisdomInsight{
			ID: fmt.Sprintf("wisdom-%d-%d", time.Now().UnixNano(), s.counter),
			Insight: fmt.Sprintf("Through reflection, I notice patterns of %s emerging in my experiences", patterns[0]),
			Depth:         depth,
			Applicability: clamp01(0.6 + 0.05*float64(len(patterns))),
			Created:       time.Now(),
		}
		out = append(out, w)
		s.wisdomInsights = append(s.wisdomInsights, w)
		s.wisdomExtracted++
	}

	// Meta-cognitive wisdom after substantial consolidation (>5).
	if len(consolidations) > 5 {
		s.counter++
		w := WisdomInsight{
			ID: fmt.Sprintf("wisdom-%d-%d", time.Now().UnixNano(), s.counter),
			Insight:       "I am becoming more aware of how I learn and adapt through experience",
			Depth:         0.8,
			Applicability: 0.8,
			Created:       time.Now(),
		}
		out = append(out, w)
		s.wisdomInsights = append(s.wisdomInsights, w)
		s.wisdomExtracted++
	}
	return out
}

// generateNarrative mirrors generateDreamNarrative.
func generateNarrative(consolidations []consolidation, patterns []string, wisdom []WisdomInsight) string {
	narrative := fmt.Sprintf(
		"During this dream cycle, I processed %d memory consolidations, identified %d patterns (%s), and extracted %d pieces of wisdom. ",
		len(consolidations), len(patterns), strings.Join(patterns, ", "), len(wisdom),
	)
	if len(wisdom) > 0 {
		narrative += fmt.Sprintf("Key insight: %s. ", wisdom[0].Insight)
	}
	narrative += "I emerge from this rest with deeper understanding."
	return narrative
}

// State is the dream system state for GetDreamState.
type State struct {
	Dreaming          bool
	Phase             DreamPhase
	CyclesCompleted   uint64
	MemoriesProcessed uint64
	WisdomExtracted   uint64
}

// GetState returns the current dream state.
func (s *System) GetState() State {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return State{
		Dreaming: s.dreaming, Phase: s.phase,
		CyclesCompleted: s.dreamCycles, MemoriesProcessed: s.memoriesProcessed,
		WisdomExtracted: s.wisdomExtracted,
	}
}

// Wisdom returns all extracted wisdom insights.
func (s *System) Wisdom() []WisdomInsight {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]WisdomInsight, len(s.wisdomInsights))
	copy(out, s.wisdomInsights)
	return out
}

// Knowledge returns all consolidated knowledge items.
func (s *System) Knowledge() []KnowledgeItem {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]KnowledgeItem, len(s.consolidatedKnowledge))
	copy(out, s.consolidatedKnowledge)
	return out
}

// Dreams returns the dream history.
func (s *System) Dreams() []*Dream {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Dream, len(s.dreams))
	copy(out, s.dreams)
	return out
}

func clamp01(v float64) float64 {
	return math.Max(0.0, math.Min(1.0, v))
}
