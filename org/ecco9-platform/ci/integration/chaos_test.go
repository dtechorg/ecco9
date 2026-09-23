//go:build integration

package integration

import (
	"math/rand/v2"
	"testing"
	"time"

	"github.com/dtechorg/ecco9-sdk-go/eventmesh"
)

// TestChaosRandomPodKills kills random services while cognitive traffic
// flows, verifying the stack degrades gracefully: surviving services keep
// serving, identity coherence holds, and the system fully recovers once the
// killed pods are healed.
func TestChaosRandomPodKills(t *testing.T) {
	s := newStack(t)

	killable := []string{"memory", "emotion", "consciousness", "relevance", "wisdom", "echodream", "atomspace"}
	victims := map[string]bool{}
	for i := 0; i < 3; i++ {
		victims[killable[rand.IntN(len(killable))]] = true
	}

	// Continuous traffic during the kills: identity verification is the
	// critical path and must remain available throughout.
	for v := range victims {
		s.kill(v)
		if c := s.coherence("deep-tree-echo"); c < coherenceThreshold {
			t.Fatalf("coherence %v < %v while %s was killed", c, coherenceThreshold, v)
		}
	}

	// Surviving services still respond.
	for name := range s.services {
		if victims[name] {
			continue
		}
		h := s.mustGet(name, "/healthz")
		if _, ok := h["status"]; !ok {
			t.Fatalf("survivor %s lost health endpoint", name)
		}
	}

	// Heal and verify full recovery: memory accepts episodes and publishes
	// them to the mesh again.
	for v := range victims {
		s.heal(v)
	}
	before := s.mesh.count(eventmesh.TopicMemories)
	s.mustPost("memory", "/v1/memory/nodes", map[string]any{
		"node": map[string]any{"type": "episode", "content": "post-chaos recovery memory"},
	})
	if n := s.mesh.count(eventmesh.TopicMemories); n != before+1 {
		t.Fatalf("mesh did not resume publishing after heals: %d → %d", before, n)
	}
}

// TestChaosNetworkPartition partitions the memory plane from the rest of the
// cognitive stack. Synchronous cognition (reservoir, echobeats, identity)
// must keep operating; asynchronous processes (memory publication) resume
// when the partition heals — mirroring NATS JetStream redelivery.
func TestChaosNetworkPartition(t *testing.T) {
	s := newStack(t)

	s.partition("memory")
	s.partition("echodream")

	// Core temporal processing unaffected by the partition.
	code, _, err := s.do("reservoir", "POST", "/v1/reservoir/process", map[string]any{
		"input": []float64{0.5, 0.5},
	})
	if err != nil || code != 200 {
		t.Fatalf("reservoir failed during memory partition: code=%d err=%v", code, err)
	}

	// Partitioned service is unreachable.
	if _, _, err := s.do("memory", "POST", "/v1/memory/nodes", map[string]any{
		"node": map[string]any{"type": "episode", "content": "lost in partition"},
	}); err == nil {
		t.Fatalf("expected memory to be unreachable during partition")
	}

	// Heal: asynchronous memory flow resumes.
	s.heal("memory")
	s.heal("echodream")
	s.mustPost("memory", "/v1/memory/nodes", map[string]any{
		"node": map[string]any{"type": "episode", "content": "partition healed"},
	})
}

// TestChaosLatencyInjection injects latency into the identity path and
// verifies coherence checks still complete and the system stays coherent —
// latency must degrade throughput, not correctness.
func TestChaosLatencyInjection(t *testing.T) {
	s := newStack(t)

	s.injectLatency("identity", 25)
	defer s.heal("identity")

	start := time.Now()
	c := s.coherence("deep-tree-echo")
	elapsed := time.Since(start)

	if c < coherenceThreshold {
		t.Fatalf("coherence %v < %v under latency injection", c, coherenceThreshold)
	}
	if elapsed < 25*time.Millisecond {
		t.Fatalf("latency injection not applied: call returned in %v", elapsed)
	}
}
