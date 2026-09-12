# ecco9-memory

Hypergraph memory service — the **knowledge substrate** of the ecco9
cognitive architecture. Stores concepts, episodes, patterns, anomalies, and
echo signatures as a true hypergraph with weighted edges and multi-way
hyperedges.

Extracted/adapted from the monorepo sources:

- `core/memory/hypergraph.go` — hypergraph with adjacency indices, type and
  time indexes, BFS traversal with relation filters, and embedding-based
  cosine similarity search (`memory/hypergraph.go`).
- `core/memory/weaver.go` — usage-adaptive memory weaving: connections are
  strengthened when traversed often, weakened when stale, pruned when their
  strength decays to zero (`memory/weaver.go`).
- `core/memory/persistent.go` — durable snapshots. The production adapter
  targets Supabase/PostgreSQL; this module ships a file-backed snapshot store
  with the same Store/Load contract, atomically written to the StatefulSet
  volume (`memory/persistent.go`).

## Contracts

Implements `ecco9.memory.v1.MemoryService` from `ecco9-proto`:

| RPC | REST route |
|-----|-----------|
| `AddNode` | `POST /v1/memory/nodes` |
| `AddEdge` | `POST /v1/memory/edges` |
| `AddHyperEdge` | `POST /v1/memory/hyperedges` |
| `Traverse` | `POST /v1/memory/traverse` (BFS, `max_depth`, `relation_filter`) |
| `SimilaritySearch` | `POST /v1/memory/similarity` (top-k cosine over 768-dim embeddings) |

Extras: `GET /healthz` (SDK health reporter), `POST /v1/memory/weave`
(triggers one weaver adaptation cycle and returns graph stats).

## Configuration

| Env var | Default | Purpose |
|---------|---------|---------|
| `ECCO9_MEMORY_ADDR` | `:8082` | HTTP listen address |
| `ECCO9_MEMORY_STATE_DIR` | *(empty = in-memory)* | Snapshot directory |

## Deployment

StatefulSet in namespace `ecco9-memory` (`deploy/statefulset.yaml`): 3 shards
with 20Gi snapshot volumes, pod affinity to `ecco9-identity` (similarity
search shares the 768-dim embedding space), HPA on the `ecco9_cognitive_load`
custom metric (3–12 replicas).

## Scaling strategy

The hypergraph is sharded by node type across StatefulSet ordinals; the
weaver runs locally per shard so adaptation stays embarrassingly parallel.
Similarity search is the hot path and scales horizontally — each pod answers
queries over its shard and the gateway merges top-k results.
