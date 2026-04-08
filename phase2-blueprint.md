# Phase 2 Blueprint: Predictive Graph Index + Distribution

**Sequence:** 2  
**Goal:** Add self‑optimization through query learning and enable horizontal scaling. PGI monitors workload, creates shortcuts for frequent patterns, and the system becomes distributed.

## Key Components
- **PGI Module**: Query log analysis, pattern detection, shortcut creation.
- **Cluster Coordination**: etcd for metadata, sharding, replication.
- **Sharded Query Execution**: Consistent hashing by node ID.
- **gRPC API**: High‑performance client interface.
- **Management Dashboard**: Web UI showing query patterns and shortcuts.

## Implementation Tasks
1. Implement PGI module:
   - Query log capture (normalized signatures, frequency, latency).
   - Pattern detection (frequent path patterns).
   - Shortcut creation (materialized edges) with cost‑benefit analysis.
   - Background updater for shortcuts.
2. Integrate PGI with query planner: optimizer uses shortcuts when beneficial.
3. Implement cluster coordination:
   - Use etcd for cluster state (nodes, shard assignments).
   - Implement node membership (join/leave, heartbeats).
   - Consistent hashing for sharding by node ID.
   - Replication (sync/async) with failover.
4. Extend query executor to handle distributed queries:
   - Coordinator splits query, sends to shards, merges results.
5. Add gRPC service with methods: `Execute`, `Import`, `Admin`.
6. Build simple web dashboard (React or similar) that displays:
   - Cluster health
   - Top queries and shortcuts
   - Query latency heatmap
7. Write integration tests for distribution and PGI.
8. Ensure PGI overhead <5% CPU.

## Verification Checklist
| # | Item | Status |
|---|------|--------|
| 1 | Query log captures normalized query signatures, frequency, and execution time. | |
| 2 | PGI automatically identifies top‑k frequent path patterns (e.g., `(A)-[r]->(B)`). | |
| 3 | When a pattern's estimated benefit exceeds threshold, a shortcut edge is materialized. | |
| 4 | Query optimizer uses shortcuts, resulting in ≥2× speedup for matching queries. | |
| 5 | Shortcuts are updated asynchronously when base data changes; staleness <5 minutes. | |
| 6 | Unused shortcuts are automatically dropped after N days. | |
| 7 | Cluster of 3 nodes can be started; data is sharded by consistent hashing. | |
| 8 | Query spanning multiple shards returns correct merged result (e.g., `MATCH (n) RETURN n`). | |
| 9 | Replication factor 2: writes acknowledged after replica confirms; failover works. | |
| 10 | gRPC clients in Python, Java, Go can execute queries and receive results. | |
| 11 | Management dashboard shows cluster health, top queries, shortcuts, and latency heatmap. | |
| 12 | PGI background processes consume <5% CPU under mixed workload. | |
| 13 | Admin API allows manual creation/deletion of shortcuts. | |

## Sprint Execution Prompt
Read: docs/architecture-guardrails.md, docs/phase1-blueprint.md (as reference), docs/phase2-blueprint.md  
Execute Sprint 2 — Predictive Graph Index + Distribution.  
Implement all components described. Use existing Phase 1 code as base.  
After implementation, run the verification checklist (13 items) and report score as X/13 PASS or FAIL with details.  
Commit to branch `phase2` with message `"feat: add PGI and distribution"`.
