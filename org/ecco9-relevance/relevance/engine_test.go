package relevance

import (
	"testing"
)

func TestEngineScoresWithinBounds(t *testing.T) {
	e := NewEngine()
	s := e.RealizeRelevance("cognitive architectures", map[string]string{"domain": "agi"})
	if s.Relevance < 0 || s.Relevance > 1 {
		t.Fatalf("relevance out of [0,1]: %v", s.Relevance)
	}
	if s.Salience < 0 || s.Salience > 1 {
		t.Fatalf("salience out of [0,1]: %v", s.Salience)
	}
	if s.Affordance < 0 || s.Affordance > 1 {
		t.Fatalf("affordance out of [0,1]: %v", s.Affordance)
	}
	if len(s.Facets) == 0 {
		t.Fatal("expected active facets at baseline 0.5")
	}
}

func TestContextBoostsSalience(t *testing.T) {
	e := NewEngine()
	noCtx := e.RealizeRelevance("x", nil)
	withCtx := e.RealizeRelevance("x", map[string]string{"a": "1", "b": "2", "c": "3"})
	if withCtx.Salience < noCtx.Salience {
		t.Fatalf("context should not reduce salience: noCtx=%v withCtx=%v", noCtx.Salience, withCtx.Salience)
	}
}

func TestUpdateFromExperienceMovesDimensions(t *testing.T) {
	e := NewEngine()
	before := e.Coherence()
	e.UpdateFromExperience(1.0) // strong positive feedback
	st := e.Status()
	knowing := st["knowing"].(map[string]float64)
	if knowing["propositional"] <= 0.5 {
		t.Fatalf("positive feedback should raise propositional knowing, got %v", knowing["propositional"])
	}
	// Significant feedback (>0.5) should transform participatory knowing.
	if knowing["participatory"] <= 0.5 {
		t.Fatalf("significant feedback should raise participatory knowing, got %v", knowing["participatory"])
	}
	if e.Coherence() == before {
		t.Fatal("coherence should be recomputed after experience")
	}
}

func TestWeakFeedbackDoesNotMoveWisdom(t *testing.T) {
	e := NewEngine()
	e.UpdateFromExperience(0.2) // below the 0.3 wisdom threshold
	w := e.Status()["wisdom"].(map[string]float64)
	// Wisdom dimensions only develop on significant feedback; with 0.2 the
	// morality update is skipped (though cross-triad mixing nudges slightly).
	if w["morality"] > 0.55 {
		t.Fatalf("weak feedback should barely move wisdom, got %v", w["morality"])
	}
}

func TestRelevanceHistoryTracked(t *testing.T) {
	e := NewEngine()
	for i := 0; i < 5; i++ {
		e.RealizeRelevance("t", nil)
	}
	st := e.Status()
	if st["total_scored"].(uint64) != 5 {
		t.Fatalf("expected 5 scored, got %v", st["total_scored"])
	}
}

func TestTriadAnalysesProduceOverallScores(t *testing.T) {
	ka := NewKnowingTriad().Analyze()
	if ka.OverallScore <= 0 || ka.OverallScore > 1 {
		t.Fatalf("knowing overall out of range: %v", ka.OverallScore)
	}
	ua := NewUnderstandingTriad().Analyze()
	if ua.OverallScore <= 0 || ua.OverallScore > 1 {
		t.Fatalf("understanding overall out of range: %v", ua.OverallScore)
	}
	wa := NewWisdomTriad().Analyze()
	if wa.OverallScore <= 0 || wa.OverallScore > 1 {
		t.Fatalf("wisdom overall out of range: %v", wa.OverallScore)
	}
}

func TestUpdateSalienceBoostsUnderdevelopedDims(t *testing.T) {
	rp := NewRealizationProcess()
	rp.UpdateSalience(map[string]float64{"weak": 0.2, "strong": 0.9})
	sal := rp.Salience()
	// Underdeveloped dimension (<0.4) gets boosted toward 0.4.
	if sal["weak"] <= 0.2 {
		t.Fatalf("weak dim should be boosted, got %v", sal["weak"])
	}
	if sal["strong"] != 0.9 {
		t.Fatalf("strong dim should be unchanged, got %v", sal["strong"])
	}
}
