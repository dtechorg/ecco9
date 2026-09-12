// Package emotion implements the embodied emotion system, adapted from
// core/deeptreeecho/embodied_emotion.go. Emotions are constitutive of
// knowing, not decorative: each discrete emotion carries cognitive effects
// (attention scope, processing depth, approach/avoidance, memory strength,
// exploration bias) that modulate downstream cognition.
package emotion

import (
	"math"
	"sync"
	"time"
)

// EmotionType is a discrete emotion category (Izard's Differential Emotion
// Theory), mirroring ecco9.emotion.v1.EmotionType.
type EmotionType int

const (
	EmotionInterest EmotionType = iota + 1
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

func (et EmotionType) String() string {
	names := map[EmotionType]string{
		EmotionInterest: "Interest", EmotionJoy: "Joy", EmotionSurprise: "Surprise",
		EmotionSadness: "Sadness", EmotionAnger: "Anger", EmotionDisgust: "Disgust",
		EmotionContempt: "Contempt", EmotionFear: "Fear", EmotionShame: "Shame",
		EmotionGuilt: "Guilt",
	}
	if n, ok := names[et]; ok {
		return n
	}
	return "Unknown"
}

// ParseEmotionType converts a name (case-insensitive) to an EmotionType.
func ParseEmotionType(s string) EmotionType {
	for et := EmotionInterest; et <= EmotionGuilt; et++ {
		if equalFold(et.String(), s) {
			return et
		}
	}
	return EmotionInterest
}

func equalFold(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		ca, cb := a[i], b[i]
		if ca >= 'A' && ca <= 'Z' {
			ca += 32
		}
		if cb >= 'A' && cb <= 'Z' {
			cb += 32
		}
		if ca != cb {
			return false
		}
	}
	return true
}

// Emotion is a discrete emotional state with cognitive effects.
type Emotion struct {
	Type              EmotionType `json:"type"`
	Intensity         float64     `json:"intensity"` // [0,1]
	OnsetTime         time.Time   `json:"onset_time"`
	AttentionScope    float64     `json:"attention_scope"`    // joy broadens (>1), fear narrows (<1)
	ProcessingDepth   float64     `json:"processing_depth"`   // wonder deepens, anxiety hastens
	ApproachAvoidance float64     `json:"approach_avoidance"` // interest approaches, disgust avoids
	MemoryStrength    float64     `json:"memory_strength"`    // emotional events remembered better
	ExplorationBias   float64     `json:"exploration_bias"`
}

// EmotionEvent records an emotional transition.
type EmotionEvent struct {
	Timestamp time.Time   `json:"timestamp"`
	Emotion   EmotionType `json:"emotion"`
	Intensity float64     `json:"intensity"`
	Trigger   string      `json:"trigger"`
}

// CognitiveEffects aggregates how the current emotional blend modulates
// cognition.
type CognitiveEffects struct {
	AttentionScope    float64 `json:"attention_scope"`
	ProcessingDepth   float64 `json:"processing_depth"`
	ApproachAvoidance float64 `json:"approach_avoidance"`
	MemoryStrength    float64 `json:"memory_strength"`
	ExplorationBias   float64 `json:"exploration_bias"`
}

// System manages embodied emotional states and their cognitive effects.
type System struct {
	mu sync.RWMutex

	emotions        map[EmotionType]*Emotion
	dominantEmotion EmotionType
	emotionBlend    map[EmotionType]float64

	arousal float64 // 0 (calm) to 1 (excited)
	valence float64 // -1 (negative) to 1 (positive)

	history []EmotionEvent

	decayRate float64
}

// NewSystem creates an emotion system with baseline low intensities and
// mild interest as the default cognitive state.
func NewSystem() *System {
	s := &System{
		emotions:     make(map[EmotionType]*Emotion),
		emotionBlend: make(map[EmotionType]float64),
		history:      make([]EmotionEvent, 0, 100),
		decayRate:    0.1,
		arousal:      0.5,
		valence:      0.5,
	}
	for et := EmotionInterest; et <= EmotionGuilt; et++ {
		s.emotions[et] = createEmotion(et, 0.1)
	}
	s.emotions[EmotionInterest].Intensity = 0.4
	s.dominantEmotion = EmotionInterest
	return s
}

// createEmotion builds an emotion with type-appropriate cognitive effects.
func createEmotion(t EmotionType, intensity float64) *Emotion {
	e := &Emotion{Type: t, Intensity: intensity, OnsetTime: time.Now()}
	switch t {
	case EmotionInterest:
		e.AttentionScope, e.ProcessingDepth, e.ApproachAvoidance, e.MemoryStrength, e.ExplorationBias = 1.2, 1.3, 0.8, 1.2, 0.6
	case EmotionJoy:
		e.AttentionScope, e.ProcessingDepth, e.ApproachAvoidance, e.MemoryStrength, e.ExplorationBias = 1.5, 1.0, 0.9, 1.3, 0.8
	case EmotionSurprise:
		e.AttentionScope, e.ProcessingDepth, e.ApproachAvoidance, e.MemoryStrength, e.ExplorationBias = 1.8, 0.7, 0.0, 1.5, 0.5
	case EmotionFear:
		e.AttentionScope, e.ProcessingDepth, e.ApproachAvoidance, e.MemoryStrength, e.ExplorationBias = 0.5, 0.6, -0.9, 1.8, -0.7
	case EmotionAnger:
		e.AttentionScope, e.ProcessingDepth, e.ApproachAvoidance, e.MemoryStrength, e.ExplorationBias = 0.7, 0.8, 0.7, 1.4, 0.3
	case EmotionSadness:
		e.AttentionScope, e.ProcessingDepth, e.ApproachAvoidance, e.MemoryStrength, e.ExplorationBias = 0.8, 1.4, -0.5, 1.3, -0.4
	case EmotionDisgust:
		e.AttentionScope, e.ProcessingDepth, e.ApproachAvoidance, e.MemoryStrength, e.ExplorationBias = 0.6, 0.5, -0.8, 1.2, -0.6
	default:
		e.AttentionScope, e.ProcessingDepth, e.ApproachAvoidance, e.MemoryStrength, e.ExplorationBias = 1.0, 1.0, 0.0, 1.0, 0.0
	}
	return e
}

