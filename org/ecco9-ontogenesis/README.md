# ecco9-ontogenesis

Ontogenetic evolution service — the **becoming core** of the ecco9 higher
cognitive layer: genome evolution via genetic algorithms and entelechy
(self-actualization) metrics.

Adapted from the ecco9 monorepo:

- **Genome** (`ontogenesis/engine.go`, from `core/ontogenesis/genome.go`) —
  mutable gene maps with Gaussian mutation and uniform crossover; Euclidean
  genetic distance; mutation parameters averaged across parents.
- **Evolution** (from `core/ontogenesis/evolution.go`) — population GA with
  tournament selection (k=3), elitism, and per-generation fitness
  re-evaluation; fitness keeps the `1/(1+sqrt(error))` form of
  `KernelGenome.Evaluate`, measuring RMS deviation from target operating
  points.
- **Kernel operations** (from `core/ontogenesis/operations.go`) — crossover
  and mutation drive self-reproduction; self-generation and
  self-optimization patterns preserved in the evolve loop.
- **Entelechy** (from `core/entelechy/`) — five actualization dimensions
  (ontological, teleological, cognitive, integrative, evolutionary) with the
  weighted fitness and Embryonic→Transcendent levels of
  `EntelechyGenome`, and the actualization differential
  `dA = α·purpose·(1−A) − β·fragmentation` from `ActualizationMetrics`.

## Contracts

Implements `ecco9.ontogenesis.v1.OntogenesisService` from `ecco9-proto`:

| RPC | REST route |
|-----|------------|
| `SubmitGenome` | `POST /v1/ontogenesis/genomes` |
| `EvaluateFitness` | `POST /v1/ontogenesis/fitness` |
| `Evolve` | `POST /v1/ontogenesis/evolve` |
| `GetEvolutionStatus` | `GET /v1/ontogenesis/evolve/{job_id}` |
| `GetActualization` | `GET /v1/ontogenesis/actualization` |

Auxiliary: `POST /v1/ontogenesis/dimensions` (entelechy assessment input),
`GET /v1/ontogenesis/status`, `GET /healthz`.

## Running

```
go build ./...
ECCO9_ONTOGENESIS_ADDR=:8091 ./ontogenesisd
```

## Deployment

StatefulSet with 2 replicas (`deploy/statefulset.yaml`) — genome lineage is
durable evolutionary state — with HPA on the `ecco9_cognitive_load` custom
metric and pod affinity to the reservoir whose kernels evolve here.
