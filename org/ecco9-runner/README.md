# ecco9-runner

Local GGUF inference runner **control plane** for the ecco9 inference plane:
model registry, KV cache management, load/unload lifecycle, and an inference
job queue with a pluggable backend interface. The actual ggml/llama.cpp cgo
engine is not portable to this environment, so the backend is the `Backend`
interface with an offline `StubBackend`; production swaps in the real engine.

Adapted from `runner/llamarunner/runner.go`, `runner/ollamarunner/runner.go`,
`kvcache/cache.go`, `llm/server.go`, and `llm/memory.go`. Mirrors
`RunnerService` from `ecco9/llm/v1/llm.proto`.

## Layout

- `runner/` — core domain logic
  - `registry.go` — `Registry` (loaded-model table, adapted from
    `llm/server.go` scheduling + `Server.loadModel` lifecycle:
    loading → ready/error → unloaded), `LoadParams`, status enum mirroring
    `llm.ServerStatus`, and `EstimateMemory` (simplified
    `llm.EstimateGPULayers`: per-layer weight share + KV bytes for the
    context window).
  - `kvcache.go` — control-plane adaptation of the `kvcache.Cache`
    interface: slot `Acquire`/`Release` (LoadCacheSlot), `Put`, `CopyPrefix`,
    `CanResume`, ranged `Remove`, `ErrKVCacheFull`.
  - `queue.go` — bounded inference job queue with parallel workers, adapted
    from the ollamarunner `seqsSem` + `nextSeq` scheduling loop; `Backend`
    interface plus offline `StubBackend` that streams deterministic tokens.
- `server/service.go` — REST adapter for the proto contract.
- `cmd/runnerd/main.go` — HTTP server on `ECCO9_RUNNER_ADDR`
  (default `:8094`).
- `deploy/deployment.yaml` — GPU Deployment (`nvidia.com/gpu: 1`,
  gpu-pool `nodeSelector`/`tolerations`) + Service + HPA on
  `ecco9_cognitive_load` (namespace `ecco9-inference`).

## Endpoints

| Method | Path | Proto RPC |
|---|---|---|
| GET | `/healthz` | — (sdk health reporter) |
| POST | `/v1/runner/models` | `LoadModel` |
| DELETE | `/v1/runner/models/{id}` | `UnloadModel` |
| GET | `/v1/runner/models` | (control-plane introspection) |
| POST | `/v1/runner/generate` | `Generate` (NDJSON stream) |

## Run

```sh
go build ./...
go run ./cmd/runnerd
curl -X POST localhost:8094/v1/runner/models \
  -d '{"model_path":"/var/lib/ecco9/models/model.gguf","gpu_layers":33,"context_size":4096}'
```
