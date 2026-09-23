package metacog

import (
	"testing"
)

func TestTrackCreatesAndUpdatesProcess(t *testing.T) {
	m := NewMonitor()
	id, ok := m.Track(CognitiveProcess{Name: "reasoning", Type: "reasoning", Service: "reservoir"})
	if !ok || id == "" {
		t.Fatalf("track failed: ok=%v id=%q", ok, id)
	}
	if got := m.Status()["active_processes"].(int); got != 1 {
		t.Fatalf("expected 1 active process, got %d", got)
	}
	// Update same ID to completion removes it from the active set.
	m.Track(CognitiveProcess{ID: id, Progress: 1.0, Completed: true, Success: true})
	if got := m.Status()["active_processes"].(int); got != 0 {
		t.Fatalf("completed process should leave active set, got %d", got)
	}
}

func TestFailedLowQualityProcessCreatesLearningGap(t *testing.T) {
	m := NewMonitor()
	id, _ := m.Track(CognitiveProcess{Name: "weak", Type: "reasoning", Service: "echo"})
	m.Track(CognitiveProcess{ID: id, Completed: true, Success: false, QualityScore: 0.2})
	gaps := m.LearningGaps()
	if len(gaps) == 0 {
		t.Fatal("expected a learning gap from failed low-quality process")
	}
	if gaps[0].Domain != "reasoning" {
		t.Fatalf("gap domain = %q, want reasoning", gaps[0].Domain)
	}
}

func TestRecordAndAssessDecision(t *testing.T) {
	m := NewMonitor()
	id := m.RecordDecision("choose strategy", "deliberate", "high stakes", 0.9)
	if id == "" {
		t.Fatal("expected decision ID")
	}
	a := m.AssessDecision(id, true, 1.0, 1.0, 0)
	if a == nil {
		t.Fatal("expected assessment")
	}
	if a.OverallQuality < 0 || a.OverallQuality > 1 {
		t.Fatalf("overall quality out of range: %v", a.OverallQuality)
	}
	// Perfect outcome with no side effects should be high quality.
	if a.OverallQuality < 0.6 {
		t.Fatalf("expected high quality for good outcome, got %v", a.OverallQuality)
	}
	if got := m.AssessDecision("nonexistent", true, 1, 1, 0); got != nil {
		t.Fatal("expected nil for unknown decision")
	}
}

func TestAssessDecisionSuggestsImprovementsOnFailure(t *testing.T) {
	m := NewMonitor()
	id := m.RecordDecision("rushed call", "intuitive", "no time", 0.3)
	a := m.AssessDecision(id, false, 1.0, 0.1, 3)
	if a.OverallQuality >= 0.7 {
		t.Fatalf("poor outcome should be low quality, got %v", a.OverallQuality)
	}
	if len(a.ImprovementSuggestions) == 0 {
		t.Fatal("expected improvement suggestions for low-quality decision")
	}
}

func TestSelectStrategyPrefersSpeedWhenUrgent(t *testing.T) {
	m := NewMonitor()
	// "pattern_recognition" makes the intuitive strategy applicable; under
	// time pressure its high speed (0.9) outscores the slower strategies.
	id, score := m.SelectStrategy("urgent pattern_recognition task", nil)
	if id == "" {
		t.Fatal("expected a strategy")
	}
	if score < 0 || score > 1 {
		t.Fatalf("score out of range: %v", score)
	}
	if id != "intuitive" {
		t.Fatalf("expected intuitive strategy under time pressure, got %q", id)
	}
}

func TestSelectStrategyRespectsCandidates(t *testing.T) {
	m := NewMonitor()
	id, _ := m.SelectStrategy("any problem", []string{"analytical"})
	if id != "analytical" {
		t.Fatalf("expected restricted candidate analytical, got %q", id)
	}
}

func TestAssessSystemProducesRecommendations(t *testing.T) {
	m := NewMonitor()
	a := m.AssessSystem()
	if a.IdentityCoherence < 0 || a.IdentityCoherence > 1 {
		t.Fatalf("coherence out of range: %v", a.IdentityCoherence)
	}
	if a.GlobalCognitiveLoad < 0 || a.GlobalCognitiveLoad > 1 {
		t.Fatalf("load out of range: %v", a.GlobalCognitiveLoad)
	}
	if len(a.Recommendations) == 0 {
		t.Fatal("expected at least the default recommendation")
	}
}

func TestAssessSystemLoadRisesWithActiveProcesses(t *testing.T) {
	m := NewMonitor()
	low := m.AssessSystem().GlobalCognitiveLoad
	for i := 0; i < 16; i++ {
		m.Track(CognitiveProcess{Name: "p", Type: "t", Service: "s"})
	}
	high := m.AssessSystem().GlobalCognitiveLoad
	if high <= low {
		t.Fatalf("load should rise with active processes: low=%v high=%v", low, high)
	}
}

func TestEvaluateTrainingQualityPassesWithinGate(t *testing.T) {
	m := NewMonitor()
	v := m.EvaluateTrainingQuality(7, 0.10, 0.05, 128)
	if !v.Passed {
		t.Fatalf("MSE 0.10 should pass gate %.2f", MaxHeldOutMSE)
	}
	if v.Quality <= 0 || v.Quality > 1 {
		t.Fatalf("quality out of range: %v", v.Quality)
	}
	if got, ok := m.TrainingValidation(7); !ok || got != v {
		t.Fatalf("validation should be recorded: ok=%v", ok)
	}
	// A passing validation is a successful process: no learning gap.
	if len(m.LearningGaps()) != 0 {
		t.Fatalf("passing validation should not open a gap, got %d", len(m.LearningGaps()))
	}
}

func TestEvaluateTrainingQualityFailureRecommendsRollback(t *testing.T) {
	m := NewMonitor()
	before := m.Status()["total_processes"].(uint64)
	v := m.EvaluateTrainingQuality(9, 0.95, 0.4, 64)
	if v.Passed {
		t.Fatalf("MSE 0.95 should fail gate %.2f", MaxHeldOutMSE)
	}
	// Failure of a low-quality process opens a learning gap.
	if len(m.LearningGaps()) == 0 {
		t.Fatal("failed validation should open a learning gap")
	}
	if got := m.Status()["total_processes"].(uint64); got <= before {
		t.Fatal("validation should track a cognitive process")
	}
	recs := m.AssessSystem().Recommendations
	found := false
	for _, r := range recs {
		if containsAny(r, "rollback") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected rollback recommendation, got %v", recs)
	}
}
