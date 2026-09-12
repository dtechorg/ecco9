# ecco9-proto

Shared Protocol Buffer contracts for the ecco9 distributed cognitive architecture.

This repository is the **single source of truth** for all inter-service contracts
in the org-wide deployment of Deep Tree Echo. Every microservice repository
(`ecco9-reservoir`, `ecco9-memory`, `ecco9-echobeats`, ...) generates its client
and server stubs from these definitions.

## Layout

```
proto/
  ecco9/common/v1/     health, metrics, errors, versioning
  ecco9/reservoir/v1/  Echo State Reservoir (online training, persona config)
  ecco9/memory/v1/     hypergraph memory, persistence
  ecco9/emotion/v1/    embodied emotion, AAR core
  ecco9/identity/v1/   identity embeddings, persona mgmt, theory of mind
  ecco9/echobeats/v1/  12-step 3-phase cognitive loop
  ecco9/consciousness/v1/ stream of consciousness layers
  ecco9/atomspace/v1/  OpenCog hypergraph knowledge representation
  ecco9/relevance/v1/  relevance realization scoring
  ecco9/wisdom/v1/     wisdom metrics, goal orchestration
  ecco9/metacog/v1/    metacognitive monitoring, self-directed learning
  ecco9/ontogenesis/v1/ genome evolution, entelechy actualization
  ecco9/echodream/v1/  dream-cycle memory consolidation
  ecco9/llm/v1/        multi-provider LLM gateway, runner
  ecco9/models/v1/     model registry and conversion
  ecco9/orchestrator/v1/ central orchestration, thread pool control
  ecco9/events/v1/     CloudEvents-compatible event envelopes
```

## Versioning

All packages are versioned (`v1`, `v2`, ...). Breaking changes require a new
major version package. Services must support `N` and `N-1` simultaneously during
rolling upgrades.

## Generating stubs

```sh
buf generate
```

## Design decisions

1. gRPC for synchronous cognition (inference, state queries).
2. NATS JetStream for asynchronous events (memory consolidation, dreams, learning).
3. Identity coherence is a first-class metric exposed by every service.
4. Cognitive load (not CPU) is the autoscaling signal.
