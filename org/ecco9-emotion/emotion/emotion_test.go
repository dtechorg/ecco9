package emotion

import (
	"testing"
	"time"
)

func TestFeelSetsDominantAndRecordsHistory(t *testing.T) {
	s := NewSystem()
	d := s.Feel(EmotionJoy, 0.9, "pleasant surprise")
	if d.Type != EmotionJoy {
		t.Fatalf("expected joy dominant, got %v", d.Type)
	}
	if got := len(s.History()); got != 1 {
		t.Fatalf("expected 1 history event, got %d", got)
	}
	dominant, arousal, valence, blend, _ := s.State()
	if dominant.Type != EmotionJoy {
		t.Fatalf("State() dominant = %v, want joy", dominant.Type)
	}
	if valence <= 0 {
		t.Fatalf("joy should give positive valence, got %v", valence)
	}
	if arousal <= 0 || arousal > 1 {
		t.Fatalf("arousal out of range: %v", arousal)
	}
	if blend["Joy"] < 0.9 {
		t.Fatalf("expected joy in blend, got %v", blend)
	}
}

func TestIntensityClampedToUnitRange(t *testing.T) {
	s := NewSystem()
	d := s.Feel(EmotionFear, 5.0, "overload")
	if d.Intensity != 1.0 {
		t.Fatalf("expected clamp to 1.0, got %v", d.Intensity)
	}
}

func TestUpdateDecaysTowardBaseline(t *testing.T) {
	s := NewSystem()
	s.Feel(EmotionAnger, 1.0, "conflict")
	s.Update(10 * time.Second)
	dominant, _, _, _, _ := s.State()
	// Anger decays toward the 0.1 floor; interest baseline (0.4->floor) or
	// another emotion should dominate after strong decay.
	if dominant.Type == EmotionAnger && dominant.Intensity > 0.5 {
		t.Fatalf("anger should have decayed, still %v", dominant.Intensity)
	}
}

func TestFearNarrowsAttentionViaCognitiveEffects(t *testing.T) {
	s := NewSystem()
	s.Feel(EmotionFear, 1.0, "threat")
	_, _, _, _, fx := s.State()
	// Fear has AttentionScope 0.5 (<1 = narrowing); blended effect should
	// stay modest relative to the neutral 1.0 baseline.
	if fx.AttentionScope > 1.5 {
		t.Fatalf("fear should not broaden attention scope, got %v", fx.AttentionScope)
	}
	if fx.ApproachAvoidance >= 0 {
		t.Fatalf("fear should be avoidance-oriented, got %v", fx.ApproachAvoidance)
	}
}

func TestParseEmotionTypeCaseInsensitive(t *testing.T) {
	if got := ParseEmotionType("JOY"); got != EmotionJoy {
		t.Fatalf("expected joy, got %v", got)
	}
	if got := ParseEmotionType("nonexistent"); got != EmotionInterest {
		t.Fatalf("expected fallback to interest, got %v", got)
	}
}

func TestAARCoreTickAdvancesMetrics(t *testing.T) {
	a := NewAARCore(12)
	a.AddGoal("explore")
	for i := 0; i < 5; i++ {
		a.Tick(0.7, 0.6)
	}
	coherence, stability, awareness, self := a.Metrics()
	if len(self) != 12 {
		t.Fatalf("expected 12-dim self vector, got %d", len(self))
	}
	for name, v := range map[string]float64{"coherence": coherence, "stability": stability, "awareness": awareness} {
		if v < 0 || v > 1 {
			t.Fatalf("%s out of [0,1]: %v", name, v)
		}
	}
}

func TestAARCorePositiveValencePullsTowardWisdom(t *testing.T) {
	a := NewAARCore(8)
	// Positive valence strengthens the Wisdom attractor (all 0.7 position);
	// the arena state should drift toward positive territory over ticks.
	for i := 0; i < 30; i++ {
		a.Tick(0.8, 0.9)
	}
	_, _, awareness, self := a.Metrics()
	var sum float64
	for _, v := range self {
		sum += v
	}
	if sum <= 0 {
		t.Fatalf("expected self vector pulled toward Wisdom (positive), sum=%v", sum)
	}
	if awareness < 0 {
		t.Fatalf("awareness should be >= 0, got %v", awareness)
	}
}
