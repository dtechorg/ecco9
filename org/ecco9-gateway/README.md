# ecco9-gateway

External-facing API gateway for the ecco9 platform. It reverse-proxies the
Ollama-compatible, OpenAI-compatible, and Deep Tree Echo HTTP surfaces to the
backend cognitive services running in the `ecco9-core` namespace.

## Build

```sh
go build ./...
```

Stdlib-only; depends on the local `github.com/dtechorg/ecco9-sdk-go` via a
`replace` directive, so builds work fully offline.

## Run

```sh
ECCO9_GATEWAY_ADDR=:8080 go run ./cmd/gatewayd
```

## Routes

| Path | Backend service |
|------|-----------------|
| `/api/generate`, `/api/chat`, `/api/embed` | ecco9-llm-gateway (:8093) |
| `/api/tags`, `/api/version` | ecco9-models (:8095) |
| `/v1/chat/completions`, `/v1/completions`, `/v1/embeddings` | ecco9-llm-gateway (:8093) |
| `/v1/models` | ecco9-models (:8095) |
| `/api/echo/status` | local aggregator (fans out to all `/healthz`) |
| `/api/echo/think` | ecco9-reservoir (:8081) |
| `/api/echo/feel` | ecco9-emotion (:8083) |
| `/api/echo/remember` | ecco9-memory (:8082) |
| `/healthz` | local health endpoint |

## Backend resolution

Each backend base URL comes from `ECCO9_SVC_<NAME>` (dashes become
underscores, e.g. `ECCO9_SVC_LLM_GATEWAY`), defaulting to in-cluster DNS such
as `http://ecco9-reservoir.ecco9-core:8081`. Backend ports: reservoir 8081,
memory 8082, emotion 8083, identity 8084, echobeats 8085, consciousness 8086,
atomspace 8087, relevance 8088, wisdom 8089, metacog 8090, ontogenesis 8091,
echodream 8092, llm-gateway 8093, runner 8094, models 8095, orchestrator 8096.

## Middleware

- **Rate limiting**: per-client token bucket (100 rps, burst 200).
- **Auth hook**: when `ECCO9_GATEWAY_API_KEYS` is set (comma-separated),
  requests must carry a matching `X-Ecco9-Key` header; `/healthz` and
  `/api/version` are exempt. Unset disables auth.

## Aggregate status

`GET /api/echo/status` probes every backend's `/healthz` concurrently and
returns per-service reachability plus an aggregate `identity_coherence` (mean
of reachable services) and a SERVING / DEGRADED / NOT_SERVING summary.

## Deploy

`deploy/deployment.yaml` ships a 3-replica Deployment in `ecco9-platform`, a
LoadBalancer Service, and an HPA scaling on the `ecco9_cognitive_load`
custom metric.
