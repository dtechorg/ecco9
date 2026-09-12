# ecco9-wisdom

Wisdom cultivation service — the **axiological core** of the ecco9 higher
cognitive layer: seven-dimensional wisdom metrics, autonomous goal
orchestration, interest-driven goal generation, and wisdom application.

Adapted from the ecco9 monorepo:

- **Seven-dimensional wisdom** (`wisdom/wisdom.go`, from
  `core/wisdom/seven_dimensional.go`) — knowledge depth/breadth, integration,
  practical application, reflective insight, ethical consideration, and
  temporal perspective; weighted overall (per Echo9 spec), coherence via
  exponential variance decay, and evolution-rate tracking.
- **Goal orchestration** (from `core/goals/goal_orchestrator.go`) — bounded
  active-goal set (max 5), priority-ordered listing, progress/completion
  tracking.
- **Interest-driven generation** (from
  `core/goals/interest_driven_generator.go`) — curiosity landscape seeded
  with Deep Tree Echo topics; composite interest score (strength .4,
  novelty .3, utility .2, recency .1).
- **Wisdom application** (from `core/deeptreeecho/wisdom_application.go`) —
  foundational wisdom base, context matching scored applicability .4 /
  timeliness .2 / specificity .2 / novelty .1 / usefulness .1.

## Contracts

Implements `ecco9.wisdom.v1.WisdomService` from `ecco9-proto`:

| RPC | REST route |
|-----|------------|
| `CreateGoal` | `POST /v1/wisdom/goals` |
| `ListGoals` | `GET /v1/wisdom/goals?type_filter=N` |
| `GetWisdom` | `GET /v1/wisdom/assessment` |
| `ApplyWisdom` | `POST /v1/wisdom/apply` |

Auxiliary: `POST /v1/wisdom/update` (dimension signals),
`POST /v1/wisdom/interest` (engagement), `POST /v1/wisdom/goals/advance`,
`GET /healthz`.

## Running

```
go build ./...
ECCO9_WISDOM_ADDR=:8089 ./wisdomd
```

## Deployment

Deployment with 2 replicas (`deploy/deployment.yaml`), HPA on the
`ecco9_cognitive_load` custom metric, pod affinity to the orchestrator.
