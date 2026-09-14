package wisdom

import (
	"testing"
)

func TestSevenDimensionalAssessmentInRange(t *testing.T) {
	sdw := NewSevenDimensionalWisdom()
	sdw.Update(0.6, 0.5, 0.7, 0.4, 0.8, 0.9, 0.5)
	a := sdw.Assess()
	if len(a.Dimensions) != 7 {
		t.Fatalf("expected 7 dimensions, got %d", len(a.Dimensions))
	}
	if a.Overall < 0 || a.Overall > 1 {
		t.Fatalf("overall wisdom out of [0,1]: %v", a.Overall)
	}
	if a.Coherence < 0 || a.Coherence > 1 {
		t.Fatalf("coherence out of [0,1]: %v", a.Coherence)
	}
}

func TestCoherenceHigherForUniformDimensions(t *testing.T) {
	uniform := NewSevenDimensionalWisdom()
	uniform.Update(0.5, 0.5, 0.5, 0.5, 0.5, 0.5, 0.5)
	skewed := NewSevenDimensionalWisdom()
	skewed.Update(0.1, 0.9, 0.2, 0.8, 0.1, 0.9, 0.3)
	if uniform.Assess().Coherence <= skewed.Assess().Coherence {
		t.Fatalf("uniform dims should be more coherent: uniform=%v skewed=%v",
			uniform.Assess().Coherence, skewed.Assess().Coherence)
	}
}

func TestDimensionValuesClamped(t *testing.T) {
	sdw := NewSevenDimensionalWisdom()
	sdw.Update(5.0, -2.0, 0.5, 0.5, 0.5, 0.5, 0.5)
	a := sdw.Assess()
	if a.Dimensions[DimKnowledgeDepth] != 1.0 {
		t.Fatalf("expected clamp to 1.0, got %v", a.Dimensions[DimKnowledgeDepth])
	}
	if a.Dimensions[DimKnowledgeBreadth] != 0.0 {
		t.Fatalf("expected clamp to 0.0, got %v", a.Dimensions[DimKnowledgeBreadth])
	}
}

func TestGoalOrchestratorLifecycle(t *testing.T) {
	o := NewGoalOrchestrator()
	id, err := o.CreateGoal(Goal{Type: GoalTypeExploration, Description: "explore", Priority: 0.8})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected generated goal ID")
	}
	goals := o.ListGoals(GoalTypeUnspecified)
	if len(goals) != 1 {
		t.Fatalf("expected 1 goal, got %d", len(goals))
	}
	if !o.AdvanceGoal(id, 0.5) {
		t.Fatal("advance failed")
	}
	if o.ListGoals(GoalTypeUnspecified)[0].Completed {
		t.Fatal("goal should not be complete at 0.5")
	}
	o.AdvanceGoal(id, 0.6) // crosses 1.0
	if !o.ListGoals(GoalTypeUnspecified)[0].Completed {
		t.Fatal("goal should be complete at >=1.0")
	}
}

func TestGoalOrchestratorCapsActiveGoals(t *testing.T) {
	o := NewGoalOrchestrator()
	for i := 0; i < 5; i++ {
		if _, err := o.CreateGoal(Goal{Description: "g"}); err != nil {
			t.Fatalf("goal %d: %v", i, err)
		}
	}
	if _, err := o.CreateGoal(Goal{Description: "overflow"}); err == nil {
		t.Fatal("expected error when exceeding maxActive (5)")
	}
}

func TestInterestGeneratorSeedsAndEngagement(t *testing.T) {
	g := NewInterestDrivenGenerator()
	seeded := g.StrongestInterests(5)
	if len(seeded) == 0 {
		t.Fatal("expected seeded interest patterns")
	}
	// Engagement strengthens a new topic and it becomes discoverable.
	g.RecordEngagement("reservoir computing", 0.9)
	g.RecordEngagement("reservoir computing", 0.9)
	g.RecordEngagement("reservoir computing", 0.9)
	found := false
	for _, p := range g.StrongestInterests(20) {
		if p.Topic == "reservoir computing" {
			found = true
			if p.Strength <= 0 {
				t.Fatal("engagement should raise strength")
			}
		}
	}
	if !found {
		t.Fatal("engaged topic not among strongest interests")
	}
}

func TestApplicationEngineFindsRelevantWisdom(t *testing.T) {
	e := NewApplicationEngine()
	// Foundational wisdom about learning should match a learning context.
	matches := e.FindRelevantWisdom("continuous learning and growth", 3)
	if len(matches) == 0 {
		t.Fatal("expected at least one relevant wisdom match")
	}
	if matches[0].Applicability <= 0.3 {
		t.Fatalf("match below threshold returned: %v", matches[0].Applicability)
	}
	// Applying wisdom increments the use counter.
	before := matches[0].Entry.TimesApplied
	e.FindRelevantWisdom("continuous learning and growth", 3)
	if matches[0].Entry.TimesApplied <= before {
		t.Fatal("TimesApplied should increase after application")
	}
}

func TestAddWisdomAndRetrieve(t *testing.T) {
	e := NewApplicationEngine()
	id := e.AddWisdom(WisdomEntry{
		Content: "Rest consolidates memory", Category: "dream",
		Applicability: []string{"sleep", "consolidation"},
		Confidence:    0.8, Usefulness: 0.7, Specificity: 0.5,
	})
	if id == "" {
		t.Fatal("expected generated wisdom ID")
	}
	matches := e.FindRelevantWisdom("memory consolidation during sleep", 5)
	found := false
	for _, m := range matches {
		if m.Entry.ID == id {
			found = true
		}
	}
	if !found {
		t.Fatal("newly added wisdom not discoverable via matching context")
	}
}
