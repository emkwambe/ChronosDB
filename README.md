# ChronosDB

> A single-node temporal graph database that gets faster the more you query it.

ChronosDB stores nodes, relationships, and properties with full bitemporal
semantics (valid time + transaction time) and continuously learns from query
workload to materialize shortcut edges that accelerate recurring queries.

**Version:** `0.1.0-dev`
**Status:** pre-alpha. Phase 0 (repo hygiene) in progress. See
[`ROADMAP.md`](./ROADMAP.md) for the honest state of each subsystem.

---

## What actually works today

- Bitemporal graph storage on BadgerDB (column families: `nodes_current`,
  `edges_current`, `nodes_history`, `edges_history`)
- `AS OF <timestamp>` and `BETWEEN <t1> AND <t2>` queries (parser + executor)
- Bulk import from CSV / JSON / SQL with `valid_from` / `valid_to` columns
- gRPC server (`:50051`) and REST gateway (`:8080`) on a single process
- Basic web UI for browsing and running queries (`cmd/webserver`, port `8081`)
- Prometheus metrics endpoint (partial coverage)

## What does NOT work yet

- **Predictive Graph Index (PGI).** The core differentiator. Scheduled for
  Phase 2 — see [`docs/blueprint.md`](./docs/blueprint.md).
- **Snapshot isolation under concurrent writers.** MVCC is not yet proven.
- **Crash-safe compaction.** Tested under happy paths only.
- **Streaming ingestion, multi-tenancy, distribution, forecasting.** Removed
  from the codebase in Phase 0. Some are scheduled for later phases; others
  are permanently out of scope.

If you see language elsewhere claiming these work, it is stale and being
cleaned up sprint by sprint.

---

## Quick start

```bash
# Build
make build

# Run server
./bin/chronosd -data-dir=./data

# In another shell: import sample data and query it
make import-all
make query-sample
```

Docker:

```bash
make docker-build && make docker-up
```

## Development

Quality gates (must pass before merge — see
[`architecture-guardrails.md`](./architecture-guardrails.md)):

```bash
make vet
make lint        # requires staticcheck
make test-race
make bench
```

## Documentation

- [`ROADMAP.md`](./ROADMAP.md) — phased delivery plan with exit criteria
- [`architecture-guardrails.md`](./architecture-guardrails.md) —
  non-negotiable scope/architecture/engineering rules
- [`docs/PRD.md`](./docs/PRD.md) — product requirements (rescoped)
- [`docs/TRD.md`](./docs/TRD.md) — technical requirements (rescoped)
- [`docs/blueprint.md`](./docs/blueprint.md) — 4-phase development blueprint
- [`docs/sprints.md`](./docs/sprints.md) — sprint-by-sprint story breakdown

## License

See [`LICENSE`](./LICENSE).
