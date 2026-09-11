# ecco9-models

Model registry and conversion lifecycle for the ecco9 inference plane:
pull/push/list/delete/convert, content-addressed blob management, and an
object-storage abstraction (`BlobStore`) with a local filesystem
implementation for offline operation (S3/MinIO satisfies the same interface
in production).

Adapted from `server/create.go`, `server/download.go`, `server/upload.go`,
`server/images.go`, `convert/convert.go`, and `fs/gguf/reader.go`. Mirrors
`ModelRegistryService` from `ecco9/models/v1/models.proto`.

## Layout

- `models/` — core domain logic
  - `registry.go` — `Manifest`/`Layer` (from `server/images.go`), the
    name→manifest `Registry` with register/get/list/delete, reference-counted
    blob pruning (`deleteUnusedLayers`), `GetSHA256Digest`, and GGUF/
    safetensors format sniffing (`fs/gguf` magic).
  - `blobstore.go` — `BlobStore` interface (Put/Get/Has/Delete/Stat) and
    `FSStore`, the offline filesystem implementation using the monorepo's
    `blobs/sha256-<hex>` layout and write-then-rename discipline.
  - `transfer.go` — `Puller` (pull from a `RemoteSource`, push with blob
    integrity verification, adapted from `download.go`/`upload.go` including
    the n² randomized backoff), progress streaming, and the `Converter`
    job lifecycle (queued → converting → quantizing → done/error, adapted
    from `convert/convert.go` + `quantizeLayer`).
- `server/service.go` — REST adapter for the proto contract.
- `cmd/modelsd/main.go` — HTTP server on `ECCO9_MODELS_ADDR`
  (default `:8095`), blob store root from `ECCO9_MODEL_STORE`.
- `deploy/statefulset.yaml` — StatefulSet with 200Gi PVC + Service + HPA on
  `ecco9_cognitive_load` (namespace `ecco9-inference`).

## Endpoints

| Method | Path | Proto RPC |
|---|---|---|
| GET | `/healthz` | — (sdk health reporter) |
| POST | `/v1/models/pull` | `PullModel` (NDJSON progress stream) |
| POST | `/v1/models/push` | `PushModel` (NDJSON progress stream) |
| GET | `/v1/models` | `ListModels` |
| DELETE | `/v1/models/{name}` | `DeleteModel` |
| POST | `/v1/models/convert` | `ConvertModel` → `{job_id}` |
| GET | `/v1/models/convert/{id}` | (conversion job introspection) |
| POST | `/v1/models/register/{name}` | (direct blob registration) |

Pulls fetch from `ECCO9_MODEL_SEED_DIR` (offline remote source). Blobs are
verified against their sha256 digest on push.

## Run

```sh
go build ./...
ECCO9_MODEL_STORE=./data go run ./cmd/modelsd
curl localhost:8095/healthz
```
