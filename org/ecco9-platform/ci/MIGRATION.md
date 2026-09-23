# Monorepo → Multi-Repo Migration Strategy (Phase 6.3)

The extraction of `org/ecco9-*` services from the `EchoCog/echollama`
monorepo proceeds in four phases. At every phase the monorepo keeps working:
extracted services publish Go modules that the monorepo imports, so callers
migrate gradually instead of flag-day.

## Phase A — Shared infrastructure first

Stand up the repositories every service depends on, while the monorepo
continues unchanged:

| Order | Repo | Contents | Effort |
|-------|------|----------|--------|
| 1 | `ecco9-proto` | Protobuf contracts + `buf` codegen config | Small |
| 2 | `ecco9-platform` | Terraform, Helm chart library, service mesh, observability, **this CI** | Medium |
| 3 | `ecco9-eventmesh` | NATS JetStream topology, streams, DLQ policy | Small |
| 4 | `ecco9-sdk-go` | Hand-written contract types mirroring proto, health reporter, middleware | Medium |

Done when: proto lints clean (`buf lint`, `buf breaking`), SDK round-trips
all contract types, the event mesh accepts the topic taxonomy, and this
repository's `org-ci` matrix is green.

## Phase B — Extract services leaf-first

Extract one service at a time, starting with repos that have **no internal
dependencies** on other cognitive services. Extraction order (from the
dependency table in issue #1):

```
ecco9-reservoir → ecco9-memory → ecco9-emotion → ecco9-identity →
ecco9-atomspace → ecco9-relevance → ecco9-consciousness → ecco9-echobeats →
ecco9-echodream → ecco9-wisdom → ecco9-ontogenesis → ecco9-metacog →
ecco9-llm-gateway → ecco9-runner → ecco9-models → ecco9-orchestrator →
ecco9-gateway → ecco9-cli
```

Each extracted repo:

1. Gets `ci/pipeline-template.yaml` installed (lint → vet → test → build →
   contract tests → image → canary deploy with identity-coherence gate).
2. Publishes `github.com/dtechorg/ecco9-<service>` Go module versions.
3. Keeps its REST/gRPC surface identical to the monorepo package it
   replaces — verified by the contract tests in `ci/integration/`.

## Phase C — Monorepo imports extracted modules (transitional)

The monorepo's `org/<service>/` packages become thin re-export shims over
the published modules, or are replaced outright by module requirements:

```go
// In the monorepo go.mod during transition:
require github.com/dtechorg/ecco9-memory v0.3.0
```

Inside this mirror repository, org modules already reference each other via
relative `replace` directives (`replace github.com/dtechorg/ecco9-sdk-go =>
../ecco9-sdk-go`); extracted repos pin released versions instead.
`.github/workflows/org-services-ci.yaml` keeps validating the mirrored
subtree — path-filtered per service, with shared-infra changes (sdk, proto,
eventmesh) fanning out to every dependent module.

## Phase D — Deprecate monorepo packages at production parity

A monorepo package is deprecated when its extracted service shows:

1. **Test parity** — the package's tests run in the extracted repo's
   pipeline (moved, not duplicated).
2. **Contract parity** — `ci/integration` contract tests pass against the
   extracted service.
3. **Coherence parity** — canary deployments pass the
   `identity-coherence-gate` analysis (≥ 0.7, see
   `servicemesh/rollout-analysis.yaml`) for a full soak window.
4. **Import migration** — no monorepo package imports the old path anymore
   (enforced by a `go list -deps` check in CI).

Deprecation mechanics: mark the monorepo package with a `Deprecated:` doc
comment, log a warning on use, and delete after one release cycle.

## Rollback

Every phase is independently reversible:

- Phase A repos are additive; deleting them changes nothing in the monorepo.
- Phase B extractions are validated by contract tests before the monorepo
  imports them.
- Phase C imports pin exact versions; reverting is a `go.mod` change.
- Deployment-level regressions roll back automatically via the identity
  coherence gate.
