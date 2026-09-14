package atomspace

import (
	"math"
	"testing"
)

func TestAddAtomAndLink(t *testing.T) {
	as := New()
	cat, err := as.AddAtom(AtomTypeConcept, "cat", nil)
	if err != nil {
		t.Fatal(err)
	}
	animal, err := as.AddAtom(AtomTypeConcept, "animal", nil)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Truth == nil || cat.Truth.Strength != 1.0 {
		t.Fatal("default truth value should have strength 1.0")
	}
	link, err := as.AddLink(InheritanceLink, []string{cat.ID, animal.ID}, &TruthValue{Strength: 0.9, Confidence: 0.8})
	if err != nil {
		t.Fatal(err)
	}
	if got := len(as.GetIncoming(animal.ID)); got != 1 {
		t.Fatalf("expected 1 incoming link for animal, got %d", got)
	}
	if _, ok := as.GetLink(link.ID); !ok {
		t.Fatal("link not retrievable")
	}
	if _, err := as.AddLink(InheritanceLink, []string{cat.ID, "missing"}, nil); err == nil {
		t.Fatal("expected error for dangling link target")
	}
}

func TestSpreadAttentionMovesSTI(t *testing.T) {
	as := New()
	a, _ := as.AddAtom(AtomTypeConcept, "a", nil)
	b, _ := as.AddAtom(AtomTypeConcept, "b", nil)
	link, _ := as.AddLink(SimilarityLink, []string{a.ID, b.ID}, &TruthValue{Strength: 1.0})
	as.UpdateAttention(link.ID, &AttentionValue{STI: 100})
	before := a.Attention.STI
	as.SpreadAttention()
	if a.Attention.STI <= before {
		t.Fatalf("expected STI to flow to atom a (before=%d after=%d)", before, a.Attention.STI)
	}
	l, _ := as.GetLink(link.ID)
	if l.Attention.STI >= 100 {
		t.Fatalf("link STI should have decreased, got %d", l.Attention.STI)
	}
}

func TestForgetPrunesOrphanedLowAttentionAtoms(t *testing.T) {
	as := New()
	orphan, _ := as.AddAtom(AtomTypeConcept, "orphan", nil)
	// Default attention AF=0 is below the forgetting threshold.
	as.Forget()
	if _, ok := as.GetAtom(orphan.ID); ok {
		t.Fatal("orphaned low-attention atom should be forgotten")
	}
}

func TestPatternMatchBindsVariables(t *testing.T) {
	as := New()
	cat, _ := as.AddAtom(AtomTypeConcept, "cat", nil)
	dog, _ := as.AddAtom(AtomTypeConcept, "dog", nil)
	animal, _ := as.AddAtom(AtomTypeConcept, "animal", nil)
	if _, err := as.AddLink(InheritanceLink, []string{cat.ID, animal.ID}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := as.AddLink(InheritanceLink, []string{dog.ID, animal.ID}, nil); err != nil {
		t.Fatal(err)
	}

	p := ParsePattern("InheritanceLink($x, " + animal.ID + ")")
	res := as.Match(p)
	if res.Count != 2 {
		t.Fatalf("expected 2 bindings, got %d", res.Count)
	}
	found := map[string]bool{}
	for _, b := range res.Bindings {
		found[b["$x"]] = true
	}
	if !found[cat.ID] || !found[dog.ID] {
		t.Fatalf("expected bindings for cat and dog, got %v", res.Bindings)
	}
}

func TestParsePatternIgnoresMalformedClauses(t *testing.T) {
	p := ParsePattern("not a clause; InheritanceLink($x, $y)")
	if len(p.Clauses) != 1 {
		t.Fatalf("expected 1 valid clause, got %d", len(p.Clauses))
	}
	if !p.Variables["$x"] || !p.Variables["$y"] {
		t.Fatal("variables not recorded")
	}
}

func TestFuseTruthValues(t *testing.T) {
	a := &TruthValue{Strength: 0.8, Confidence: 0.6, Count: 2}
	b := &TruthValue{Strength: 0.5, Confidence: 0.4, Count: 3}
	and := FuseTruthValues(a, b, "and")
	if and.Strength != 0.4 {
		t.Fatalf("and strength = %v, want 0.4", and.Strength)
	}
	or := FuseTruthValues(a, b, "or")
	if or.Strength != 0.9 {
		t.Fatalf("or strength = %v, want 0.9", or.Strength)
	}
	not := FuseTruthValues(a, nil, "not")
	if math.Abs(not.Strength-0.2) > 1e-9 {
		t.Fatalf("not strength = %v, want ~0.2", not.Strength)
	}
	rev := FuseTruthValues(a, b, "revision")
	if rev.Count != 5 {
		t.Fatalf("revision should sum counts, got %v", rev.Count)
	}
}

func TestStatusReportsCounts(t *testing.T) {
	as := New()
	if _, err := as.AddAtom(AtomTypeConcept, "solo", nil); err != nil {
		t.Fatal(err)
	}
	st := as.Status()
	if st["atoms"].(int) != 1 {
		t.Fatalf("expected 1 atom in status, got %v", st["atoms"])
	}
	if st["sti_funds"].(int64) != 100000 {
		t.Fatalf("unexpected STI funds: %v", st["sti_funds"])
	}
}
