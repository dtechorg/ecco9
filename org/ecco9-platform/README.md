# ecco9-platform

Platform deployment foundation for the ecco9 distributed cognitive architecture.

## Contents

- `terraform/` — cluster provisioning (GKE/EKS) with GPU node pools for runners
- `helm/ecco9-service/` — shared Helm chart library used by every cognitive service
- `servicemesh/` — Istio configuration: mTLS, traffic management, canary routing
- `observability/` — Prometheus, Grafana, Tempo, Loki stack plus cognitive dashboards
- `secrets/` — External Secrets Operator integration with Vault/SOPS

## Namespaces

| Namespace | Purpose |
|-----------|---------|
| `ecco9-core` | reservoir, echobeats, emotion, consciousness |
| `ecco9-memory` | memory, atomspace, identity |
| `ecco9-inference` | llm-gateway, runner (GPU) |
| `ecco9-higher` | wisdom, metacog, echodream, ontogenesis |
| `ecco9-platform` | gateway, orchestrator |
| `ecco9-data` | PostgreSQL, Redis, NATS |

## Identity coherence gate

Every deployment pipeline includes an identity-coherence check. If coherence
drops below threshold (default 0.7) after a rollout, Argo Rollouts triggers
an automatic rollback. See `servicemesh/rollout-analysis.yaml`.
