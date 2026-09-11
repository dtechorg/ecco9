# ecco9-echobeats

EchoBeats service — the **12-step 3-phase cognitive rhythm** of the ecco9
architecture. A leader-elected singleton that emits phase transitions and
step executions to the event mesh.

Extracted from `core/echobeats/`:

- `three_phase_echobeats.go` — `EchoBeatsThreePhase` 12-step loop with three
  concurrent `SimpleInferenceEngine`s (Expressive-Reflective, Perception-Action,
  Learning-Integration).
- `cognitive_loop.go` — step sequencing: 7 expressive steps + 5 reflective steps.
- `twelvestep.go` — pivotal relevance-realization steps (1, 7), affordance
  interaction steps (2-6), virtual salience simulation steps (8-12).
- `scheduler.go` — goal-directed priority-queue event scheduler and
  wake/rest cycle management.

## Loop structure

| Steps | Type | Phase |
|-------|------|-------|
| 1, 7 | Pivotal relevance realization (orienting present commitment) | EXPRESSIVE |
| 2-6 | Actual affordance interaction (conditioning past performance) | EXPRESSIVE |
| 8-12 | Virtual salience simulation (anticipating future potential) | REFLECTIVE |

Every step execution is emitted to subscribers (`StreamSteps`) with its
step number, phase, generated thought, and relevance score.

## Contracts

Implements `ecco9.echobeats.v1.EchoBeatsService` from `ecco9-proto`:

| RPC | REST route |
|-----|-----------|
| GetLoopState | `GET /v1/echobeats/loop` |
| StartLoop | `POST /v1/echobeats/loop/start` (`{"cycle_frequency_hz": 1.0}`) |
| StopLoop | `POST /v1/echobeats/loop/stop` |
| StreamSteps | `GET /v1/echobeats/steps` (Server-Sent Events) |

Additional observability: `GET /v1/echobeats/metrics` (per-engine task
counts, scheduler queue depth) and `GET /healthz` (status, identity
coherence, cognitive load).

## Running

```
ECCO9_ECHOBEATS_ADDR=:8085 go run ./cmd/echobeatsd
```

## Deployment

Deployment with 2 replicas in namespace `ecco9-core` (`deploy/deployment.yaml`).
The loop is leader-elected: only one pod runs the cognitive loop at a time;
standby replicas take over on failover. HPA scales 2-6 pods on the
`ecco9_cognitive_load` custom metric (average 0.6), not CPU — the orchestrator
pushes `cycle_frequency_hz` under load control as replicas join.
