# ecco9 CI/CD Pipeline Architecture (Phase 6)

Continuous integration and delivery for the ecco9 distributed cognitive
architecture. Three tiers, mirroring issue #7:

## 6.1 Per-repo pipeline — `pipeline-template.yaml`

Every `org/ecco9-*` repository installs the same pipeline:

```
lint → vet → test (race + coverage) → build → contract tests
     → image build/push → canary deploy → identity-coherence gate
     → automatic rollback
```

`workflows/org-ci.yaml` applies the template's CI stages to every org Go
module (used by extracted repos with a single-entry matrix, and by the
monorepo mirror as a full fan-out).

## 6.2 Cross-repo integration testing — `integration/`

| Suite | What it validates |
|-------|-------------------|
| `cognitive_integration_test.go` | Full stack deployed to an ephemeral namespace; 12-step EchoBeats loop; feedforward/feedback/recurrent/modulatory event flows |
| `contract_test.go` | Consumer-driven (Pact-style) contracts: health surface, CloudEvents envelope, 768-dim identity embeddings, reservoir train response, cognitive metrics shape |
| `chaos_test.go` | Random pod kills, network partitions, latency injection; graceful degradation and full recovery |
| `echo_coherence_test.go` | Identity coherence ≥ 0.7 during deployments; rollback condition discrimination |

Run locally (no containers needed — the stack deploys in-process):

```sh
cd org/ecco9-platform/ci/integration
go test -tags=integration -count=1 -v ./...
```

`docker-compose.integration.yaml` boots the same topology with real images
plus a NATS JetStream container, for smoke-testing images and reproducing
CI failures.

## 6.3 Monorepo → multi-repo migration — `MIGRATION.md`

Four phases (shared infra → leaf-first extraction → transitional monorepo
imports → deprecation at production parity) with per-phase exit criteria and
rollback. During the transition, `.github/workflows/org-services-ci.yaml`
keeps the monorepo `org/` subtree under the same gates.

## Identity coherence as a first-class metric

Every deployment pipeline includes an automated identity-coherence check.
If coherence drops below threshold (default 0.7) after a deployment,
automatic rollback triggers — implemented as the `identity-coherence-gate`
Argo Rollouts AnalysisTemplate in `../servicemesh/rollout-analysis.yaml`
and exercised by `integration/echo_coherence_test.go`.
