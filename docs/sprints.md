# ChronosDB Sprint Plan

Each sprint is 2 weeks, one owner, one feature branch, one merged PR at
the end. Sprint stories are sized to fit comfortably in that window. If a
story overruns, the sprint re-runs; it does not flip to "done".

See [`blueprint.md`](./blueprint.md) for the phase-level contract and
[`../architecture-guardrails.md`](../architecture-guardrails.md) for the
rules every sprint must satisfy.

---

## Phase 0 — Sprint 0.1: "Honest baseline"

| ID      | Story                                                                                                                            |
| ------- | -------------------------------------------------------------------------------------------------------------------------------- |
| S0.1.1  | Delete overclaims: `predictive/`, `multitenancy/`, `streaming/`, `kafka_producer/`, `tenantctl/`, root PDFs, `temp_method.txt`. |
| S0.1.2  | Rewrite README, ROADMAP, architecture-guardrails, PRD, TRD. Version → `0.1.0-dev`. Archive "v5.0" release language.              |
| S0.1.3  | CI: `go vet` + `staticcheck` + `go test -race` + coverage gate 70%. Baseline benchmark emitted as JSON artifact.                 |
| S0.1.4  | Reference workload: synthetic bitemporal graph generator (10M edges, seeded, deterministic) + ground-truth query set.            |
| S0.1.5  | `make bench` harness runs baseline, emits JSON, plotted in README.                                                               |

**Sprint exit:** CI green; bench baseline committed; tree contains only
what the README says it does.

---

## Phase 1 — Sprint 1.1: "Storage correctness"

| ID      | Story                                                                                                                 |
| ------- | --------------------------------------------------------------------------------------------------------------------- |
| S1.1.1  | Lock down key encoding (`{partition}:node:{id}:{valid_from}:{txn_id}`); document invariants in `internal/storage/core`. |
| S1.1.2  | MVCC snapshot isolation: transaction manager, read timestamps, write conflict detection, regression tests.           |
| S1.1.3  | Group-commit WAL; configurable fsync policy; crash-recovery test kills process mid-commit.                           |
| S1.1.4  | Bitemporal correctness suite: 50+ tests covering valid-time overlaps, txn-time monotonicity, point-in-time replay.   |

**Sprint exit:** MVCC + WAL pass all new tests under `-race`.

## Phase 1 — Sprint 1.2: "Temporal query execution"

| ID      | Story                                                                                                         |
| ------- | ------------------------------------------------------------------------------------------------------------- |
| S1.2.1  | ChronosQL parser: `AS OF`, `BETWEEN`, `HISTORY`, `AT`, `duration.between()`, `overlaps()`. AST + unit tests. |
| S1.2.2  | Logical planner with `TimeSlice`, `TemporalJoin`, `TemporalAggregate` operators.                             |
| S1.2.3  | Physical executor: Volcano-style, pull-based, streaming results. Integration with gRPC stream.                |
| S1.2.4  | Integration tests: every use-case query from `PRD.md §5` runs and returns correct results on reference data.  |

**Sprint exit:** Every PRD use-case query runs and returns ground-truth
results on the reference dataset.

## Phase 1 — Sprint 1.3: "Compaction & performance"

| ID      | Story                                                                                                |
| ------- | ---------------------------------------------------------------------------------------------------- |
| S1.3.1  | Snapshot+delta compaction: background job, configurable interval, crash-safe with kill-during test.  |
| S1.3.2  | Time-partitioned buckets: cold partitions become read-only; planner prunes by time bounds.           |
| S1.3.3  | Performance tuning to hit Phase 1 exit numbers (10ms point, 50ms single-hop, 50k writes/sec, 2.5×).  |
| S1.3.4  | Phase 1 benchmark report (JSON + README plot).                                                       |

**Sprint exit:** All Phase 1 exit-criteria numbers green; benchmark report
published.

---

## Phase 2 — Sprint 2.1: "Workload monitor"

| ID      | Story                                                                                                      |
| ------- | ---------------------------------------------------------------------------------------------------------- |
| S2.1.1  | Query-signature normalization (shape-preserving, literal-stripping). Deterministic hashing.                |
| S2.1.2  | Count-min sketch for frequency + EWMA for recency; per-signature latency histogram.                        |
| S2.1.3  | Sampling strategy: 10% default, tunable; lock-free ring buffer shared with executor.                       |
| S2.1.4  | Overhead benchmark: monitor CPU overhead must stay **<1%** under sustained load.                           |

**Sprint exit:** Monitor runs in production-like load with measurable,
sub-1% overhead.

## Phase 2 — Sprint 2.2: "Shortcut manager"

