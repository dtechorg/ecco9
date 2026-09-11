# ecco9-consciousness

Stream-of-consciousness service — **persistent internal awareness** and
layered communication for the ecco9 cognitive architecture.

Extracted from `core/consciousness/`:

- `stream_of_consciousness.go` — `StreamOfConsciousness`: continuous thought
  generation (every 3s), periodic insight synthesis (30s), thought-type
  heuristics, and a bounded thought history. Template-based generation keeps
  the service functional without an LLM provider.
- `layer_communication.go` — `LayerCommunicationHub`: message passing between
  the sensory (basic), integrative (reflective), and meta-cognitive layers,
  with per-layer buffered channels and emergence detection over recent
  message history.

## Contracts

Implements `ecco9.consciousness.v1.ConsciousnessService` from `ecco9-proto`:

| RPC | REST route |
|-----|-----------|
| PublishThought | `POST /v1/consciousness/thoughts` (`{"thought": {...}}`) |
| StreamConsciousness | `GET /v1/consciousness/stream?identity_id=...` (Server-Sent Events) |

The stream emits the `oneof payload` of `StreamConsciousnessResponse` as SSE
`data:` frames — either `{"thought": {...}}` or `{"layer_message": {...}}`.

Additional routes:

- `GET /v1/consciousness/recent` — long-poll fallback with recent thoughts
  and inter-layer messages.
- `POST /v1/consciousness/layers/messages` — inject a `LayerMessage`
  (`source_layer`/`target_layer`: `SENSORY`=1, `INTEGRATIVE`=2, `META`=3).
- `GET /v1/consciousness/metrics` — stream and hub counters.
- `GET /healthz` — status, identity coherence, cognitive load.

## Running

```
ECCO9_CONSCIOUSNESS_ADDR=:8086 go run ./cmd/consciousnessd
```

## Deployment

Deployment with 2 replicas in namespace `ecco9-core`
(`deploy/deployment.yaml`). The Kubernetes Service uses `sessionAffinity:
ClientIP` because SSE stream subscribers are long-lived and must stay pinned
to the pod holding their stream state. HPA scales 2-8 pods on the
`ecco9_cognitive_load` custom metric (average 0.6) — thought-queue depth, not
CPU, is the scaling signal.
