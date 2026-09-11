# ecco9-atomspace

AtomSpace service — the **weighted, labeled hypergraph** knowledge store of
the ecco9 cognitive architecture, with ECAN attention allocation and
clause-based pattern matching.

Extracted from `core/opencog/atomspace.go`:

- `AtomSpace` — core hypergraph: atoms, links, and an incoming reverse index
  for efficient traversal.
- `TruthValue` — PLN probabilistic truth values (strength / confidence /
  count) with and/or/not/revision fusion.
- `AttentionBank` — ECAN economic attention: STI/LTI funds, attention-focus
  boundary, importance indices, and a forgetting mechanism that prunes
  low-focus atoms and orphaned links.
- `PatternMatcher` — clause-based pattern matching with `$variable` binding
  over link outgoing sets (from the `Match`/`matchClause` implementation).
- `SpreadAttention` — ECAN diffusion round: STI flows from links to outgoing
  atoms proportional to truth-value strength.

## Contracts

Implements `ecco9.atomspace.v1.AtomSpaceService` from `ecco9-proto`:

| RPC | REST route |
|-----|-----------|
| AddAtom | `POST /v1/atomspace/atoms` (`{"atom": {"type": 1, "name": "cat", "truth": {...}}}`) |
| AddLink | `POST /v1/atomspace/links` (`{"link": {"type": "InheritanceLink", "outgoing_atom_ids": [...]}}`) |
| PatternMatch | `POST /v1/atomspace/pattern-match` (`{"pattern": "InheritanceLink($x, <atom-id>)", "max_results": 50}`) |
| UpdateAttention | `POST /v1/atomspace/attention` (`{"atom_id": "...", "attention": {"sti": 100, "lti": 10}}`) |

Additional routes:

- `GET /v1/atomspace/atoms/{id}` — atom/link lookup by ID.
- `GET /v1/atomspace/status` — atom/link counts, ECAN funds, AF boundary.
- `GET /healthz` — status, identity coherence, cognitive load.

Pattern query language: semicolon-separated clauses
`LinkType($var, concrete-id, ...)`; `$`-prefixed tokens bind, all others must
match outgoing IDs exactly.

## Running

```
ECCO9_ATOMSPACE_ADDR=:8087 go run ./cmd/atomspaced
```

## Deployment

StatefulSet with 3 shards in namespace `ecco9-memory`
(`deploy/statefulset.yaml`), headless Service for stable per-pod DNS, 20Gi
persistent volumes for hypergraph checkpoint recovery. HPA scales 3-10 pods
on the `ecco9_cognitive_load` custom metric (average 0.6): hypergraph
occupancy and pattern-match queue depth drive scaling, not CPU. Sharding is
by atom-ID hash; ECAN `SpreadAttention` runs per-shard and cross-shard
diffusion flows over the event mesh.
