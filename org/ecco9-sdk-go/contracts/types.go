// Package contracts defines the canonical Go types mirroring the protobuf
// definitions in ecco9-proto. These types are the hand-written SDK surface
// used until `buf generate` produces the gRPC stubs; keeping them here lets
// all service repositories compile against stable contracts offline.
package contracts

import "time"

// ServingStatus mirrors ecco9.common.v1.ServingStatus.
type ServingStatus int32

const (
	ServingStatusUnspecified ServingStatus = iota
	ServingStatusServing
	ServingStatusNotServing
	ServingStatusDegraded
)

// CognitiveMetrics mirrors ecco9.common.v1.CognitiveMetrics.
type CognitiveMetrics struct {
	ServiceName                string
	InstanceID                 string
	Timestamp                  time.Time
	ThoughtQueueDepth          float64
	MemoryConsolidationBacklog float64
	DreamCycleFreshness        time.Duration
	IdentityCoherenceDelta     float64
	ReservoirEchoProperty      float64
	ActiveGoroutines           int
}

// PersonaType mirrors ecco9.reservoir.v1.PersonaType.
type PersonaType int32

const (
	PersonaUnspecified PersonaType = iota
	PersonaContemplativeScholar
	PersonaDynamicExplorer
	PersonaCautiousAnalyst
	PersonaCreativeVisionary
)

// EmotionType mirrors ecco9.emotion.v1.EmotionType.
type EmotionType int32

const (
	EmotionUnspecified EmotionType = iota
	EmotionInterest
	EmotionJoy
	EmotionSurprise
	EmotionSadness
	EmotionAnger
	EmotionDisgust
	EmotionContempt
	EmotionFear
	EmotionShame
	EmotionGuilt
)

// EmotionalState mirrors ecco9.emotion.v1.EmotionalState.
type EmotionalState struct {
	Type              EmotionType
	Intensity         float64
	AttentionScope    float64
	ProcessingDepth   float64
	ApproachAvoidance float64
	MemoryStrength    float64
	ExplorationBias   float64
	OnsetTime         time.Time
}

// CognitivePhase mirrors ecco9.echobeats.v1.CognitivePhase.
type CognitivePhase int32

const (
	CognitivePhaseUnspecified CognitivePhase = iota
	CognitivePhaseExpressive
	CognitivePhaseReflective
	CognitivePhaseIntegrative
)

// StepExecution mirrors ecco9.echobeats.v1.StepExecution.
type StepExecution struct {
	Step           int32
	Phase          CognitivePhase
	Thought        string
	ExecutedAt     time.Time
	RelevanceScore float64
}

// ThreadPoolDirective mirrors ecco9.orchestrator.v1.ThreadPoolDirective.
type ThreadPoolDirective struct {
	ServiceName      string
	TargetWorkers    int32
	CycleFrequencyHz float64
	Reason           string
}
