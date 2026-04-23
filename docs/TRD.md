# ChronosDB — Technical Requirements Document (TRD)

| Field          | Value                                                                   |
| -------------- | ----------------------------------------------------------------------- |
| Document Title | ChronosDB: Technical Requirements                                       |
| Version        | 2.0 (rescoped)                                                          |
| Status         | Active                                                                  |
| Supersedes     | TRD v1.0 (2025-03-11), which described a distributed multi-tier system |

This TRD translates the rescoped [PRD](./PRD.md) into concrete technical
designs, data structures, algorithms, and performance targets for a
**single-node** bitemporal graph database with a learned-shortcut index.

---

## 1. System overview

ChronosDB v1.0 has exactly two internal subsystems on top of a common
storage layer:

1. **Temporal Graph Core (TKG).** BadgerDB-backed bitemporal storage,
   ChronosQL parser/planner/executor, MVCC, WAL, compaction.
2. **Predictive Graph Index (PGI).** Sampling workload monitor, pattern
   detector, cost-benefit shortcut manager, eviction.

Wrapping both: a gRPC service, a REST gateway, Prometheus metrics, an
audit log, and a static web console.

There are **no** other subsystems in v1.0. There is no coordinator, no
sharding, no forecasting, no multi-tenancy module, no Kafka Connect.

## 2. Architecture

```
                ┌──────────────────────────────────────────────┐
                │                Client                         │
                │   (gRPC driver / REST / web console)          │
                └─────────────────────┬────────────────────────┘
                                      │
                                      ▼
                ┌──────────────────────────────────────────────┐
                │              chronosd (single process)        │
                │                                                │
                │  ┌──────────────┐     ┌─────────────────────┐  │
                │  │  gRPC server │     │ REST (grpc-gateway) │  │
                │  └───────┬──────┘     └──────────┬──────────┘  │
                │          │                        │             │
                │          └────────┬───────────────┘             │
                │                   ▼                              │
                │         ┌───────────────────┐                    │
                │         │   Query Executor   │                    │
                │         └─────────┬─────────┘                    │
                │                   │                              │
                │                   ▼                              │
                │         ┌───────────────────┐                    │
                │         │   Planner         │◄─┐                 │
                │         └─────────┬─────────┘  │                 │
                │                   │            │ shortcut paths  │
                │                   ▼            │                 │
                │         ┌───────────────────┐  │  ┌───────────┐  │
                │         │   Storage (TKG)   │  └──│    PGI    │  │
                │         │ ┌───────────────┐ │     └─────┬─────┘  │
                │         │ │ BadgerDB KV   │ │           │        │
                │         │ │ nc: / nh:     │ │           │ samples│
                │         │ │ ec: / eh:     │ │◄──────────┘        │
                │         │ │ sc: shortcuts │ │                    │
                │         │ │ value log     │ │                    │
                │         │ └───────────────┘ │                    │
                │         └───────────────────┘                    │
                │                                                    │
                │  ┌──────────────┐   ┌──────────────────────┐      │
                │  │ Prometheus   │   │ Audit log / slog     │      │
                │  │ /metrics     │   │                       │      │
                │  └──────────────┘   └──────────────────────┘      │
                └──────────────────────────────────────────────┘
```

One binary. One data directory. One `docker-compose.yml` for the full
demo (chronosd + optional Kafka + Prometheus + Grafana).

## 3. Storage engine

**Backend.** BadgerDB v4 (pure-Go LSM). No pluggable backends in v1.0.

**Logical column families.** Badger does not have named column families like
some other LSM stores, so CFs are implemented as key prefixes in a single
keyspace.

| Prefix | Logical CF        | Contents                                                  |
| ------ | ----------------- | --------------------------------------------------------- |
| `nc:`  | `nodes_current`   | Latest snapshot of node labels and properties (derived).  |
| `ec:`  | `edges_current`   | Latest snapshot of edges (derived).                       |
| `nh:`  | `nodes_history`   | Temporal deltas for nodes.                                |
| `eh:`  | `edges_history`   | Temporal deltas for edges.                                |
| `ix:`  | `indexes`         | Secondary indexes on property values.                     |
| `sc:`  | `shortcuts`       | PGI-materialized shortcut edges with validity windows.    |
| `mt:`  | `meta`            | Schema, counters, sequence numbers, shortcut manifests.   |

**Key encoding.**

```
node:      {partition}:n:{node_id}:{valid_from}:{txn_id}
edge:      {partition}:e:{edge_id}:{valid_from}:{txn_id}
property:  {partition}:p:{entity_type}:{entity_id}:{prop}:{valid_from}
shortcut:  {partition}:s:{pattern_id}:{src_id}:{dst_id}:{valid_from}
```

