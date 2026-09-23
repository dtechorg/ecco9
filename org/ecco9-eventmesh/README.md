# ecco9-eventmesh

NATS JetStream event mesh: the central nervous system for inter-service
communication in the ecco9 distributed cognitive architecture.

## Topic taxonomy

Topics mirror cognitive pathways. All payloads are `ecco9.events.v1.EventEnvelope`
serialized protobuf with `data_content_type = "application/protobuf"`.

| Topic | Producer | Consumers | Purpose |
|-------|----------|-----------|---------|
| `echo.thoughts` | consciousness, echobeats | memory, relevance, metacog | Thought stream |
| `echo.emotions` | emotion | consciousness, reservoir, memory | Affective state |
| `echo.memories` | memory, gateway | dream, relevance, reservoir | Episodic memory collection |
| `echo.dreams` | echodream | memory, wisdom, metacog | Consolidated knowledge |
| `echo.resonance` | reservoir | emotion, consciousness, identity | Reservoir state echoes |
| `echo.training` | relevance | reservoir | Filtered training samples |
| `echo.metrics` | all services | orchestrator, metacog | Cognitive load telemetry |
| `echo.directives` | orchestrator | all services | Adaptive control directives |

## Streams (JetStream)

- `ECHO-CORE`: `echo.thoughts`, `echo.emotions`, `echo.resonance` — limits retention, 24h
- `ECHO-MEMORY`: `echo.memories`, `echo.dreams` — interest retention, replayable for training
- `ECHO-CONTROL`: `echo.metrics`, `echo.directives`, `echo.training` — work-queue retention
- `ECHO-DLQ`: `<topic>.dlq` dead letter subjects — retained 30d for postmortem replay

## Retry policy and dead letter queues

Every canonical consumer (provisioned in `streams/streams.yaml`) uses
explicit acks with **max 5 deliveries** and a linear backoff (5s → 5m).
When a message exhausts its deliveries, JetStream emits a
`$JS.EVENT.ADVISORY.CONSUMER.MAX_DELIVERIES.<stream>.<consumer>` advisory;
the `ecco9-metacog` DLQ bridge republishes the offending envelope onto the
matching `ECHO-DLQ` subject (`<topic>.dlq`) and raises an anomaly signal.

## Event replay

`ECHO-MEMORY` retains events for 7 days, so dream-cycle consolidation and
reservoir training can be re-run by creating a new durable consumer with
`--deliver all --replay original`. `ECHO-DLQ` retains for 30 days, allowing
failed events to be replayed after the consuming service is repaired.

## Event flow patterns

The mesh mirrors the cognitive architecture through four canonical flows:

- **Feedforward (perception → action):**
  `gateway → reservoir → echobeats → llm-gateway → response`
- **Feedback (experience → learning):**
  `response → memory → relevance → dream → wisdom → reservoir (weight update)`
- **Recurrent (state maintenance):**
  `reservoir ↔ consciousness ↔ emotion` (continuous state synchronization)
- **Modulatory (meta-control):**
  `metacog → orchestrator → all services` (adaptive parameter tuning)

### Online reservoir training pipeline

The feedback flow is realized as a continuous online learning pipeline over
the `ECHO-MEMORY` and `ECHO-CONTROL` streams:

1. **Collection** — `ecco9-memory` publishes episodic memories to `echo.memories`.
2. **Preprocessing** — `ecco9-relevance` (`relevance-filter` consumer) scores and
   filters memories for training relevance, republishing to `echo.training`.
3. **Training** — `ecco9-reservoir` (`reservoir-trainer` consumer) consumes
   relevant memories and updates readout weights via RLS.
4. **Validation** — `ecco9-metacog` evaluates training quality via prediction
   error on a held-out set.
5. **Deployment** — updated reservoir parameters are versioned and hot-reloaded
   without downtime.
6. **Feedback** — training metrics feed into `ecco9-ontogenesis` for
   evolutionary parameter search.

## Deployment

See `nats/values.yaml` for the Helm values (NATS cluster mode with JetStream,
3 replicas, persistent volumes). Streams and consumers are provisioned by
the idempotent Job in `streams/streams.yaml`.