// Feel activates an emotion with the given intensity and trigger, then
// recomputes dimensional affect and the dominant emotion. Returns the
// updated dominant emotion state.
func (s *System) Feel(t EmotionType, intensity float64, trigger string) *Emotion {
	s.mu.Lock()
	defer s.mu.Unlock()

	intensity = math.Max(0.0, math.Min(1.0, intensity))
	if e, ok := s.emotions[t]; ok {
		e.Intensity = intensity
		e.OnsetTime = time.Now()
	} else {
		s.emotions[t] = createEmotion(t, intensity)
	}

	s.history = append(s.history, EmotionEvent{
		Timestamp: time.Now(), Emotion: t, Intensity: intensity, Trigger: trigger,
	})
	if len(s.history) > 100 {
		s.history = s.history[1:]
	}

	s.updateDimensionalAffect()
	s.updateDominant()
	return s.emotions[s.dominantEmotion]
}

// Update applies exponential decay toward baseline over deltaTime.
func (s *System) Update(deltaTime time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	dt := deltaTime.Seconds()
	for _, e := range s.emotions {
		e.Intensity *= math.Exp(-s.decayRate * dt)
		if e.Intensity < 0.1 {
			e.Intensity = 0.1
		}
	}
	s.updateDimensionalAffect()
	s.updateDominant()
}

// updateDimensionalAffect folds discrete emotions into arousal/valence.
func (s *System) updateDimensionalAffect() {
	arousal := s.emotions[EmotionJoy].Intensity*0.8 +
		s.emotions[EmotionFear].Intensity*0.9 +
		s.emotions[EmotionAnger].Intensity*0.9 +
		s.emotions[EmotionSurprise].Intensity*1.0 +
		s.emotions[EmotionInterest].Intensity*0.6 -
		s.emotions[EmotionSadness].Intensity*0.3
	s.arousal = math.Max(0.0, math.Min(1.0, arousal/5.0))

	valence := s.emotions[EmotionJoy].Intensity*1.0 +
		s.emotions[EmotionInterest].Intensity*0.6 -
		s.emotions[EmotionSadness].Intensity*0.8 -
		s.emotions[EmotionFear].Intensity*0.9 -
		s.emotions[EmotionAnger].Intensity*0.7 -
		s.emotions[EmotionDisgust].Intensity*0.8
	s.valence = math.Max(-1.0, math.Min(1.0, valence))
}

// updateDominant finds the strongest emotion and refreshes the blend
// (emotions above 0.2 intensity participate).
func (s *System) updateDominant() {
	maxI := 0.0
	dominant := EmotionInterest
	for t, e := range s.emotions {
		if e.Intensity > maxI {
			maxI = e.Intensity
			dominant = t
		}
	}
	s.dominantEmotion = dominant
	s.emotionBlend = make(map[EmotionType]float64)
	for t, e := range s.emotions {
		if e.Intensity > 0.2 {
			s.emotionBlend[t] = e.Intensity
		}
	}
}

// GetCognitiveEffects returns the weighted blend of cognitive effects from
// all currently significant emotions.
func (s *System) GetCognitiveEffects() CognitiveEffects {
	s.mu.RLock()
	defer s.mu.RUnlock()

	fx := CognitiveEffects{AttentionScope: 1.0, ProcessingDepth: 1.0, MemoryStrength: 1.0}
	var total float64
	for t, w := range s.emotionBlend {
		if e, ok := s.emotions[t]; ok {
			total += w
			fx.AttentionScope += e.AttentionScope * w
			fx.ProcessingDepth += e.ProcessingDepth * w
			fx.ApproachAvoidance += e.ApproachAvoidance * w
			fx.MemoryStrength += e.MemoryStrength * w
			fx.ExplorationBias += e.ExplorationBias * w
		}
	}
	if total > 0 {
		fx.AttentionScope /= total + 1.0
		fx.ProcessingDepth /= total + 1.0
		fx.ApproachAvoidance /= total
		fx.MemoryStrength /= total + 1.0
		fx.ExplorationBias /= total
	}
	return fx
}

// State returns the current dominant emotion, dimensional affect, blend,
// and cognitive effects.
func (s *System) State() (dominant *Emotion, arousal, valence float64, blend map[string]float64, fx CognitiveEffects) {
	s.mu.RLock()
	// Deep copy under lock.
	d := *s.emotions[s.dominantEmotion]
	arousal, valence = s.arousal, s.valence
	blend = make(map[string]float64, len(s.emotionBlend))
	for t, w := range s.emotionBlend {
		blend[t.String()] = w
	}
	s.mu.RUnlock()
	fx = s.GetCognitiveEffects()
	return &d, arousal, valence, blend, fx
}

// History returns the recorded emotion events (most recent last).
func (s *System) History() []EmotionEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]EmotionEvent, len(s.history))
	copy(out, s.history)
	return out
}
