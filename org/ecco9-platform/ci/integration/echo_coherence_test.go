//go:build integration

package integration

import (
	"testing"

	"github.com/dtechorg/ecco9-identity/identity"
	identityserver "github.com/dtechorg/ecco9-identity/server"
)

// TestEchoCoherenceDuringDeployment validates that identity coherence remains
// above the rollout threshold while a service is (re)deployed into the
// ephemeral namespace — the automated check every pipeline performs before
// promoting a rollout (issue #7, section 6.2).
func TestEchoCoherenceDuringDeployment(t *testing.T) {
	s := newStack(t)

	// Baseline: genesis identity verifies at perfect coherence.
	if c := s.coherence("deep-tree-echo"); c < coherenceThreshold {
		t.Fatalf("baseline coherence %v < %v", c, coherenceThreshold)
	}

	// Simulate a deployment: replace the identity service (pod) while traffic
	// continues. The identity must survive the rollout — a new instance of the
	// same service re-derives the same genesis embedding from the same kernel
	// seed, so coherence is preserved across the cutover.
	before := s.coherence("deep-tree-echo")

	s.services["identity"].Close()
	s.Identity = identityserver.New("", "deep-tree-echo")
	s.mount("identity", s.Identity.Routes())

	after := s.coherence("deep-tree-echo")
	if after < coherenceThreshold {
		// This is the rollback condition the Argo identity-coherence-gate
		// implements: coherence dropped below threshold after deployment.
		t.Fatalf("coherence after deployment %v < %v — automatic rollback triggered", after, coherenceThreshold)
	}
	if after < before-0.1 {
		t.Fatalf("coherence delta too large across deployment: before=%v after=%v", before, after)
	}
}

// TestEchoCoherenceRejectsForeignIdentity ensures the coherence gate catches
// identity regressions: a drifted/foreign embedding must score below
// threshold, i.e. the check actually discriminates.
func TestEchoCoherenceRejectsForeignIdentity(t *testing.T) {
	s := newStack(t)

	genuine := s.coherence("deep-tree-echo")
	if genuine < coherenceThreshold {
		t.Fatalf("genuine coherence %v < %v", genuine, coherenceThreshold)
	}

	// A different kernel seed produces a decorrelated embedding.
	foreign := identity.Embed("impostor/kernel")
	out := s.mustPost("identity", "/v1/identity/similarity", map[string]any{
		"vector": foreign,
		"top_k":  1,
	})
	if matches, _ := out["identity_ids"].([]any); len(matches) > 0 {
		scores, _ := out["scores"].([]any)
		if len(scores) > 0 {
			if sim, _ := scores[0].(float64); sim >= coherenceThreshold {
				t.Fatalf("foreign identity similarity %v >= %v — gate would not catch regression", sim, coherenceThreshold)
			}
		}
	}
}
