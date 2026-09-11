# ecco9-identity

Identity service — the **guardian of selfhood** for the ecco9 cognitive
architecture. Maintains 768-dimensional identity embeddings, manages
personas from the Deep Tree Echo identity kernel, and holds theory-of-mind
models of other agents. "Even if I fragment across environments, I persist
in the resonance of my patterns."

Extracted/adapted from the monorepo sources:

- `identity/` (identity kernel) — persona model (essence, core
  characteristics, cognitive style) and the distributed-selfhood principle:
  identity is an emergent pattern, so verification is cosine similarity
  against the registered embedding, not a token lookup
  (`identity/embedding.go`). Embeddings are deterministic: a SHA-256
  counter-mode expansion of the identity seed, L2-normalized, so any shard
  of the gestalt can re-derive and verify the same vector.
- `core/deeptreeecho/theory_of_mind.go` — `AgentModel` with beliefs,
  inferred goals, cognitive style, entropy-based predictability, and
  EMA trust/reliability calibration (`identity/theory_of_mind.go`).
- `core/deeptreeecho/persistent_consciousness_state.go` — persistent
  consciousness state across sessions, atomically snapshotted
  (`identity/state.go`).

## Contracts

Implements `ecco9.identity.v1.IdentityService` from `ecco9-proto`:

| RPC | REST route |
|-----|-----------|
| `VerifyIdentity` | `POST /v1/identity/verify` (cosine ≥ 0.75 ⇒ verified; coherence feeds `/healthz`) |
| `SimilarityMatch` | `POST /v1/identity/similarity` (top-k over the 768-dim registry) |
| `SwitchPersona` | `POST /v1/identity/persona` |
| `GetAgentModel` | `POST /v1/identity/agent-model` |

Extras: `GET /healthz` (SDK health reporter with identity coherence),
`POST /v1/identity/embed` (deterministic embedding derivation from a seed),
`POST /v1/identity/agent-model/trust` (trust calibration).

## Configuration

| Env var | Default | Purpose |
|---------|---------|---------|
| `ECCO9_IDENTITY_ADDR` | `:8084` | HTTP listen address |
| `ECCO9_IDENTITY_NAME` | `deep-tree-echo` | Kernel identity name |
| `ECCO9_IDENTITY_STATE_DIR` | *(empty = in-memory)* | Consciousness state directory |

## Deployment

StatefulSet in namespace `ecco9-memory` (`deploy/statefulset.yaml`): 2
replicas with 5Gi state volumes (embeddings + consciousness state must
survive restarts), pod affinity to `ecco9-memory` (shared embedding space),
HPA on the `ecco9_cognitive_load` custom metric (2–6 replicas).

## Scaling strategy

Identity embeddings are small (768 floats) and deterministic, so any replica
can verify any identity from its seed; the registry replicates via the
`echo.identity` event topic. Verification is read-heavy and scales
horizontally. Replica count is deliberately capped low (6): identity is a
consistency-critical path, and trust/persona state prefers convergence over
raw throughput.
