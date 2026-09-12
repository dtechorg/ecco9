package echodream

import "testing"

func TestSingletonMemoryRemainsEligibleForLaterConsolidation(t *testing.T) {
	s := New()
	firstID, _ := s.SubmitMemory(EpisodicMemory{Content: "recurring pattern", Importance: 0.9})
	s.StartDreamCycle(10)
	if got := s.GetState().MemoriesProcessed; got != 0 {
		t.Fatalf("singleton memory was marked processed: got %d", got)
	}
	if got := len(s.Knowledge()); got != 0 {
		t.Fatalf("singleton memory unexpectedly created %d knowledge items", got)
	}

	secondID, _ := s.SubmitMemory(EpisodicMemory{Content: "recurring signal", Importance: 0.8})
	s.StartDreamCycle(10)
	state := s.GetState()
	if state.MemoriesProcessed != 2 {
		t.Fatalf("expected both memories to consolidate, got %d processed", state.MemoriesProcessed)
	}
	knowledge := s.Knowledge()
	if len(knowledge) != 1 {
		t.Fatalf("expected one knowledge item, got %d", len(knowledge))
	}
	if len(knowledge[0].SourceMemoryIDs) != 2 || knowledge[0].SourceMemoryIDs[0] != firstID || knowledge[0].SourceMemoryIDs[1] != secondID {
		t.Fatalf("unexpected source memories: %#v", knowledge[0].SourceMemoryIDs)
	}
}
