# Phase 1 Blueprint: Core Temporal Graph (MVP)

**Sequence:** 1  
**Goal:** Deliver a single‑node graph database with native valid‑time support, enabling time‑travel queries (`AS OF`). Establish the foundation for temporal storage and querying.

## Key Components
- **Storage Engine**: RocksDB with column families for current state and history.
- **Temporal Versioning**: Delta‑based storage with periodic snapshots.
- **Data Model**: Nodes and edges with time‑varying properties.
- **ChronosQL Parser**: Extend openCypher with `AS OF` and `BETWEEN`.
- **Query Executor**: Single‑node, no distribution.
- **REST API**: Basic CRUD and query endpoints.
- **Background Compactor**: Merges deltas into snapshots.

## Implementation Tasks
1. Set up project structure per guardrails.
2. Implement RocksDB wrapper with column families: `nodes_current`, `edges_current`, `nodes_history`, `edges_history`.
3. Implement temporal storage logic:
   - On write, store new version with valid time range.
   - On read, apply deltas to reconstruct state at a given timestamp.
4. Implement ChronosQL parser (based on openCypher) with `AS OF` and `BETWEEN` support.
5. Implement basic query executor that uses the storage engine.
6. Create REST API using `net/http` and gorilla/mux (or similar):
   - `POST /v1/db/{db}/query` for queries.
   - `POST /v1/db/{db}/import` for bulk import.
7. Implement background compactor that runs periodically to create snapshots.
8. Add basic authentication (API key via header).
9. Write integration tests for all features.
10. Ensure performance targets (50k writes/sec, point lookup <10ms).

## Verification Checklist
| # | Item | Status |
|---|------|--------|
| 1 | Node can be created with labels and multiple properties via API. | |
| 2 | Edge can be created between two nodes with a type and properties. | |
| 3 | Property values can be updated; previous values are retained with valid‑time ranges. | |
| 4 | `AS OF <timestamp>` query returns correct graph state at that moment (node/edge existence and property values). | |
| 5 | `BETWEEN <start> AND <end>` query returns all versions valid during the interval. | |
| 6 | Deletion of a node or edge is recorded as a version with end time (soft delete). | |
| 7 | Compaction process runs periodically (configurable) and merges deltas into a new base snapshot without data loss. | |
| 8 | REST API accepts ChronosQL in request body and returns JSON results. | |
| 9 | Bulk import of CSV/JSON with timestamps creates temporal nodes/edges correctly. | |
| 10 | Write throughput exceeds 50,000 operations/sec on SSD (measured with mixed create/update). | |
| 11 | Point lookup of node by ID (no temporal qualifier) returns current state in <10ms P99. | |
| 12 | After restart, all committed data is recovered (WAL replay works). | |
| 13 | Basic authentication (API key) is enforced on all endpoints. | |

## Sprint Execution Prompt
Read: docs/architecture-guardrails.md, docs/phase1-blueprint.md  
Execute Sprint 1 — Core Temporal Graph MVP.  
Follow the implementation tasks above. Create all necessary files in the appropriate directories as defined in guardrails.  
After implementation, run the verification checklist (13 items) and report score as X/13 PASS or FAIL with details.  
Commit to branch `phase1` with message `"feat: implement core temporal graph MVP"`.
