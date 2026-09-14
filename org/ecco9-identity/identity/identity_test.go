package identity

import (
	"math"
	"testing"
)

func TestEmbedIsDeterministicAndNormalized(t *testing.T) {
	a := Embed("deep-tree-echo")
	b := Embed("deep-tree-echo")
	if len(a) != EmbeddingDim {
		t.Fatalf("expected %d dims, got %d", EmbeddingDim, len(a))
	}
	var mag float64
	for i := range a {
		if a[i] != b[i] {
			t.Fatal("Embed is not deterministic for the same seed")
		}
		mag += a[i] * a[i]
	}
	if math.Abs(math.Sqrt(mag)-1.0) > 1e-6 {
		t.Fatalf("expected unit-norm embedding, got %v", math.Sqrt(mag))
	}
	c := Embed("different-seed")
	if CosineSimilarity(a, c) > 0.99 {
		t.Fatal("different seeds produced near-identical embeddings")
	}
}

func TestRegisterValidatesDimension(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Register("id1", make([]float64, 10)); err == nil {
		t.Fatal("expected error for wrong embedding dimension")
	}
	if _, err := r.Register("", Embed("x")); err == nil {
		t.Fatal("expected error for empty identity id")
	}
	if _, err := r.Register("id1", Embed("x")); err != nil {
		t.Fatal(err)
	}
	if got := r.Count(); got != 1 {
		t.Fatalf("expected 1 identity, got %d", got)
	}
}

func TestVerifyIdentityGenesisAndCoherence(t *testing.T) {
	r := NewRegistry()
	vec := Embed("shard-1")
	// First sighting registers and verifies at full coherence.
	ok, coh, err := r.VerifyIdentity(&IdentityEmbedding{IdentityID: "a", Vector: vec})
	if err != nil || !ok {
		t.Fatalf("genesis verify failed: ok=%v err=%v", ok, err)
	}
	if coh != 1.0 {
		t.Fatalf("expected coherence 1.0 at genesis, got %v", coh)
	}
	// Same vector again verifies.
	ok, coh, _ = r.VerifyIdentity(&IdentityEmbedding{IdentityID: "a", Vector: vec})
	if !ok || coh < 0.999 {
		t.Fatalf("re-verify failed: ok=%v coherence=%v", ok, coh)
	}
	// A very different vector should fail the threshold.
	ok, coh, _ = r.VerifyIdentity(&IdentityEmbedding{IdentityID: "a", Vector: Embed("unrelated")})
	if ok {
		t.Fatalf("mismatched identity verified at coherence %v", coh)
	}
}

func TestSimilarityMatchRanksNearestFirst(t *testing.T) {
	r := NewRegistry()
	if _, err := r.Register("near", Embed("topic-alpha")); err != nil {
		t.Fatal(err)
	}
	if _, err := r.Register("far", Embed("zzz-different")); err != nil {
		t.Fatal(err)
	}
	ids, scores := r.SimilarityMatch(Embed("topic-alpha"), 2)
	if len(ids) != 2 || len(scores) != 2 {
		t.Fatalf("expected 2 results, got %d/%d", len(ids), len(scores))
	}
	if ids[0] != "near" {
		t.Fatalf("expected 'near' first, got %q", ids[0])
	}
	if scores[0] < scores[1] {
		t.Fatal("scores not in descending order")
	}
}

func TestSwitchPersonaAndDefault(t *testing.T) {
	r := NewRegistry()
	if _, err := r.SwitchPersona("id1", "dynamic-explorer"); err != nil {
		t.Fatal(err)
	}
	if got := r.ActivePersona("id1").Name; got != "dynamic-explorer" {
		t.Fatalf("expected dynamic-explorer, got %q", got)
	}
	if got := r.ActivePersona("unknown").Name; got != "contemplative-scholar" {
		t.Fatalf("expected default contemplative-scholar, got %q", got)
	}
	if _, err := r.SwitchPersona("id1", "no-such-persona"); err == nil {
		t.Fatal("expected error for unknown persona")
	}
}

func TestTheoryOfMindTrustAndPredictability(t *testing.T) {
	tom := NewTheoryOfMind()
	// Repeated identical actions => high predictability.
	for i := 0; i < 8; i++ {
		tom.RecordAction("agent-x", "share")
	}
	m := tom.GetAgentModel("agent-x")
	if m.Predictability <= 0.5 {
		t.Fatalf("repeated action should raise predictability, got %v", m.Predictability)
	}
	// Positive outcomes increase trust from the 0.7 baseline.
	tom.UpdateTrust("agent-x", 1.0)
	if got := tom.GetAgentModel("agent-x").TrustScore; got <= 0.7 {
		t.Fatalf("trust should increase on good outcome, got %v", got)
	}
	// Negative outcomes decrease trust.
	tom.UpdateTrust("agent-x", 0.0)
	tom.UpdateTrust("agent-x", 0.0)
	if got := tom.GetAgentModel("agent-x").TrustScore; got >= 0.75 {
		t.Fatalf("trust should fall on bad outcomes, got %v", got)
	}
}

func TestTheoryOfMindPredictActionAndDeception(t *testing.T) {
	tom := NewTheoryOfMind()
	if got := tom.PredictAction("nobody"); got != "unknown_action" {
		t.Fatalf("expected unknown_action for untracked agent, got %q", got)
	}
	for i := 0; i < 4; i++ {
		tom.RecordAction("agent-y", "explore")
	}
	if got := tom.PredictAction("agent-y"); got != "explore" {
		t.Fatalf("expected most common action, got %q", got)
	}
	if got := tom.DetectDeception("nobody"); got != 0.5 {
		t.Fatalf("expected neutral 0.5 for unknown agent, got %v", got)
	}
	// Only RecordAction created a model; PredictAction/DetectDeception use
	// read paths and must not materialize models for unknown agents.
	if got := tom.ModelCount(); got != 1 {
		t.Fatalf("expected 1 model, got %d", got)
	}
}

func TestStateStoreRoundTrip(t *testing.T) {
	ss, err := NewStateStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if !ss.Enabled() {
		t.Fatal("expected store enabled")
	}
	in := &ConsciousnessState{
		IdentityName:   "echo",
		AwarenessLevel: 0.8,
		InterestTopics: map[string]float64{"patterns": 0.9},
		ActivePersona:  "dynamic-explorer",
	}
	if err := ss.Save(in); err != nil {
		t.Fatal(err)
	}
	if ss.SaveCount() != 1 {
		t.Fatal("expected 1 save")
	}
	out := ss.Load("echo")
	if out.AwarenessLevel != 0.8 || out.ActivePersona != "dynamic-explorer" {
		t.Fatalf("state not restored: %+v", out)
	}
	if out.InterestTopics["patterns"] != 0.9 {
		t.Fatal("interest topics not restored")
	}
	// A fresh session ID is minted for continuity-across-sessions.
	if out.SessionID == "" {
		t.Fatal("expected a session id")
	}
}

func TestStateStoreGenesisWhenEmpty(t *testing.T) {
	ss, err := NewStateStore("")
	if err != nil {
		t.Fatal(err)
	}
	if ss.Enabled() {
		t.Fatal("expected store disabled for empty dir")
	}
	g := ss.Load("echo")
	if g.ActivePersona != "contemplative-scholar" || g.AwarenessLevel != 0.5 {
		t.Fatalf("unexpected genesis state: %+v", g)
	}
}
