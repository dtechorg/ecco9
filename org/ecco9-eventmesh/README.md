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

## Dead letter queues

Every consumer gets a DLQ subject `<topic>.dlq` with max 5 redeliveries.
DLQ messages are monitored by `ecco9-metacog` as anomaly signals.

## Deployment

See `nats/values.yaml` for the Helm values (NATS cluster mode with JetStream,
3 replicas, persistent volumes).
