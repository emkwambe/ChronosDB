# ChronosDB — Product Requirements Document (PRD)

| Field          | Value                                                               |
| -------------- | ------------------------------------------------------------------- |
| Document Title | ChronosDB: Single-Node Bitemporal Graph Database with Learned Shortcuts |
| Version        | 2.0 (rescoped)                                                      |
| Status         | Active                                                              |
| Supersedes     | PRD v1.0 (2025-03-11), which described a distributed predictive multi-model system |

---

## 1. Background

Modern applications increasingly need to reason about how their data
*evolved* and to trace relationships as they existed at arbitrary points
in the past. Relational databases treat time as just another column,
producing painful self-join queries. Graph databases traverse
relationships well but lack first-class time semantics. Time-series
databases do time well but cannot traverse. No mainstream system combines
**native bitemporal graph storage** with **workload-adaptive indexing**.

ChronosDB fills that gap in the simplest form that proves worth:
single-node, bitemporal, self-optimizing via learned shortcuts.

## 2. Product overview

ChronosDB is a single-node graph database that:

- Stores every change to nodes, edges, and properties with both **valid
  time** and **transaction time**.
- Exposes **ChronosQL**, a Cypher-extended query language with temporal
  clauses (`AS OF`, `BETWEEN`, `HISTORY`, `AT`).
- Monitors its own query workload and **automatically materializes
  shortcut edges** for recurring temporal access patterns, so the system
  gets faster the more it is used.
- Ships as a single binary with BadgerDB embedded. No cluster, no
  coordinator, no external dependencies beyond the OS.

## 3. Goals and non-goals

### Primary goals

| ID | Goal                                                                                         |
| -- | -------------------------------------------------------------------------------------------- |
| G1 | Make point-in-time graph queries easy to write and correct by construction.                  |
| G2 | Cut recurring temporal query latency by **≥ 5×** via learned shortcuts.                       |
| G3 | Eliminate manual index tuning for the common case.                                           |
| G4 | Ship a honest, reproducible benchmark suite that compares favorably against Neo4j workarounds. |

### Secondary goals

- Provide clean gRPC + REST APIs with OpenAPI and proto documentation.
- Provide a minimal web console for live metrics, EXPLAIN, and shortcut inspection.
- Enable streaming ingest via a simple Kafka consumer.

### Non-goals (v1.0)

- Distributed / multi-node deployment
- Predictive analytics, forecasting, or any ML model besides the PGI
  frequency/latency estimators
- Multi-tenancy, LDAP/SSO, row-level access control
- SQL access layer, federated queries
- Kafka Connect plugin, Spark integration, Airflow operators

These may be reconsidered post-v1.0 per the cut-line table in
[`blueprint.md`](./blueprint.md).

## 4. Target users

| Persona                | Primary need                                                                                      |
| ---------------------- | ------------------------------------------------------------------------------------------------- |
| Compliance engineer    | Replay the state of permissions / records as of an arbitrary past date for audit.                 |
| Fraud investigator     | Inspect the transfer graph as it existed when a suspicious event occurred.                        |
| Analytics engineer     | Attribute events to the dimension members that were valid at event time (SCD-Type-2 done right).  |
| Platform / MLOps team  | Retrieve entity features as of a training-label timestamp without target leakage.                 |
| Incident responder     | Time-travel the service-dependency graph to 30 seconds before an outage.                          |

## 5. Use cases and user stories

1. **Compliance audit.** *"Show me who had access to record X on
   2023-06-14, and how that permission chain evolved."*
2. **Fraud investigation.** *"Replay the account-to-account transfer graph
   as it existed the day the suspicious wire cleared."*
3. **Supply-chain provenance.** *"Trace this batch backward through
   suppliers as of the production date, not today's suppliers."*
4. **Permission archaeology.** *"Did user U have permission to resource R
   via any group chain at time T?"*
5. **Slowly-changing dimensions.** *"Attribute this sale to the sales rep
   who owned the territory at the time of sale, not today's rep."*
6. **Knowledge graph versioning for ML.** *"Train on the entity embeddings
   as they existed when the label was generated."*
7. **Incident forensics.** *"What did the service-dependency graph look
   like 30 seconds before the outage?"*
8. **Regulatory reporting.** *"Prove the state of this patient record and
   its linked consents on the date of the DSAR."*

These eight define the acceptance test set for Phase 1 and the
benchmarking workload for Phase 4.

## 6. Functional requirements

### 6.1 Temporal data model

| ID   | Requirement                                                                           | Priority | Phase |
| ---- | ------------------------------------------------------------------------------------- | -------- | ----- |
| F1.1 | Nodes, edges, and properties with values that change over time.                        | P0       | 1     |
| F1.2 | Store both valid time and transaction time on every record.                            | P0       | 1     |
| F1.3 | Time-stamped edges with optional time-varying properties.                              | P0       | 1     |
| F1.4 | Optional schema; properties may appear and disappear over time.                        | P1       | 1     |
| F1.5 | System-maintained `sys_start` / `sys_end` for transaction time.                        | P1       | 1     |

### 6.2 Query language (ChronosQL)

| ID   | Requirement                                                                                      | Priority | Phase |
| ---- | ------------------------------------------------------------------------------------------------ | -------- | ----- |
| F2.1 | Cypher-like syntax with `AS OF <ts>`, `BETWEEN <t1> AND <t2>`, `HISTORY OF <prop>`, `AT <ts>`.   | P0       | 1     |
| F2.2 | Temporal functions: `duration.between()`, `timepoint()`, `overlaps()`.                            | P0       | 1     |
| F2.3 | Pattern matching across time-varying graphs.                                                      | P0       | 1     |
| F2.4 | Time-window aggregation (`PERIOD`).                                                               | P1       | 1-2   |
| F2.5 | Time-travel retrieval of full graph state as of any past timestamp.                               | P0       | 1     |
| F2.6 | `EXPLAIN` surfaces planner choices including shortcut usage.                                      | P0       | 2     |

