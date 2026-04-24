# Storage Invariants

These invariants are load-bearing. Any code in `internal/storage/`,
`internal/query/`, or `internal/pgi/` may assume they hold. A violation
is a bug — never a configuration option.

Every invariant here is covered by a test in
`internal/storage/core/keys_test.go` or
`internal/storage/temporal/bitemporal_test.go`. If a test is skipped,
the skip references the sprint that closes the gap.

---

## I1 — Single source of truth

History is authoritative. Current-state column families (`nc:`, `ec:`)
are **derived** from history and may always be rebuilt by replaying
history from the beginning. A reader that needs certainty reads
history. A reader that needs speed reads current-state.

Corollary: compaction never rewrites history. Compaction may rewrite
current-state from history, but history rows are append-only and
immutable once written.

## I2 — Identifier restrictions

Node IDs, edge IDs, property names, label names, and edge types must
not contain the reserved separator byte `:` (0x3A). This is enforced
at encode time by `validateID` in `keys.go`. Violating IDs produce
`ErrInvalidID` at write time.

Rationale: the alternative (escape sequences) introduces ambiguity in
range scans and requires parsing on every key. The separator choice is
stable for v1.0.

## I3 — Timestamp encoding

All `valid_from`, `valid_to`, and transaction-time stamps are
microseconds since Unix epoch, stored as big-endian int64. Big-endian
was chosen so byte-order comparison matches numeric order under
Badger's lexicographic iteration.

`valid_to == 0` means "unbounded future". No other sentinel values are
reserved. Code that distinguishes "unbounded" from "closed" checks
`valid_to == 0` explicitly; do not rely on implicit comparisons.

## I4 — Valid-time semantics: half-open interval

A version is live at time `t` iff:

    valid_from <= t < valid_to    (when valid_to != 0)
    valid_from <= t               (when valid_to == 0)

Valid-time is **inclusive at the lower bound, exclusive at the upper
bound**. This matches standard interval-arithmetic conventions and
avoids ambiguity at transition points.

**Current code:** `temporal.GetNodeAsOf` uses `valid_from <= t <=
valid_to` (both inclusive). This is an inconsistency to be fixed in
Sprint 1.2; tests in the bitemporal suite that rely on the correct
half-open convention are skipped with a reference to S1.2.

## I5 — Transaction-time monotonicity

For a given logical entity (node ID or edge ID), successive writes
receive strictly increasing `txn_id` values. Transaction IDs are
globally monotonic; no two concurrent transactions share a txn_id.

**Current code:** `temporal.go` does not track txn IDs. Transaction
management lands in Sprint 1.2. Until then, txn_id is supplied by the
caller and the tests use explicit values.

## I6 — Version overlap at most one per entity per point

For a given node ID (or edge ID), at most one history row is live at
any instant in valid-time. Writes must close out the prior version
(setting `valid_to = new_version.valid_from`) before opening a new
one.

This is the bitemporal correctness property most likely to regress on
concurrent writes. Enforcement is the transaction manager's job
(Sprint 1.2).

## I7 — Key ordering

Within a column family, keys sort lexicographically. Because `id` is
encoded as raw bytes and timestamps as big-endian int64, iteration
under a prefix `nh:<id>:` yields versions of that entity in
(valid_from, txn_id) order — which is the natural bitemporal replay
order.

History-key layout (for reference):

    nh:<id>:<valid_from_be8>:<txn_id_be8>
    eh:<id>:<valid_from_be8>:<txn_id_be8>
    ph:<entity_id>:<prop>:<valid_from_be8>:<txn_id_be8>
    sc:<pattern>:<src>:<dst>:<valid_from_be8>:<txn_id_be8>

The CF prefix is two characters plus `:`. The trailing timestamp and
txn_id are fixed-width 8 bytes each, so key length is predictable to
within the variable ID portion.

## I8 — Reproducibility under `AS OF`

For any query Q with `AS OF <t>` and any later writes W made after Q
returned, re-running Q with the same `AS OF <t>` must return the
identical result.

This is the bitemporal-reproducibility guarantee. It is the single
most important invariant for compliance, audit, SCD, and investigation
use cases. Every feature — including PGI shortcuts (Phase 2) and TVA
decay weighting (Phase 5 candidate) — must preserve it.

## I9 — Soft delete does not destroy history

A soft-deleted node or edge is marked `Deleted = true` in a new
history row with `valid_from = delete_time`. Prior history remains
queryable. `GetNodeAsOf(id, t)` for `t < delete_time` must return the
pre-delete state.

**Current code:** `temporal.SoftDeleteNode` writes a new current-state
row and one history row, which is almost right — the history for
pre-delete times must still be reachable via `nh:<id>:` prefix scan
(Sprint 1.2 wires this up).

## I10 — Shortcut correctness subordination

Shortcut rows in `sc:` are *hints*. For every query Q, the planner
must always be able to answer Q correctly without shortcuts. A
corrupted, stale, or missing shortcut may degrade latency but must
never change the result set.

Formally: `result(Q with shortcuts) == result(Q without shortcuts)`
for every query Q at every `AS OF` time.

This invariant is also written into the Phase 2 guardrail in
[`../../architecture-guardrails.md`](../../../architecture-guardrails.md)
and re-asserted in the Phase 5 candidate design.

---

## Summary table

| Invariant | Subject                                           | Status today                    | Closed by   |
| --------- | ------------------------------------------------- | ------------------------------- | ----------- |
| I1        | History is the source of truth                     | Partial (no compaction yet)     | S1.3.1      |
| I2        | ID character restrictions                          | Enforced at encode              | S1.1.1 ✓    |
| I3        | Timestamp encoding (microseconds, BE int64)        | Enforced at encode              | S1.1.1 ✓    |
| I4        | Half-open valid-time                               | Broken in GetNodeAsOf           | S1.2        |
| I5        | Monotonic txn IDs                                  | Not tracked                     | S1.1.2      |
| I6        | At most one live version per entity per instant   | Not enforced                    | S1.1.2      |
| I7        | Lexicographic key ordering = bitemporal order      | Enforced at encode              | S1.1.1 ✓    |
| I8        | AS OF reproducibility                              | Partial                         | S1.2        |
| I9        | Soft delete preserves history                      | Partial                         | S1.2        |
| I10       | Shortcut correctness subordinate to from-scratch   | N/A (no shortcuts yet)          | Phase 2     |
