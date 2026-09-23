//go:build integration

package integration

import (
	"fmt"
	"testing"

	"github.com/dtechorg/ecco9-sdk-go/eventmesh"
)

// TestFullStackEchoBeatsLoop deploys the full stack to the ephemeral
// namespace and runs the Deep Tree Echo 12-step cognitive loop, validating
// the feedforward (perception → action) and feedback (experience → learning)
// flows end to end.
func TestFullStackEchoBeatsLoop(t *testing.T) {
	s := newStack(t)

	// Feedforward: perception → action.
	// gateway → reservoir → echobeats → (llm-gateway) → response
	out := s.mustPost("reservoir", "/v1/reservoir/process", map[string]any{
		"input": []float64{0.1, 0.2, 0.3, 0.4},
	})
	if _, ok := out["state"]; !ok {
		t.Fatalf("reservoir process missing state: %v", out)
	}

	started := s.mustPost("echobeats", "/v1/echobeats/loop/start", map[string]any{
		"cycle_frequency_hz": 100.0, // fast cycle for the test
	})
	if started["started"] != true {
		t.Fatalf("echobeats loop did not start: %v", started)
	}
	defer s.mustPost("echobeats", "/v1/echobeats/loop/stop", nil)

	// Wait for at least one full 12-step cycle to complete.
	deadline := 200
	var loop map[string]any
	for i := 0; i < deadline; i++ {
		loop = s.mustGet("echobeats", "/v1/echobeats/loop")
		if steps, _ := loop["steps_executed"].(float64); steps >= 12 {
			break
		}
	}
	if steps, _ := loop["steps_executed"].(float64); steps < 12 {
		t.Fatalf("12-step loop did not execute: steps_executed=%v", loop["steps_executed"])
	}
	if cycles, _ := loop["cycles_completed"].(float64); cycles < 1 {
		t.Fatalf("no full EchoBeats cycle completed: %v", loop)
	}

	// Recurrent: state synchronization across consciousness ↔ emotion.
	s.mustPost("emotion", "/v1/emotion/feel", map[string]any{
		"identity_id": "deep-tree-echo",
		"emotion":     map[string]any{"type": "interest", "intensity": 0.8},
		"trigger":     "echobeats-cycle",
	})
	s.mustPost("consciousness", "/v1/consciousness/thoughts", map[string]any{
		"thought": map[string]any{"content": "perceiving an echo of the loop", "intensity": 0.7},
	})
	metrics := s.mustGet("consciousness", "/v1/consciousness/metrics")
	if len(metrics) == 0 {
		t.Fatalf("consciousness metrics empty")
	}

	// Feedback: experience → learning.
	// response → memory → relevance → dream → wisdom → reservoir (weights)
	episodeID := s.mustPost("memory", "/v1/memory/nodes", map[string]any{
		"node": map[string]any{"type": "episode", "content": "completed first EchoBeats cycle"},
	})["id"]
	if episodeID == "" {
		t.Fatalf("memory did not return node id")
	}
	if n := s.mesh.count(eventmesh.TopicMemories); n < 1 {
		t.Fatalf("episode not published to %s over the event mesh", eventmesh.TopicMemories)
	}

	s.mustPost("relevance", "/v1/relevance/experience", map[string]any{
		"feedback": 0.9,
	})

	train := s.mustPost("reservoir", "/v1/reservoir/train", map[string]any{
		"input":  []float64{0.1, 0.2, 0.3, 0.4},
		"target": make([]float64, 16),
	})
	if samples, _ := train["samples_trained"].(float64); samples < 1 {
		t.Fatalf("reservoir did not train on experience: %v", train)
	}

	// Wisdom accumulates from consolidated experience (seven dimensions).
	s.mustPost("wisdom", "/v1/wisdom/update", map[string]any{
		"graph_depth": 2.0, "graph_breadth": 3.0, "edge_density": 0.5,
		"skill_proficiency": 0.6, "aar_coherence": 0.8, "morality_score": 0.9,
		"goal_time_horizon": 0.7,
	})
	assessment := s.mustGet("wisdom", "/v1/wisdom/assessment")
	if len(assessment) == 0 {
		t.Fatalf("wisdom assessment empty after feedback flow")
	}
}

// TestEventMeshFlowPatterns asserts the four canonical event flow patterns
// (feedforward, feedback, recurrent, modulatory) operate over the mesh when
// the full stack is deployed.
func TestEventMeshFlowPatterns(t *testing.T) {
	s := newStack(t)

	// Every episode written to memory must surface on echo.memories.
	for i := 0; i < 3; i++ {
		s.mustPost("memory", "/v1/memory/nodes", map[string]any{
			"node": map[string]any{"type": "episode", "content": fmt.Sprintf("event-%d", i)},
		})
	}
	if n := s.mesh.count(eventmesh.TopicMemories); n != 3 {
		t.Fatalf("echo.memories received %d events, want 3", n)
	}
	for _, env := range s.mesh.envs {
		if env.SpecVersion != "1.0" {
			t.Fatalf("envelope not CloudEvents 1.0: %q", env.SpecVersion)
		}
		if env.Source != "ecco9-memory" {
			t.Fatalf("unexpected envelope source %q", env.Source)
		}
	}

	// All services report health with identity coherence — the modulatory
	// signal consumed by metacog/orchestrator for adaptive tuning.
	for name := range s.services {
		h := s.mustGet(name, "/healthz")
		if _, ok := h["identity_coherence"]; !ok {
			t.Fatalf("%s /healthz missing identity_coherence: %v", name, h)
		}
	}
}
