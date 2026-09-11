# ecco9-echodream

EchoDream service — the **consolidation core** of the ecco9 higher cognitive
layer: dream-cycle memory consolidation and wisdom extraction during rest.

Adapted from the ecco9 monorepo:

- **Dream cycle** (`echodream/dream.go`, from `core/echodream/echodream.go`
  and `dream_cycle_integration.go`) — four-phase cycles: collection
  (importance-ordered gathering under `max_memories`), consolidation
  (similarity grouping by dominant terms, merging groups of 2+ memories
  into knowledge with confidence from group size and mean importance),
  wisdom extraction (recurring themes become insights; >5 consolidations
  yield meta-cognitive wisdom), and integration (dream narrative).
- **Consolidation algorithms** (from
  `core/echodream/consolidation_algorithms.go`) — importance weighting and
  pattern-based grouping adapted from the hypergraph/tensor consolidators.

## Contracts

Implements `ecco9.echodream.v1.EchoDreamService` from `ecco9-proto`:

| RPC | REST route |
|-----|------------|
| `StartDreamCycle` | `POST /v1/echodream/cycles` |
| `GetDreamState` | `GET /v1/echodream/state` |
| `SubmitMemory` | `POST /v1/echodream/memories` |

Auxiliary: `GET /v1/echodream/wisdom`, `GET /v1/echodream/knowledge`,
`GET /v1/echodream/dreams`, `GET /healthz`.

## Running

```
go build ./...
ECCO9_ECHODREAM_ADDR=:8092 ./echodreamd
```

## Deployment

StatefulSet with 2 replicas (`deploy/statefulset.yaml`) — consolidated
knowledge and wisdom are durable artifacts — with HPA on the
`ecco9_cognitive_load` custom metric and pod affinity to the memory service.

## Consolidation pipeline

```
memory (episodic) → echodream (consolidate) → wisdom → wisdom service
                              ↓
                    identity (coherence update)
```
