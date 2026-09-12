# ecco9-emotion

Embodied emotion service — the **affective core** of the ecco9 cognitive
architecture. Emotions here are constitutive of knowing, not decorative:
each discrete emotion carries cognitive effects (attention scope, processing
depth, approach/avoidance, memory strength, exploration bias) that modulate
reservoir dynamics, memory encoding, and goal orchestration.

Extracted/adapted from the monorepo sources:

- `core/deeptreeecho/embodied_emotion.go` — Izard's Differential Emotion
  Theory: 10 discrete emotions with per-type cognitive effect profiles,
  exponential decay toward baseline, arousal/valence dimensional affect, and
  dominant-emotion blending (`emotion/emotion.go`).
- `core/deeptreeecho/aar_core.go` — Agent-Arena-Relation geometric
  self-awareness. The Agent (urge-to-act) is driven by arousal; the Arena
  (need-to-be) holds Wisdom/Curiosity/Balance attractors whose pull is
  modulated by valence; the Relation (emergent self) arises from their
  tension, yielding coherence, stability, and awareness metrics
  (`emotion/aar.go`). The AAR coherence feeds the SDK health reporter as
  the identity-coherence signal.

## Contracts

Implements `ecco9.emotion.v1.EmotionService` from `ecco9-proto`:

| RPC | REST route |
|-----|-----------|
| `Feel` | `POST /v1/emotion/feel` |
| `GetEmotionalState` | `POST /v1/emotion/state` (includes arousal/valence, blend, cognitive effects, AAR metrics) |
| `StreamEmotions` | `POST /v1/emotion/stream` (NDJSON chunk per second until disconnect) |

Extras: `GET /healthz` (SDK health reporter with AAR coherence),
`POST /v1/emotion/aar/goal` (register a goal with the Agent component).

## Configuration

| Env var | Default | Purpose |
|---------|---------|---------|
| `ECCO9_EMOTION_ADDR` | `:8083` | HTTP listen address |

## Deployment

Deployment in namespace `ecco9-core` (`deploy/deployment.yaml`): 2 replicas,
pod affinity to `ecco9-reservoir` (emotional effects modulate temporal
processing on the low-latency path), HPA on the `ecco9_cognitive_load`
custom metric (2–8 replicas).

## Scaling strategy

Emotion systems are keyed by `identity_id`; replicas shard identities by
consistent hashing at the gateway. Affective state is short-lived (decays to
baseline within seconds), so replicas are interchangeable — the AAR core
re-converges in under a second after rescheduling, making the service
stateless enough for a plain Deployment.
