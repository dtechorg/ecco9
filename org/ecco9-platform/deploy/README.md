# Phase 7: Deployment Topology at Scale

Production Kubernetes layout and multi-region strategy for the ecco9
distributed cognitive architecture (issue #8). This directory is the
deployable counterpart to the platform foundation (terraform, helm,
servicemesh, observability) and the Phase 6 CI/CD pipelines in `../ci/`.

## 7.1 Production Kubernetes layout

| Namespace | Services | Replicas | Resources |
|-----------|----------|----------|-----------|
| `ecco9-core` | reservoir, echobeats, emotion, consciousness | 2-4 each | 2 CPU, 4Gi |
| `ecco9-memory` | memory, atomspace, identity | 2-3 each | 4 CPU, 8Gi |
| `ecco9-inference` | llm-gateway, runner | 2-8 | GPU nodes |
| `ecco9-higher` | wisdom, metacog, echodream, ontogenesis | 1-2 each | 1 CPU, 2Gi |
| `ecco9-platform` | gateway, orchestrator | 3+ each | 2 CPU, 4Gi |
| `ecco9-data` | PostgreSQL, Redis, NATS | HA config | Managed |

The layout is encoded as Kustomize bases:

- `namespaces/namespaces.yaml` — the six namespaces, labeled with their
  cognitive tier and with strict-mTLS / injection policy.
- `topology.yaml` — one HelmChartInflationGenerator entry per service,
  rendered from the shared `helm/ecco9-service` library chart with the
  replica range, resource envelope, and tier placement from the table.
- `kustomization.yaml` — composes namespaces + topology into the
  single-region base. Region overlays live under `multiregion/`.

Render the base:

```sh
kustomize build --enable-helm org/ecco9-platform/deploy
```

Per-service `deploy/` manifests in `org/ecco9-*/deploy/` remain the
authoritative workload specs; the topology generator pins scale, placement,
and resources so the production layout stays consistent with the issue #8
table and reviewable in one place.

### Placement rules

- **Reservoir is the temporal core**: `ecco9-reservoir` runs on
  high-memory-bandwidth nodes (`ecco9.io/tier=memory-bandwidth`) and is
  co-located with `ecco9-llm-gateway` for minimal inference pipeline
  latency (pod affinity, mirroring `org/ecco9-reservoir/deploy/statefulset.yaml`).
- **Inference runs on GPU nodes**: `ecco9-runner` (and GPU-backed
  llm-gateway pools) tolerate the `ecco9.io/gpu` taint and schedule onto
  the `gpu-runner-pool` node pool from `../terraform/main.tf`.
- **Cognitive load is the scaling signal**: every HPA targets
  `ecco9_cognitive_load` (thought queue depth, memory consolidation
  backlog, dream cycle freshness, identity coherence delta) via the
  Prometheus adapter — not CPU/memory. See `../helm/ecco9-service/templates/_hpa.yaml`.

## 7.2 Multi-region considerations

`multiregion/` contains the per-region overlay artifacts:

| Concern | Mechanism | Artifact |
|---------|-----------|----------|
| Identity coherence | Cross-region event replication (NATS supercluster over gateway connections) | `multiregion/nats-supercluster.yaml` |
| Reservoir state | Eventual consistency via CRDT-based state merging between regional reservoirs | `multiregion/reservoir-crdt.yaml` |
| Model artifacts | CDN / object-storage replication with regional caching | `multiregion/model-artifacts.yaml` |
| Global orchestrator | Federated with regional orchestrators for latency-sensitive decisions | `multiregion/orchestrator-federation.yaml` |

Regions are independent failure domains: each runs the full 7.1 layout.
Identity, reservoir echoes, and dreams flow between regions over the
supercluster; latency-sensitive orchestration decisions (scaling,
rollbacks) are made by the regional orchestrator, while the global
orchestrator only arbitrates cross-region policy (traffic weight, model
promotion, coherence thresholds).

## Key design decisions (issue #8)

1. **gRPC for synchronous, NATS for asynchronous** — inference and state
   queries use gRPC; memory consolidation, dream cycles, and learning use
   the event mesh (`org/ecco9-eventmesh`).
2. **Reservoir as the temporal core** — dedicated high-bandwidth nodes,
   co-located with the LLM gateway.
3. **Identity coherence as a first-class metric** — every rollout passes
   the `identity-coherence-gate` AnalysisTemplate
   (`../servicemesh/rollout-analysis.yaml`); coherence < 0.7 triggers
   automatic rollback (wired into `../ci/pipeline-template.yaml`).
4. **Monorepo compatibility during transition** — `github.com/EchoCog/echollama`
   keeps working; extracted services publish Go modules the monorepo
   imports (`../ci/MIGRATION.md`).
5. **Cognitive load as the scaling signal** — HPAs scale on
   `ecco9_cognitive_load`, thought queue depth, memory consolidation
   backlog, dream cycle freshness, and identity coherence delta.

## Validation

`topology_test.go` (plain `go test`, no cluster required) parses the
kustomization and topology generator and asserts the 7.1 table is covered
exactly: every service in the right namespace, replica bounds within the
issue ranges, resource envelopes matching, and all four multi-region
artifacts present.
