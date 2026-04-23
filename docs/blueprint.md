# ChronosDB Development Blueprint

**Thesis.** A single-node, bitemporal graph database that learns from its
own workload and materializes shortcut edges for recurring temporal
queries. Two subsystems, no distribution, no forecasting.

**Horizon.** ~28 weeks / 4 phases / 11 sprints / one engineer full-time
(or two part-time). Each sprint is 2 weeks.

This blueprint is the contract between phases. The sprint-by-sprint
execution plan lives in [`sprints.md`](./sprints.md). The non-negotiable
rules for every phase live in
[`../architecture-guardrails.md`](../architecture-guardrails.md).

---

## Phase 0 — Repo hygiene & foundation (1 sprint / 2 weeks)

**Goal.** Get the codebase honest. Strip overclaims, delete stubs, lock in
CI, define the reference workload.

**Scope.**
- Remove dead packages (`predictive`, `multitenancy`, `streaming`,
  `kafka_producer`, `tenantctl`) and clutter files (root PDFs,
  `temp_method.txt`, old phase blueprints).
- Rewrite README, ROADMAP, PRD, TRD, architecture guardrails to match
  reality.
- Stand up CI: `go vet`, `staticcheck`, `go test -race`, and a reproducible
  benchmark harness (`make bench`) that emits JSON.
- Commit a deterministic synthetic bitemporal graph generator (10M edges)
  as the reference dataset for every perf claim.

**Exit criteria.**
- [x] Overclaimed packages and clutter removed from the tree.
- [x] README / ROADMAP / guardrails / PRD / TRD rewritten and checked in.
- [ ] CI pipeline green on `main`.
- [ ] `make bench` produces baseline JSON artifact.
- [ ] Reference dataset generator committed with a seeded ground-truth
      query set.

**Risks.** None material. If CI setup drags, it replaces a sprint's work —
it does not slip the phase.

---

## Phase 1 — Bitemporal core (3 sprints / 6 weeks)

**Goal.** A single-node temporal graph store with *provable* bitemporal
semantics, MVCC snapshot isolation, WAL-durable writes, crash-safe
snapshot+delta compaction, and ChronosQL for `AS OF` / `BETWEEN` /
`HISTORY` / `AT`.

**Scope.**
- Lock down key encoding (`{partition}:node:{id}:{valid_from}:{txn_id}`)
  and document the invariants.
- Implement MVCC snapshot isolation (transaction manager, read timestamps,
  write conflict detection) and a group-commit WAL.
- Build the ChronosQL parser for temporal clauses and functions
  (`AS OF`, `BETWEEN`, `HISTORY`, `AT`, `duration.between()`, `overlaps()`).
- Logical planner with `TimeSlice`, `TemporalJoin`, `TemporalAggregate`.
- Pull-based Volcano executor; streaming results.
- Background snapshot+delta compaction; time-partitioned buckets become
  read-only.

**Exit criteria.**
- 100% of the bitemporal correctness test suite passes (point-in-time
  reproducibility, transaction-time monotonicity, valid-time overlap
  semantics).
- `AS OF` point-lookup **p99 < 10ms** on reference dataset.
- Single-hop temporal traversal **p99 < 50ms**.
- Sustained write throughput **≥ 50k updates/sec** on SSD.
- Storage overhead **≤ 2.5×** current-state equivalent.
- Compaction is crash-safe (kill-during-compaction test recovers cleanly
  from WAL).

**Risks.** MVCC correctness is the single biggest hazard. Bias toward
simpler algorithms and exhaustive property-based tests; do not optimize
before the test suite is green.

---

## Phase 2 — Predictive Graph Index (3 sprints / 6 weeks)

**Goal.** The differentiator. The database observes its own query traffic
and creates shortcut edges for frequent temporal patterns, such that the
planner picks them automatically and recurring queries get dramatically
faster.

**Scope.**
- Workload monitor: sampled (default 10%) lock-free ring buffer of
  normalized query signatures.
- Pattern detector: count-min sketch for frequency, EWMA for recency, a
  per-signature latency histogram. **No neural nets. No Markov chains.**
- Cost-benefit shortcut manager:
  `benefit = avg_latency_saved × frequency`,
  `cost = storage + write_amplification`. Materializes when benefit clears
  a configurable threshold.
- Shortcuts stored under a dedicated BadgerDB key prefix (`sc:`) with
  validity windows, provenance metadata, and an estimated-benefit attribute.
- Planner integration: shortcuts appear as alternative access paths with
  selectivity estimates. `EXPLAIN` surfaces which were used and why.
- Shortcut maintenance: async invalidation on base-data changes
  (bounded-staleness, never blocks writes); TTL + LRU + cap-based
  eviction.

**Exit criteria.**
- Workload monitor CPU overhead **< 1%** under sustained load.
- Pattern-detected → shortcut-materialized → planner-picked →
  latency-dropped pipeline works end to end on reference workload.
