# ecco9-llm-gateway

Unified multi-provider LLM gateway for the ecco9 inference plane: provider
registry, circuit breakers, rate limiting, failover, and cost tracking across
OpenAI, Anthropic, OpenRouter, Featherless, and local GGUF backends.

Adapted from `core/deeptreeecho/multi_provider_llm.go`,
`llm_client.go`, `llm_client_v6.go`, `anthropic_provider.go`,
`openai_provider.go`, `openrouter_provider.go`, `featherless_client.go`, and
`core/llm/provider.go`. Mirrors `LLMGatewayService` from
`ecco9/llm/v1/llm.proto`.

## Layout

- `gateway/` — core domain logic
  - `provider.go` — provider enum, request/response types, backend registry,
    priority-sorted selection with preferred-provider-first ordering, and
    per-request cost accounting (mirrors `MultiProviderLLM` failover loop).
  - `breaker.go` — three-state circuit breaker (closed/open/half-open with
    failure threshold and reset timeout), token-bucket rate limiter, and
    per-provider stats with token/cost tracking.
  - `gateway.go` — `Gateway` router with failover and provider status, plus
    `StubTransport` (offline `Transport` implementation). Provider HTTP calls
    are abstracted behind the `Transport` interface so the service compiles
    and runs offline; inject an HTTP transport for production.
- `server/service.go` — REST adapter for the proto contract.
- `cmd/llmgatewayd/main.go` — HTTP server on `ECCO9_LLM_GATEWAY_ADDR`
  (default `:8093`).
- `deploy/deployment.yaml` — Deployment + Service + HPA on
  `ecco9_cognitive_load` (namespace `ecco9-inference`).

## Endpoints

| Method | Path | Proto RPC |
|---|---|---|
| GET | `/healthz` | — (sdk health reporter) |
| POST | `/v1/llm/generate` | `Generate` |
| POST | `/v1/llm/generate/stream` | `GenerateStream` (NDJSON) |
| GET | `/v1/llm/providers/status` | `GetProviderStatus` |

## Providers

Backends register only when their API key env var is present
(`ANTHROPIC_API_KEY`, `OPENROUTER_API_KEY`, `OPENAI_API_KEY`,
`FEATHERLESS_API_KEY`); the local GGUF backend (via `ECCO9_RUNNER_URL`) is
always registered unless `ECCO9_DISABLE_LOCAL_GGUF` is set. Priority order:
anthropic 100 > openrouter 90 > openai 70 > featherless 50 > local 10.

## Run

```sh
go build ./...
go run ./cmd/llmgatewayd
curl localhost:8093/healthz
```
