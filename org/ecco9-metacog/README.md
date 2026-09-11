# ecco9-metacog

Metacognitive monitoring service — the **self-awareness core** of the ecco9
higher cognitive layer: process monitoring, decision quality assessment,
strategy selection, and self-directed learning gap identification.

Adapted from the ecco9 monorepo:

- **Metacognitive monitor** (`metacog/monitor.go`, from
  `core/deeptreeecho/metacognitive_monitor.go`) — active process tracking
  with self-model updates on completion, decision logging with four-axis
  quality assessment (appropriateness / timeliness / effectiveness /
  efficiency), and the deliberate / intuitive / analytical strategy
  repertoire scored reliability .3 + success .3 + speed-or-accuracy .4.
- **Self-directed learning** (from
  `core/deeptreeecho/self_directed_learning.go`) — knowledge gaps surfaced
  from low-quality process outcomes and registered for goal orchestration.
- **Recursive self-improvement** (from `core/improvement/recursive.go`) —
  global assessment (cognitive load, identity coherence, recommendations)
  consumed by the orchestrator's adaptation loop.

## Contracts

Implements `ecco9.metacog.v1.MetaCogService` from `ecco9-proto`:

| RPC | REST route |
|-----|------------|
| `Monitor` | `POST /v1/metacog/monitor` |
| `SelectStrategy` | `POST /v1/metacog/strategy` |
| `ListLearningGaps` | `GET /v1/metacog/learning_gaps` |
| `AssessSystem` | `GET /v1/metacog/assessment` |

Auxiliary: `POST /v1/metacog/decisions`, `POST /v1/metacog/decisions/assess`,
`POST /v1/metacog/learning_gaps`, `GET /v1/metacog/status`, `GET /healthz`.

## Running

```
go build ./...
ECCO9_METACOG_ADDR=:8090 ./metacogd
```

## Deployment

Deployment with 2 replicas (`deploy/deployment.yaml`), HPA on the
`ecco9_cognitive_load` custom metric, pod affinity to the orchestrator.