- `EXPLAIN` shows shortcut usage and estimated vs actual benefit.
- **Warm p99 ≥ 5× better than cold p99** after 1000 queries on
  YCSB-temporal.
- Shortcut storage capped at 20% of base graph; TTL + cap-based eviction
  works under churn.
- Write-path latency regression from PGI **≤ 5%**.

**Risks.** Shortcut invalidation under concurrent writes is subtle —
wrong design leads to stale reads. Guardrail: shortcuts are *hints*, and
the planner must always be able to ignore them and still return correct
results.

---

## Phase 3 — Operability & ingestion (2 sprints / 4 weeks)

**Goal.** Make ChronosDB usable by someone who isn't the author. Harden
ingestion, metrics, backup, and docs; ship a demo.

**Scope.**
- Bulk importer hardening to sustain **500 MB/s**; resumable, idempotent
  by `(entity_id, valid_from)`.
- Simple single-node Kafka consumer with idempotent upserts and lag
  metrics. **Not** Kafka Connect — that is out of v1.0 scope.
- Full Prometheus coverage: QPS, p50/p99, shortcut hit rate, compaction
  lag, storage growth, Kafka lag.
- Snapshot backup + WAL-based restore, verified by chaos tests that kill
  the process during backup and during restore.
- Docker image + `docker-compose` demo with preloaded fraud dataset.
- Three end-to-end tutorials — compliance audit, SCD analytics,
  permission archaeology — each runnable by `make tutorial-<name>`.
- OpenAPI spec + proto doc comments + generated reference docs.
- Management console trimmed to three panels: live metrics, query editor
  with `EXPLAIN`, shortcut inspector.

**Exit criteria.**
- Importer holds 500 MB/s on reference hardware for ≥ 10 minutes.
- Kafka consumer survives producer restart and duplicate batches.
- Full Prometheus metric suite visible on default Grafana dashboard.
- Backup + restore chaos tests pass.
- All three tutorials run to completion from a fresh container.
- API reference docs generated and linked from README.

**Risks.** Docs drift. Guardrail: every public API surface change in a PR
must update the OpenAPI spec / proto comments in the same PR.

---

## Phase 4 — Benchmarks, hardening, v1.0 (2 sprints / 4 weeks)

**Goal.** Prove the worth publicly. Four benchmarks, one soak test, basic
security, `v1.0.0` tag.

**Scope.**
- Benchmark harness comparing ChronosDB to Neo4j with a standard
  temporal-workaround (versioned edges) on a 100M-edge synthetic graph.
- YCSB-temporal workload extension published for the ecosystem.
- Real-workload trace benchmark (synthetic fraud-investigation trace)
  showing positive shortcut ROI.
- 72-hour soak test: memory, goroutine count, compaction health, shortcut
  cache stability.
- TLS 1.3 on all ports; token auth; audit log of all queries; OWASP
  top-10 review of API surface.
- Release engineering: semver commitment, migration guide, changelog,
  `v1.0.0` tag. Launch blog post.

**Exit criteria.**
- Published benchmark report with reproduction instructions:
  1. ≥ **10×** faster `AS OF` traversal vs Neo4j on 100M-edge graph.
  2. PGI warm p99 **≥ 5×** better than cold p99 on YCSB-temporal.
  3. Storage overhead **≤ 2.5×** confirmed on real workload trace.
  4. Positive shortcut ROI on investigation workload trace.
- 72-hour soak test passes with no leak / stall / cache bloat.
- OWASP review clean; no high/critical findings in API layer.
- `v1.0.0` tagged with release notes and migration guide.

**Risks.** The Neo4j comparison is the most visible claim; biased
benchmarks backfire hard. Guardrail: publish the raw harness, the
workload, and the competitor configuration — no cherry-picking.

---

## Dependency graph (phase level)

```
Phase 0 ──► Phase 1 ──► Phase 2 ──► Phase 3 ──► Phase 4
                            │             │
                            └── needs correct ──┘
                                TKG + planner
```

No phase depends on unbuilt work from a later phase. If a phase's exit
criteria slip, the fix is to re-run the last sprint — not to skip forward.

## Cut lines (what we do *not* build, and when we might)

| Item                          | Reconsider when                           |
| ----------------------------- | ----------------------------------------- |
| Distributed mode / sharding   | Single-node has 10+ production users      |
| PAL / forecasting             | Likely never; separate product            |
| Multi-tenancy                 | Managed cloud offering exists             |
| LDAP / SSO                    | First paying enterprise asks              |
| GNN shortcut prediction       | Count-min sketch proven insufficient      |
| Kafka Connect plugin          | Simple consumer insufficient in practice  |
| SQL via Calcite               | Never — stay focused                      |
| Federated queries             | Never                                     |
