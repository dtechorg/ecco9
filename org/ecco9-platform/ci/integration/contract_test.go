//go:build integration

package integration

import (
	"encoding/json"
	"testing"

	"github.com/dtechorg/ecco9-identity/identity"
	"github.com/dtechorg/ecco9-sdk-go/contracts"
	"github.com/dtechorg/ecco9-sdk-go/eventmesh"
)

// Consumer-driven contract tests (Pact-style): each test states the contract
// a consumer relies on, then verifies the provider honors it. Contracts are
// anchored in the shared types from ecco9-sdk-go/contracts and
// ecco9-sdk-go/eventmesh, which mirror the ecco9-proto definitions — the
// same surface the generated gRPC stubs will expose.

// TestContractHealthSurface: every service consumes monitoring via the common
// health contract — /healthz must expose ServingStatus, identity coherence,
// and cognitive load as defined in ecco9.common.v1.
func TestContractHealthSurface(t *testing.T) {
	s := newStack(t)

	for name := range s.services {
		h := s.mustGet(name, "/healthz")

		status, ok := h["status"].(float64)
		if !ok {
			t.Fatalf("%s /healthz missing numeric status: %v", name, h)
		}
		if contracts.ServingStatus(int32(status)) != contracts.ServingStatusServing {
			t.Fatalf("%s status=%v, want SERVING", name, status)
		}
		if c, ok := h["identity_coherence"].(float64); !ok || c < 0 || c > 1 {
			t.Fatalf("%s identity_coherence out of contract range: %v", name, h)
		}
		if _, ok := h["cognitive_load"].(float64); !ok {
			t.Fatalf("%s missing cognitive_load: %v", name, h)
		}
	}
}

// TestContractEventEnvelope: consumers of the event mesh (memory → relevance
// → reservoir training pipeline, dream consolidation) rely on the
// CloudEvents 1.0 envelope contract from ecco9.events.v1.
func TestContractEventEnvelope(t *testing.T) {
	s := newStack(t)

	s.mustPost("memory", "/v1/memory/nodes", map[string]any{
		"node": map[string]any{"type": "episode", "content": "contract episode"},
	})
	if len(s.mesh.envs) == 0 {
		t.Fatalf("no envelope published")
	}
	env := s.mesh.envs[len(s.mesh.envs)-1]

	if env.SpecVersion != "1.0" || env.ID == "" || env.Source == "" || env.Type == "" {
		t.Fatalf("envelope violates CloudEvents contract: %+v", env)
	}
	if env.Time.IsZero() {
		t.Fatalf("envelope missing time: %+v", env)
	}
	// The topic taxonomy must match the cognitive pathway constants.
	valid := map[string]bool{
		eventmesh.TopicThoughts: true, eventmesh.TopicEmotions: true,
		eventmesh.TopicMemories: true, eventmesh.TopicDreams: true,
		eventmesh.TopicResonance: true, eventmesh.TopicTraining: true,
		eventmesh.TopicMetrics: true, eventmesh.TopicDirectives: true,
	}
	if !valid[s.mesh.topics[len(s.mesh.topics)-1]] {
		t.Fatalf("publication off the topic taxonomy: %q", s.mesh.topics[len(s.mesh.topics)-1])
	}
}

// TestContractIdentityEmbedding: consumers of identity verification rely on
// 768-dimensional embeddings and a [0,1] coherence score — the surface
// declared by ecco9.identity.v1 and mirrored in the SDK contracts.
func TestContractIdentityEmbedding(t *testing.T) {
	s := newStack(t)

	if identity.EmbeddingDim != 768 {
		t.Fatalf("embedding dimension contract changed: %d", identity.EmbeddingDim)
	}

	// Wrong-dimension embeddings are rejected per contract.
	bad := make([]float64, 32)
	code, _, err := s.do("identity", "POST", "/v1/identity/verify", map[string]any{
		"embedding": map[string]any{"identity_id": "contract-test", "vector": bad},
	})
	if err != nil {
		t.Fatalf("verify call errored: %v", err)
	}
	if code == 200 {
		t.Fatalf("provider accepted %d-dim embedding; contract requires 768", len(bad))
	}
}

// TestContractReservoirTrainResponse: the reservoir training pipeline
// consumers (metacog validation, ontogenesis feedback) rely on the
// TrainSample response surface: prediction_error, samples_trained,
// checkpoint version.
func TestContractReservoirTrainResponse(t *testing.T) {
	s := newStack(t)

	out := s.mustPost("reservoir", "/v1/reservoir/train", map[string]any{
		"input":  []float64{0.1, 0.2},
		"target": make([]float64, 16),
	})
	for _, field := range []string{"prediction_error", "samples_trained", "checkpoint"} {
		if _, ok := out[field]; !ok {
			t.Fatalf("train response missing contract field %q: %v", field, out)
		}
	}

	// Contract: mismatched target dimension is a client error, never a 500.
	code, _, err := s.do("reservoir", "POST", "/v1/reservoir/train", map[string]any{
		"input":  []float64{0.1},
		"target": []float64{0.1},
	})
	if err != nil {
		t.Fatalf("train call errored: %v", err)
	}
	if code == 500 {
		t.Fatalf("provider returned 500 for contract violation, want 4xx")
	}
}

// TestContractCognitiveMetricsShape: the orchestrator's thread-pool
// controller consumes CognitiveMetrics from every service (ecco9.common.v1);
// the SDK type must round-trip the fields the scaling signal needs.
func TestContractCognitiveMetricsShape(t *testing.T) {
	m := contracts.CognitiveMetrics{
		ServiceName:                "ecco9-reservoir",
		ThoughtQueueDepth:          3,
		MemoryConsolidationBacklog: 12,
		IdentityCoherenceDelta:     0.02,
		ReservoirEchoProperty:      0.95,
	}
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back contracts.CognitiveMetrics
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back != m {
		t.Fatalf("CognitiveMetrics round-trip mismatch: %+v ≠ %+v", back, m)
	}
}
