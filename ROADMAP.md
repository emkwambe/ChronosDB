# ChronosDB Roadmap

**Current version:** `0.1.0-dev`
**Target for v1.0:** ~28 weeks, 4 phases, 11 sprints.

This roadmap supersedes all prior "v5.0 COMPLETE" claims. The codebase is at
Phase 1 level functionally; Phase 0 (repo hygiene) is in progress. Nothing
rolls forward until its phase's exit criteria are all green.

For the full phase/sprint breakdown see [`docs/blueprint.md`](./docs/blueprint.md)
and [`docs/sprints.md`](./docs/sprints.md).

---

## Phase 0 — Repo hygiene & foundation (2 weeks)

Strip overclaims, lock in CI, define the reference workload.

**Exit criteria**

- [x] Delete `predictive/`, `multitenancy/`, `streaming/`, `tenantctl`,
      `kafka_producer`, duplicate PDFs, `temp_method.txt`
- [x] README rewritten; version set to `0.1.0-dev`
- [x] CI pipeline (`.github/workflows/ci.yml`): `go vet` + `go mod tidy`
      drift check + `staticcheck` + `go test -race` + benchmark baseline
      captured as a GitHub Actions artifact
- [x] Reference dataset generator committed at `test/synthetic/`
      (package `synthetic`): deterministic, bitemporal, scale profiles
      Small / Medium / Large (Large = 10M edges). Determinism + ground-truth
      consistency tests pass.
- [x] `make bench` produces reproducible baseline JSON
      (`bench-results.json`) via `scripts/bench.sh`.

## Phase 1 — Bitemporal core (6 weeks)

Single-node TKG with provable bitemporal semantics.

**Exit criteria**

- 100% of bitemporal correctness test suite passes
- `AS OF` point-lookup p99 < 10ms on reference dataset
- Single-hop temporal traversal p99 < 50ms
- Write throughput ≥ 50k updates/sec/node on SSD
- Storage overhead ≤ 2.5× current-state equivalent
- Compaction is crash-safe (kill-during-compaction test passes)

## Phase 2 — Predictive Graph Index (6 weeks)

Workload monitor, cost-benefit shortcut manager, planner integration, eviction.

**Exit criteria**

- Workload monitor captures normalized signatures with <1% CPU overhead
- Pattern → shortcut → planner → latency-drop pipeline end-to-end
- `EXPLAIN` shows shortcut usage and estimated benefit
- Warm p99 ≥ 5× better than cold p99 after 1000 queries on YCSB-temporal
- Shortcut storage capped at 20% of base graph; TTL eviction works
- Write path latency regression from PGI ≤ 5%

## Phase 3 — Operability & ingestion (4 weeks)

Make it usable by someone who isn't the author.

**Exit criteria**

- Bulk importer sustains 500 MB/s on reference hardware
- Simple single-node Kafka consumer (idempotent upserts, lag metrics)
- Full Prometheus metric coverage
- Snapshot backup + WAL restore verified by chaos test
- Docker demo + three tutorials (compliance audit, SCD analytics,
  permission archaeology)
- REST + gRPC APIs fully documented (OpenAPI + proto comments)

## Phase 4 — Benchmarks, hardening, v1.0 (4 weeks)

Prove the worth publicly; tag `v1.0.0`.

**Exit criteria**

- Four benchmarks published with reproducible harness:
  1. ≥10× faster AS OF traversal vs Neo4j (temporal workaround) on 100M-edge graph
  2. PGI warm p99 ≥ 5× better than cold p99 on YCSB-temporal
  3. Storage overhead ≤ 2.5× confirmed on real workload trace
  4. Positive shortcut ROI on captured investigation workload
- 72-hour soak test passes (no leak, no compaction stall, no cache bloat)
- TLS 1.3, token auth, audit log, OWASP review clean
- `v1.0.0` tagged with release notes, migration guide, blog post

---

## Candidate post-v1.0 extensions

Designed but **not committed**. Lives as a sketch so the v1.0
architecture does not foreclose them; explicitly excluded from v1.0
exit criteria.

- **Phase 5 (candidate): Temporal Value Analytics (TVA).** Built-in
  `decay()` / `age()` functions in ChronosQL, per-edge-type learned
  decay rates, scenario-rerun overrides. Inspired by time-value-of-
  money analytics. See
  [`docs/phase5-temporal-value.md`](./docs/phase5-temporal-value.md)
  and [`docs/chronosql-decay.md`](./docs/chronosql-decay.md).
  Promotion to committed requires Phase 4 green + PRD amendment +
  two confirmed pilot users.

---

## Permanently out of scope (v1.0)

These were in prior docs but do not serve the single-node,
bitemporal-graph-with-learned-shortcuts thesis. They will not ship in v1.0
and may never ship:

- Distributed mode / sharding / coordinator / etcd
- Predictive Analytics Layer (PAL) / forecasting / ML model registry
- Multi-tenancy and LDAP/SSO
- SQL access layer (Calcite)
- Federated queries across clusters
- Graph Neural Network shortcut prediction
- Kafka Connect plugin (the simple consumer above is sufficient)

Bringing any of these back requires a new PRD amendment and explicit
re-scoping, not quiet feature creep.