### 6.3 Storage engine

| ID   | Requirement                                                                            | Priority | Phase |
| ---- | -------------------------------------------------------------------------------------- | -------- | ----- |
| F3.1 | Snapshot + delta storage of temporal history.                                           | P0       | 1     |
| F3.2 | Periodic compaction of deltas into snapshots, crash-safe via WAL.                       | P0       | 1     |
| F3.3 | Time-based partitioning for pruning old data.                                           | P1       | 1     |
| F3.4 | MVCC snapshot isolation for writes.                                                     | P0       | 1     |
| F3.5 | Storage overhead ≤ 2.5× the equivalent current-state graph.                             | P0       | 1     |

### 6.4 Predictive Graph Index (PGI)

| ID   | Requirement                                                                                    | Priority | Phase |
| ---- | ---------------------------------------------------------------------------------------------- | -------- | ----- |
| F4.1 | Sample executed queries and extract normalized signatures with frequency and latency.          | P0       | 2     |
| F4.2 | Detect frequent temporal motifs (single-hop and 2-hop).                                        | P0       | 2     |
| F4.3 | Materialize shortcut edges when `benefit > cost`; store in dedicated column family.            | P0       | 2     |
| F4.4 | Cost-benefit reevaluation and eviction (TTL + LRU + cap-based).                                | P1       | 2     |
| F4.5 | `EXPLAIN` shows shortcuts used, estimated and actual benefit.                                   | P1       | 2     |
| F4.6 | PGI never blocks the write or read path; monitor overhead < 1% CPU.                            | P0       | 2     |
| F4.7 | Shortcuts are hints — correctness must not depend on them.                                      | P0       | 2     |

### 6.5 APIs and interfaces

| ID   | Requirement                                                                       | Priority | Phase |
| ---- | --------------------------------------------------------------------------------- | -------- | ----- |
| F5.1 | gRPC service with `Execute` streaming and `Import` streaming RPCs.                 | P0       | 1     |
| F5.2 | REST gateway (grpc-gateway) with OpenAPI spec.                                     | P0       | 1     |
| F5.3 | Go client; Python / Node.js client generated from proto.                            | P1       | 3     |
| F5.4 | Bulk import of CSV / JSON / SQL with explicit `valid_from` / `valid_to` columns.   | P0       | 1     |
| F5.5 | Simple single-node Kafka consumer with idempotent upserts and lag metrics.         | P1       | 3     |
| F5.6 | Web console with metrics, query editor with EXPLAIN, shortcut inspector.           | P1       | 3     |

### 6.6 Operability

| ID   | Requirement                                                                 | Priority | Phase |
| ---- | --------------------------------------------------------------------------- | -------- | ----- |
| F6.1 | Prometheus metrics: QPS, p50/p99, shortcut hit rate, compaction lag, etc.   | P0       | 3     |
| F6.2 | Snapshot backup + WAL-based restore; chaos tested.                           | P0       | 3     |
| F6.3 | Structured logging with `log/slog`.                                          | P0       | 1     |
| F6.4 | TLS 1.3 + token auth + audit log of queries.                                 | P0       | 4     |

## 7. Non-functional requirements

### 7.1 Performance targets

| Metric                                         | Target          | Phase |
| ---------------------------------------------- | --------------- | ----- |
| Sustained write throughput                     | ≥ 50k ops/sec   | 1     |
| Point-lookup p99 (`AS OF`)                     | < 10 ms         | 1     |
| Single-hop temporal traversal p99              | < 50 ms         | 1     |
| PGI cold→warm p99 improvement after 1000 queries | ≥ 5×          | 2     |
| Storage overhead vs current-state graph        | ≤ 2.5×          | 1-2   |
| Bulk-import throughput                         | ≥ 500 MB/s      | 3     |

### 7.2 Reliability

- Crash-safe WAL; compaction is transactional.
- 72-hour soak without memory leak, compaction stall, or shortcut cache bloat (Phase 4 gate).

### 7.3 Security

- TLS 1.3 on gRPC and REST.
- Bearer-token auth on REST; mTLS option on gRPC.
- Audit log of all queries, append-only.
- OWASP top-10 review clean at v1.0.

## 8. Constraints and assumptions

- **Single-node only.** Distribution is permanently out of scope for v1.0.
- **Predictive models must be lightweight.** No neural nets, no
  forecasting. Count-min sketch + EWMA only.
- **Bitemporal correctness is load-bearing.** If a shortcut could ever
  return a result that contradicts a from-scratch query, it is a bug.
- Target hardware: x86_64 Linux, 16 cores / 64 GB RAM / NVMe SSD for
  benchmark reproduction.

## 9. Release criteria

A release is ready when **all** of its phase's exit criteria in
[`blueprint.md`](./blueprint.md) are green *and* the guardrails in
[`../architecture-guardrails.md`](../architecture-guardrails.md) have
been respected throughout. Partial completion does not ship.

## 10. Glossary

- **TKG** — Temporal Knowledge Graph: graph with time-aware nodes/edges.
- **PGI** — Predictive Graph Index: subsystem that learns shortcuts.
- **Shortcut** — pre-computed edge/view stored to accelerate queries.
- **Valid time** — when a fact is true in reality.
- **Transaction time** — when a fact was recorded in the database.
- **MVCC** — Multi-Version Concurrency Control.
- **LSM** — Log-Structured Merge tree.
