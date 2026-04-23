# ChronosDB Architecture Guardrails

These are the non-negotiables for ChronosDB v1.0. Any PR, sprint, or feature
that violates them gets rejected, full stop. Bringing a scope-guardrail item
back into scope requires a PRD amendment, not a quiet import.

---

## 1. Scope guardrails

- **Single-node only.** No distributed code, no sharding, no coordinator, no
  etcd. If it requires a second process, it is out.
- **Two subsystems, not three.** TKG (bitemporal storage + ChronosQL) and
  PGI (learned shortcuts). PAL / forecasting is deleted from the repo, not
  commented out.
- **No feature work without a failing test first.** Every capability ships
  with correctness tests; every perf claim ships with a benchmark.
- **No "v5.0 COMPLETE" language anywhere.** Versioning stays at `0.x` until
  all Phase 4 exit criteria are hit honestly.
- **Docs reflect code, not ambition.** README describes what runs today.
  Future plans live in `ROADMAP.md` and are labeled unimplemented.

## 2. Architectural guardrails

- **Append-only on the write path.** No in-place updates to history.
  `nodes_current` / `edges_current` column families are *derived* from
  history, never the source of truth.
- **PGI never blocks the write or read path.** Workload monitoring is
  sampled and lock-free; shortcut creation is a background job; shortcut
  eviction never invalidates in-flight queries.
- **Shortcuts are hints, not authority.** The planner must always be able
  to answer a query without shortcuts. A corrupted or stale shortcut
  degrades latency, never correctness.
- **Bitemporal correctness is load-bearing.** Every read must be
  reproducible given `(valid_time, transaction_time)`. No "eventually
  consistent" reads of history.
- **One storage engine (BadgerDB), one query language (ChronosQL), one wire
  protocol (gRPC with REST gateway).** No pluggable backends until v1
  ships.

## 3. Engineering guardrails

- **No dependency added without justification in the PR description.**
  `go.mod` stays tight.
- **Every exported type or function has a godoc line.** Internal packages
  can skip it.
- **CI must pass:** `go vet`, `staticcheck`, `go test -race ./...`, and the
  benchmark suite (regression gate at ±10%).
- **No files in repo root except:** `README.md`, `LICENSE`, `go.mod`,
  `go.sum`, `Makefile`, `Dockerfile`, `docker-compose.yml`, `.gitignore`,
  `ROADMAP.md`, `architecture-guardrails.md`. Everything else lives in a
  subdirectory.
- **PRs stay under 500 lines of diff** (excluding generated proto, vendored
  code, and test fixtures). Split bigger work.
- **One feature branch per sprint story.** No long-lived branches.

## 4. Data guardrails

- **Storage overhead ≤ 2.5× the equivalent current-state graph** for the
  reference dataset. Measured every sprint; regression blocks merge.
- **No silent data loss on compaction.** Compaction is transactional; a
  crash mid-compaction must be recoverable from WAL.
- **Shortcut storage is capped** (default 20% of base graph size).
  Eviction triggers when cap is hit, not only when shortcuts go stale.

## 5. Release guardrails

- **A phase is done only when all of its exit criteria are green.** Partial
  completion doesn't roll forward; the sprint that missed it re-runs.
- **Every phase ends with a public, reproducible benchmark report** emitted
  by `make bench`.

---

## Technology stack (v1.0)

| Concern       | Choice                                                |
| ------------- | ----------------------------------------------------- |
| Language      | Go 1.21+                                              |
| Storage       | BadgerDB v4 (pure-Go LSM)                             |
| APIs          | gRPC (primary) + REST via grpc-gateway (secondary)    |
| Serialization | Protocol Buffers                                      |
| Metrics       | Prometheus over HTTP                                  |
| Logging       | `log/slog` structured logging                         |
| Config        | YAML file + env-var overrides                         |

## Repository layout (v1.0)

```
/chronosdb/
├── cmd/
│   ├── chronosd/       # Single-node server binary (gRPC + REST)
│   ├── importer/       # Bulk CSV/JSON/SQL importer
│   ├── webserver/      # Static web UI backend
│   └── testclient/     # Integration test harness
├── internal/
│   ├── storage/
│   │   ├── core/       # BadgerDB wrapper, logical column families (key prefixes)
│   │   ├── temporal/   # Bitemporal versioning, MVCC, WAL
│   │   └── compaction/ # Snapshot + delta compaction (Phase 1)
│   ├── query/
│   │   ├── parser/     # ChronosQL parser (Phase 1)
│   │   ├── planner/    # Logical + physical planner (Phase 1-2)
│   │   └── executor/   # Pull-based execution engine
│   ├── pgi/            # Predictive Graph Index (Phase 2)
│   │   ├── monitor/    # Workload sampler
│   │   ├── pattern/    # Count-min sketch + EWMA pattern detector
│   │   └── shortcut/   # Cost-benefit manager + materialization
│   ├── api/
│   │   ├── grpc/       # gRPC service impl
│   │   └── rest/       # REST gateway
│   ├── ingest/         # Kafka consumer (Phase 3)
│   ├── metrics/        # Prometheus exporters
│   └── security/       # TLS, token auth, audit log (Phase 4)
├── pkg/
│   └── chronosql/      # Public AST types for embedders
├── proto/              # Protocol Buffer definitions
├── docs/               # PRD, TRD, blueprint, sprints, tutorials
├── deployments/        # Dockerfile, docker-compose, Prometheus, Grafana
├── scripts/            # Build + codegen scripts
├── test/
│   ├── e2e/            # End-to-end integration tests
│   ├── load/           # Load + benchmark harness
│   └── synthetic/      # Reference dataset generator
└── webui/              # Static management console
```

Directories **explicitly not present** in v1.0 and not to be re-introduced
without a PRD amendment: `internal/cluster/`, `internal/pal/`,
`internal/multitenancy/`, `internal/streaming/` (replaced by `internal/ingest/`).

## 6. Development workflow

- One feature branch per sprint story, named `phaseN/sprintM-short-slug`.
- PR description includes: story ID, exit-criteria impact, benchmark delta.
- Merges require: green CI + one reviewer + benchmark regression ≤10%.
- Every phase closes with a tagged release candidate
  (`v0.N.0-rc.M`) and a published benchmark report.
