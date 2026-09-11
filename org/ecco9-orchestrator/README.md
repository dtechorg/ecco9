# ecco9-orchestrator

The **central nervous system** of the ecco9 distributed cognitive architecture.

## Responsibilities

- **Agent/Task/Workflow lifecycle** via Kubernetes CRDs (`CognitiveAgent`,
  `CognitiveTask`, `CognitiveWorkflow`) — see `deploy/crds/crds.yaml`.
- **Echo state thread pool controller** (`orchestrator/threadpool.go`):
  computes the global cognitive load index from every service's reported
  metrics and emits adaptive `ThreadPoolDirective`s that set reservoir worker
  counts, EchoBeats cadence, and runner parallelism. This is a homeostatic
  control loop maintaining optimal cognitive throughput.

## Contracts

Implements `ecco9.orchestrator.v1.OrchestratorService` from `ecco9-proto`.

## Homeostatic control law

```
global_load = LoadIndex(all service metrics)

reservoir_workers = scale(load, 2, 64)
echobeats_hz      = 0.5 + 1.5 * load
runner_workers    = scale(load, 1, 16)
```

Directives are recomputed no more often than every 5s (hysteresis) to avoid
oscillation, then pushed to services on `echo.directives` and applied via the
`ReservoirControlService.SetThreadPoolSize` RPC.

## Deployment

Leader-elected active-passive pair (3 replicas) in `ecco9-platform`
(`deploy/deployment.yaml`).
