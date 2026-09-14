package memory

import (
	"path/filepath"
	"testing"
)

func buildGraph(t *testing.T) (*HypergraphMemory, string, string, string) {
	t.Helper()
	hg := NewHypergraphMemory()
	a, err := hg.AddNode(&MemoryNode{Type: NodeConcept, Content: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	b, err := hg.AddNode(&MemoryNode{Type: NodeConcept, Content: "beta"})
	if err != nil {
		t.Fatal(err)
	}
	c, err := hg.AddNode(&MemoryNode{Type: NodePattern, Content: "gamma"})
	if err != nil {
		t.Fatal(err)
	}
	return hg, a, b, c
}

func TestAddNodeAssignsIDAndDefaults(t *testing.T) {
	hg := NewHypergraphMemory()
	id, err := hg.AddNode(&MemoryNode{})
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected generated ID")
	}
	n, err := hg.GetNode(id)
	if err != nil {
		t.Fatal(err)
	}
	if n.Type != NodeConcept {
		t.Fatalf("expected default type concept, got %q", n.Type)
	}
	if n.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

func TestAddEdgeRequiresExistingNodes(t *testing.T) {
	hg, a, b, _ := buildGraph(t)
	if _, err := hg.AddEdge(&MemoryEdge{SourceID: a, TargetID: b}); err != nil {
		t.Fatalf("valid edge rejected: %v", err)
	}
	if _, err := hg.AddEdge(&MemoryEdge{SourceID: a, TargetID: "missing"}); err == nil {
		t.Fatal("expected error for missing target node")
	}
}

func TestTraverseBFSRespectsDepth(t *testing.T) {
	hg, a, b, c := buildGraph(t)
	if _, err := hg.AddEdge(&MemoryEdge{SourceID: a, TargetID: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := hg.AddEdge(&MemoryEdge{SourceID: b, TargetID: c}); err != nil {
		t.Fatal(err)
	}
	nodes, edges, err := hg.Traverse(a, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	// Depth 1: start + direct neighbor only.
	if len(nodes) != 2 {
		t.Fatalf("expected 2 nodes at depth<=1, got %d", len(nodes))
	}
	if len(edges) == 0 {
		t.Fatal("expected traversed edges")
	}
	nodes, _, err = hg.Traverse(a, 3, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes at depth<=3, got %d", len(nodes))
	}
	if _, _, err := hg.Traverse("nope", 2, ""); err == nil {
		t.Fatal("expected error for unknown start node")
	}
}

func TestSimilaritySearchRanksByCosine(t *testing.T) {
	hg := NewHypergraphMemory()
	if _, err := hg.AddNode(&MemoryNode{Content: "x", Embedding: []float64{1, 0}}); err != nil {
		t.Fatal(err)
	}
	yID, err := hg.AddNode(&MemoryNode{Content: "y", Embedding: []float64{0, 1}})
	if err != nil {
		t.Fatal(err)
	}
	res := hg.SimilaritySearch([]float64{0.9, 0.1}, 1)
	if len(res) != 1 {
		t.Fatalf("expected 1 result, got %d", len(res))
	}
	if res[0].ID == yID {
		t.Fatal("expected orthogonal node to rank last")
	}
}

func TestCosineSimilarityEdgeCases(t *testing.T) {
	if got := CosineSimilarity([]float64{1, 0}, []float64{1}); got != 0 {
		t.Fatalf("mismatched lengths should give 0, got %v", got)
	}
	if got := CosineSimilarity([]float64{}, []float64{}); got != 0 {
		t.Fatalf("empty vectors should give 0, got %v", got)
	}
	if got := CosineSimilarity([]float64{1, 1}, []float64{1, 1}); got < 0.999 {
		t.Fatalf("identical vectors should give ~1, got %v", got)
	}
}

func TestWeaverStrengthensFrequentConnections(t *testing.T) {
	mw := NewMemoryWeaver()
	for i := 0; i < 15; i++ {
		mw.RecordAccess("a", "b")
	}
	if got := mw.ConnectionCount(); got != 1 {
		t.Fatalf("expected 1 connection, got %d", got)
	}
	cycle := mw.WeaveConnections()
	if cycle.WeightAdjustments != 1 {
		t.Fatalf("expected 1 weight adjustment, got %d", cycle.WeightAdjustments)
	}
	if len(mw.GetAdaptationHistory()) != 1 {
		t.Fatal("expected adaptation cycle recorded")
	}
}

func TestPersistentStoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ps, err := NewPersistentStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !ps.Enabled() {
		t.Fatal("expected persistence enabled with state dir")
	}

	hg, a, b, _ := buildGraph(t)
	if _, err := hg.AddEdge(&MemoryEdge{SourceID: a, TargetID: b, Relation: "knows"}); err != nil {
		t.Fatal(err)
	}
	if err := ps.Save(hg); err != nil {
		t.Fatal(err)
	}
	if saves, _ := ps.Stats(); saves != 1 {
		t.Fatalf("expected 1 save, got %d", saves)
	}

	restored := NewHypergraphMemory()
	if err := ps.Load(restored); err != nil {
		t.Fatal(err)
	}
	n, e, _ := restored.Stats()
	if n != 3 || e != 1 {
		t.Fatalf("expected 3 nodes/1 edge after restore, got %d/%d", n, e)
	}
	if filepath.Ext(ps.stateFile) != ".json" {
		t.Fatalf("unexpected state file %q", ps.stateFile)
	}
}

func TestPersistentStoreDisabledRoundTrip(t *testing.T) {
	ps, err := NewPersistentStore("")
	if err != nil {
		t.Fatal(err)
	}
	if ps.Enabled() {
		t.Fatal("expected persistence disabled for empty state dir")
	}
	hg := NewHypergraphMemory()
	if err := ps.Save(hg); err != nil {
		t.Fatal(err)
	}
	if err := ps.Load(hg); err != nil {
		t.Fatal(err)
	}
	if saves, _ := ps.Stats(); saves != 0 {
		t.Fatal("disabled store should not record saves")
	}
}