Invariant: history keys are append-only. Current-state CFs are *derived*
from history and can always be rebuilt by replaying transactions from
time zero.

**Temporal storage.** Snapshot + periodic compaction: a base snapshot
plus change logs; compaction merges deltas into a new snapshot at a
configurable interval (default daily). Data older than a threshold moves
into read-only time buckets (monthly).

**Transactions.** MVCC snapshot isolation on a single node. No 2PC, no
Raft — single-process commit via a group-commit WAL. Crash recovery
replays the WAL up to the last durable checkpoint.

**Caching.** BadgerDB block and index cache for hot data; a thin LRU
cache in ChronosDB for frequently accessed time slices; a dedicated
shortcut cache keyed by `(pattern_id, valid_range)`.

## 4. Query engine

**Parser.** Cypher-like grammar extended with:

```
temporal-clause = "AS OF" timestamp
                | "BETWEEN" timestamp "AND" timestamp
                | "HISTORY" "OF" property-expression
                | "AT" timestamp
timestamp       = ISO-8601-datetime / integer-microseconds / parameter
```

**Logical planner.** AST → logical tree with temporal operators
(`TimeSlice`, `TemporalJoin`, `TemporalAggregate`), using relational
algebra extended with time.

**Physical planner.** Cost-based optimization using statistics
(histograms, cardinalities). Choice of access path from: primary-key
lookup, secondary index scan, full CF scan, or **shortcut scan** (PGI
paths). For time-range queries, prune partitions using time bounds.

**Execution.** Pull-based Volcano executor, streaming results over gRPC.

**EXPLAIN.** Returns the chosen plan tree, the estimated cost per
operator, and — when PGI shortcuts are used — the shortcut ID, its
estimated benefit, and how stale it is.

## 5. Predictive Graph Index

### 5.1 Workload monitor

- Sampled (default 10%, configurable) lock-free ring buffer of completed
  queries.
- Each entry: normalized signature, wall-clock latency, access paths
  used, estimated cardinality.
- Signature normalization: AST walker that strips literals and parameter
  values but preserves shape, labels, relationship types, and temporal
  clauses.

### 5.2 Pattern detection

- Per-signature count-min sketch for frequency.
- EWMA for recency (half-life configurable; default 24h).
- Per-signature latency histogram (HDR-style, bounded).
- Motif extractor surfaces frequent 1-hop and 2-hop relationship patterns
  with their temporal envelopes.

### 5.3 Shortcut manager

- Candidate = (motif, time-envelope, projection).
- `benefit = avg_latency_saved × freq`
- `cost = storage_bytes + write_amplification × write_rate`
- Materialize when `benefit > threshold × cost`; reevaluate on fixed
  interval.
- Shortcuts stored in the `shortcuts` CF with `(pattern_id, valid_from,
  valid_to, provenance, estimated_benefit)`.
- On base-data change: enqueue async invalidation job; shortcut marked
  stale and skipped by planner until refreshed.

### 5.4 Eviction

- TTL (idle shortcuts evicted after inactivity).
- LRU tie-break.
- Hard cap: shortcut-CF size ≤ 20% of base graph size.
- Eviction never blocks reads or writes; planner simply stops using the
  evicted shortcut.

### 5.5 Correctness guardrail

Shortcuts are *hints*. For every query, the planner must have a
from-scratch alternative; a corrupted or stale shortcut may degrade
latency but must never change the result set. Property tests verify:
*for all queries Q, result(Q with shortcuts) == result(Q without)*.

## 6. Data models (protobuf)

```proto
message NodeRecord {
  string id = 1;
  repeated string labels = 2;
  map<string, TemporalValue> properties = 3;
  int64 valid_from = 4;          // microseconds since epoch
  int64 valid_to   = 5;          // 0 = unbounded future
  int64 txn_id     = 6;
}

message EdgeRecord {
  string id = 1;
  string type = 2;
  string source_id = 3;
  string target_id = 4;
  map<string, TemporalValue> properties = 5;
  int64 valid_from = 6;
  int64 valid_to   = 7;
  int64 txn_id     = 8;
}

message TemporalValue {
  repeated ValueChange changes = 1;
}

message ValueChange {
  oneof value {
    int64  int_val    = 1;
    double double_val = 2;
    string str_val    = 3;
    bool   bool_val   = 4;
  }
  int64 valid_from = 10;
  int64 valid_to   = 11;
}

message ShortcutRecord {
  string pattern_id = 1;          // e.g., "Customer_to_Product"
  string source_id  = 2;
  string target_id  = 3;
  map<string, Value> properties = 4;   // pre-aggregated / projected
  int64  valid_from = 5;
  int64  valid_to   = 6;
  double estimated_benefit = 7;
  string provenance = 8;          // pointer back to base records
}
```