| ID      | Story                                                                                                                |
| ------- | -------------------------------------------------------------------------------------------------------------------- |
| S2.2.1  | Pattern detector: extract frequent `(A)-[r]->(B)` and 2-hop motifs from signatures.                                  |
| S2.2.2  | Cost-benefit scorer: `benefit = saved_latency × freq`, `cost = storage + write_amp`; threshold config.               |
| S2.2.3  | Shortcut materialization job: writes into `shortcuts` CF with validity window + provenance metadata.                 |
| S2.2.4  | Shortcut invalidation on base-data change: async, bounded staleness, never blocks writes. Correctness tests.         |

**Sprint exit:** Shortcuts create themselves for the reference workload's
frequent motifs; writes are not blocked by invalidation.

## Phase 2 — Sprint 2.3: "Planner integration & eviction"

| ID      | Story                                                                                                |
| ------- | ---------------------------------------------------------------------------------------------------- |
| S2.3.1  | Planner hook: treat shortcuts as alternative access paths with selectivity estimates.                 |
| S2.3.2  | `EXPLAIN` output: shortcut usage, estimated vs actual benefit, shortcut provenance.                   |
| S2.3.3  | Eviction: TTL + LRU + cap-based; storage cap enforced at 20% of base graph.                           |
| S2.3.4  | Phase 2 benchmark: self-improvement curve (cold → warm p99) published.                                |

**Sprint exit:** Warm p99 ≥ 5× cold p99 on YCSB-temporal; eviction holds
the cap under churn.

---

## Phase 3 — Sprint 3.1: "Ingestion & operability"

| ID      | Story                                                                                                              |
| ------- | ------------------------------------------------------------------------------------------------------------------ |
| S3.1.1  | Bulk importer hardening: 500 MB/s sustained, resumable, idempotent by `(entity_id, valid_from)`.                   |
| S3.1.2  | Kafka consumer (simple, single-node) with idempotent upserts and lag metrics. **No Kafka Connect.**                |
| S3.1.3  | Prometheus metrics: full coverage (QPS, latency, shortcut hit rate, compaction lag, storage growth, Kafka lag).    |
| S3.1.4  | Snapshot backup + WAL restore; chaos test kills during each step.                                                  |

**Sprint exit:** Importer + Kafka + metrics + backup/restore all survive a
chaos run.

## Phase 3 — Sprint 3.2: "Docs & demos"

| ID      | Story                                                                                                      |
| ------- | ---------------------------------------------------------------------------------------------------------- |
| S3.2.1  | OpenAPI spec + proto doc comments + generated reference docs linked from README.                            |
| S3.2.2  | Three tutorials runnable with `make tutorial-compliance`, `tutorial-scd`, `tutorial-permissions`.           |
| S3.2.3  | Docker image + docker-compose demo with preloaded fraud dataset; 2-minute walkthrough script.              |
| S3.2.4  | Management console trimmed to three panels: live metrics, query editor with EXPLAIN, shortcut inspector.    |

**Sprint exit:** A new engineer can clone, `docker-compose up`, and run
all three tutorials without asking for help.

---

## Phase 4 — Sprint 4.1: "Public benchmarks"

| ID      | Story                                                                                                     |
| ------- | --------------------------------------------------------------------------------------------------------- |
| S4.1.1  | Neo4j comparison harness: same queries, same data, apples-to-apples. Publish raw harness + results.       |
| S4.1.2  | YCSB-temporal extension published for the ecosystem.                                                      |
| S4.1.3  | Real-workload trace benchmark (synthetic fraud-investigation trace) showing shortcut ROI.                 |
| S4.1.4  | Benchmark report draft: four results + reproduction instructions + honest limitations section.            |

**Sprint exit:** Reviewer outside the team can reproduce the published
numbers from the harness alone.

## Phase 4 — Sprint 4.2: "Hardening & v1.0"

| ID      | Story                                                                                                       |
| ------- | ----------------------------------------------------------------------------------------------------------- |
| S4.2.1  | 72-hour soak test: memory, goroutine count, compaction health, shortcut cache stability.                     |
| S4.2.2  | Security basics: TLS 1.3, token auth, audit log of queries, OWASP review of API layer.                       |
| S4.2.3  | Release engineering: semver commitment, migration guide, changelog, `v1.0.0` tag.                            |
| S4.2.4  | Launch: blog post + HN post + outreach to three prospective users in compliance / fraud / SCD segments.      |

**Sprint exit:** `v1.0.0` tagged and announced; issue tracker ready for
post-launch traffic.

---

## Sprint ceremonies (lightweight)

- **Kickoff (Monday week 1):** review story list, confirm owner, name the
  feature branch.
- **Mid-sprint check (Wednesday week 2):** green/yellow/red per story;
  escalate blockers to scope trim *now*, not Friday.
- **Close (Friday week 2):** PR merged, CI green, benchmark delta
  recorded, phase-level exit criteria updated.

## What explicitly is *not* a sprint deliverable

- "Integration with X" that requires X we do not have
- "Production readiness" without a benchmark to prove it
- "Refactoring" unless a specific sprint story names it
- Design documents beyond what the blueprint and this plan already define
