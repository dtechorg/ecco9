# ecco9-reservoir

Echo State Reservoir service — the **temporal core** of the ecco9 cognitive
architecture.

Extracted from `core/deeptreeecho/echo_state_reservoir.go` and extended with:

- **Online reservoir training** via Recursive Least Squares on readout weights
  (`reservoir/rls.go`). Consumes filtered samples from the `echo.training`
  topic and hot-reloads checkpoint versions without downtime.
- **Echo state thread pool controller** (`reservoir/threadpool.go`). The
  orchestrator pushes target worker counts via
  `ReservoirControlService.SetThreadPoolSize` based on the global cognitive
  load index.

## Contracts

Implements `ecco9.reservoir.v1.ReservoirService` and
`ecco9.reservoir.v1.ReservoirControlService` from `ecco9-proto`.

## Deployment

StatefulSet with 3 sharded partitions (`deploy/statefulset.yaml`), HPA on the
`ecco9_cognitive_load` custom metric, pod affinity to the LLM gateway for
low-latency inference.

## Online training pipeline

```
memory → echo.memories → relevance (filter) → echo.training → reservoir (RLS)
                                                              ↓
                                                    metacog (validation)
                                                              ↓
                                            ontogenesis (evolutionary search)
```