## 7. APIs

### 7.1 gRPC (primary)

```proto
service ChronosDB {
  rpc Execute(QueryRequest) returns (stream QueryResponse);
  rpc Import(stream ImportRecord) returns (ImportSummary);
  rpc GetMetadata(MetadataRequest) returns (MetadataResponse);
  rpc Admin(AdminRequest) returns (AdminResponse);   // backup, restore, config
}
```

### 7.2 REST (grpc-gateway)

- `POST /v1/db/{db}/query` — JSON body `{ "query": "...", "params": {...} }`
- `POST /v1/db/{db}/import` — newline-delimited JSON stream
- `GET  /v1/db/{db}/stats` — point-in-time metrics
- `POST /v1/admin/backup` / `POST /v1/admin/restore`

### 7.3 OpenAPI

Generated from the proto definitions. Committed alongside proto changes;
drift is a CI failure.

## 8. Observability

- **Metrics (Prometheus).** QPS, p50/p99 per endpoint, write rate, WAL
  fsync latency, compaction lag, storage size per CF, shortcut hit rate,
  shortcut cache size, Kafka lag (when enabled).
- **Logging.** Structured via `log/slog`; per-request correlation ID.
- **Audit log.** Append-only file (rotated daily) recording
  `(ts, principal, query, affected_ids, result_size)` for every query.

## 9. Security

- TLS 1.3 on gRPC and REST endpoints (configurable cert paths).
- Bearer-token auth on REST; optional mTLS on gRPC.
- Optional at-rest encryption via BadgerDB's built-in AES encryption.
- OWASP top-10 review of the API surface at v1.0 (Phase 4 gate).

## 10. Deployment and operations

- **Packaging.** One statically-linked binary; minimal Alpine Docker
  image; `docker-compose.yml` for the full demo stack.
- **Configuration.** One YAML file (`chronosdb.yaml`); env-var overrides
  via `CHRONOSDB_<SECTION>_<KEY>`; hot-reload for PGI thresholds only.
- **Backup.** Snapshot-based (BadgerDB `Backup`/`Load`) + value-log archiving.
- **Restore.** Load checkpoint, replay WAL to target timestamp.
- **Upgrades.** In-place with version compatibility declared per minor
  version; downgrades unsupported across minor versions.

## 11. Testing

- **Unit tests.** Every public function in `internal/storage`,
  `internal/query`, `internal/pgi`.
- **Bitemporal property tests.** For all `(q, valid_time,
  transaction_time)`, `exec(q)` at `(v, t)` is reproducible forever.
- **Integration tests.** Each use case from the PRD §5 runs end to end
  on the reference dataset and matches ground truth.
- **Benchmark suite.** `make bench` — mandatory regression gate at ±10%.
- **Chaos tests.** Kill during compaction, backup, restore, bulk import.
- **Soak test.** 72 hours at 60% of peak throughput (Phase 4 gate).

## 12. Performance targets

Reproduced here from the PRD for engineering reference:

| Metric                                           | Target        | Phase |
| ------------------------------------------------ | ------------- | ----- |
| Point lookup p99 (`AS OF`)                       | < 10 ms       | 1     |
| Single-hop traversal p99                         | < 50 ms       | 1     |
| Complex multi-hop p99 (no shortcut)              | < 2 s         | 2     |
| Complex multi-hop p99 (with shortcut)            | < 200 ms      | 2     |
| Sustained write throughput                       | ≥ 50k ops/sec | 1     |
| Bulk import throughput                           | ≥ 500 MB/s    | 3     |
| Shortcut availability after pattern detection    | ≤ 5 minutes   | 2     |
| Storage overhead                                 | ≤ 2.5×        | 1-2   |
| Monitor CPU overhead                             | < 1%          | 2     |

## 13. Out of scope (will not be implemented in v1.0)

- Distributed transactions, sharding, replication.
- Forecasting, LSTM/Prophet/XGBoost, GNN-based shortcut prediction.
- Multi-tenancy, LDAP/SSO, RBAC below the database level.
- SQL access layer, federated queries, document/key-value APIs.
- Kafka Connect plugin, Spark integration.

## 14. References

- [Rescoped PRD](./PRD.md)
- [Development Blueprint](./blueprint.md)
- [Sprint Plan](./sprints.md)
- [Architecture Guardrails](../architecture-guardrails.md)
