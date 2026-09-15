package runner

import (
	"testing"
)

func TestRegistryLoadTransitionsToReady(t *testing.T) {
	reg := NewRegistry()
	m, err := reg.Load(LoadParams{ModelPath: "/models/a.gguf", GPULayers: 8, ContextSize: 2048}, &StubBackend{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if m.Status != StatusReady {
		t.Fatalf("Status = %v, want ready", m.Status)
	}
	if m.Progress != 1.0 {
		t.Fatalf("Progress = %v, want 1.0", m.Progress)
	}
	if m.Cache == nil {
		t.Fatal("expected KV cache to be allocated on load")
	}
	if m.ID == "" {
		t.Fatal("expected a generated model ID")
	}
}

func TestRegistryLoadDuplicatePathConflicts(t *testing.T) {
	reg := NewRegistry()
	b := &StubBackend{}
	if _, err := reg.Load(LoadParams{ModelPath: "/models/dup.gguf"}, b); err != nil {
		t.Fatalf("first Load: %v", err)
	}
	if _, err := reg.Load(LoadParams{ModelPath: "/models/dup.gguf"}, b); err == nil {
		t.Fatal("expected duplicate-path load to fail")
	}
}

func TestRegistryLoadBackendErrorMarksError(t *testing.T) {
	reg := NewRegistry()
	// StubBackend.Load rejects an empty model path.
	if _, err := reg.Load(LoadParams{ModelPath: ""}, &StubBackend{}); err == nil {
		t.Fatal("expected load error for empty path")
	}
}

func TestRegistryGetByIDAndPath(t *testing.T) {
	reg := NewRegistry()
	m, err := reg.Load(LoadParams{ModelPath: "/models/find.gguf"}, &StubBackend{})
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	byID, err := reg.Get(m.ID)
	if err != nil || byID != m {
		t.Fatalf("Get by ID = %v, %v", byID, err)
	}
	byPath, err := reg.Get("/models/find.gguf")
	if err != nil || byPath != m {
		t.Fatalf("Get by path = %v, %v", byPath, err)
	}
	if _, err := reg.Get("nonexistent"); err != ErrModelNotFound {
		t.Fatalf("expected ErrModelNotFound, got %v", err)
	}
}

func TestRegistryUnloadRemovesModel(t *testing.T) {
	reg := NewRegistry()
	b := &StubBackend{}
	m, err := reg.Load(LoadParams{ModelPath: "/models/u.gguf"}, b)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reg.Unload(m.ID, b) {
		t.Fatal("Unload should return true for a loaded model")
	}
	if _, err := reg.Get(m.ID); err != ErrModelNotFound {
		t.Fatalf("expected ErrModelNotFound after unload, got %v", err)
	}
	// Second unload reports false.
	if reg.Unload(m.ID, b) {
		t.Fatal("Unload of missing model should return false")
	}
}

func TestRegistryListReturnsLoadedModels(t *testing.T) {
	reg := NewRegistry()
	b := &StubBackend{}
	for _, p := range []string{"/models/1.gguf", "/models/2.gguf", "/models/3.gguf"} {
		if _, err := reg.Load(LoadParams{ModelPath: p}, b); err != nil {
			t.Fatalf("Load %s: %v", p, err)
		}
	}
	if got := len(reg.List()); got != 3 {
		t.Fatalf("List len = %d, want 3", got)
	}
}

func TestEstimateMemoryScalesWithLayers(t *testing.T) {
	size := uint64(8 << 30) // 8 GiB
	full := EstimateMemory(size, 32, LoadParams{GPULayers: 32, ContextSize: 4096})
	part := EstimateMemory(size, 32, LoadParams{GPULayers: 16, ContextSize: 4096})
	if !full.Fully {
		t.Fatal("32/32 layers should report Fully")
	}
	if part.Fully {
		t.Fatal("16/32 layers should not report Fully")
	}
	if part.VRAMBytes >= full.VRAMBytes {
		t.Fatalf("partial VRAM (%d) should be less than full (%d)", part.VRAMBytes, full.VRAMBytes)
	}
	// Zero/negative gpu layers falls back to all layers.
	auto := EstimateMemory(size, 32, LoadParams{GPULayers: 0, ContextSize: 4096})
	if auto.Layers != 32 {
		t.Fatalf("auto layers = %d, want 32", auto.Layers)
	}
}

func TestStubBackendForwardEmitsTokens(t *testing.T) {
	b := &StubBackend{}
	if err := b.Load(LoadParams{ModelPath: "/models/s.gguf"}); err != nil {
		t.Fatalf("Load: %v", err)
	}
	var tokens []string
	var sawDone bool
	pt, ct, err := b.Forward("model-1", &GenerateRequest{
		Messages: []ChatMessage{{Role: "user", Content: "hello there"}},
	}, func(tok string, done bool) {
		tokens = append(tokens, tok)
		if done {
			sawDone = true
		}
	})
	if err != nil {
		t.Fatalf("Forward: %v", err)
	}
	if !sawDone {
		t.Fatal("expected a terminal done token")
	}
	if pt != 2 {
		t.Fatalf("prompt tokens = %d, want 2", pt)
	}
	if ct != len(tokens) {
		t.Fatalf("completion tokens = %d, want %d (emitted)", ct, len(tokens))
	}
}

func TestStubBackendForwardWithoutLoadFails(t *testing.T) {
	b := &StubBackend{}
	if _, _, err := b.Forward("m", &GenerateRequest{}, func(string, bool) {}); err == nil {
		t.Fatal("expected error when forwarding before load")
	}
}
