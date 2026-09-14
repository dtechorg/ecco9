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

## Deployment

See `nats/values.yaml` for the Helm values (NATS cluster mode with JetStream,
3 replicas, persistent volumes). Streams and consumers are provisioned by
the idempotent Job in `streams/streams.yaml`.
