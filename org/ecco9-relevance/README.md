# ecco9-relevance

Relevance Realization Ennead service — the **salience core** of the ecco9
higher cognitive layer. Scores events for relevance, salience, and affordance
through Vervaeke's triad-of-triads framework.

Adapted from `core/relevance/` in the ecco9 monorepo:

- **Knowing triad** (`relevance/engine.go`) — propositional, procedural,
  perspectival, and participatory ways of knowing, with per-way learning
  rates and gnostic integration.
- **Understanding triad** — nomological, normative, and narrative orders,
  integrated by geometric mean (all three required for meaning).
- **Wisdom triad** — morality, meaning, and mastery, integrated into
  eudaimonia (flourishing) by a weighted geometric mean.
- **Realization process** — cross-triad weighted relevance, salience
  landscape modulation (underdeveloped dimensions boosted), and sophrosyne
  (variance-penalty optimization around the 0.05 optimum).

## Contracts

Implements `ecco9.relevance.v1.RelevanceService` from `ecco9-proto`:

| RPC | REST route |
|-----|------------|
| `Score` | `POST /v1/relevance/score` |
| `ScoreStream` | `POST /v1/relevance/score_stream` (batched) |

Auxiliary: `GET /v1/relevance/status`, `POST /v1/relevance/experience`
(feedback learning), `GET /healthz` (identity coherence + cognitive load).

## Running

```
go build ./...
ECCO9_RELEVANCE_ADDR=:8088 ./relevanced
```

## Deployment

Deployment with 2 replicas (`deploy/deployment.yaml`), HPA on the
`ecco9_cognitive_load` custom metric, pod affinity to the event mesh for
low-latency event scoring.

## Scoring pipeline

```
eventmesh → relevance (score/filter) → echo.training → reservoir (RLS)
                                                        ↓
                                              metacog (validation)
```
