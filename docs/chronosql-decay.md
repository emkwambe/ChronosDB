# ChronosQL `decay()` — Function Design

**Status:** Candidate design for Phase 5
([`phase5-temporal-value.md`](./phase5-temporal-value.md)). Not
implemented. Parser, planner, and executor do not recognize `decay()`
or `age()` today; committing them is a Phase 5 decision.

This document specifies the exact signature, semantics, planner
integration, and EXPLAIN surface so nothing in Phases 1–4 accidentally
prevents the feature from being added cleanly.

---

## 1. Signatures

```
age(r: RELATIONSHIP)            -> DURATION      -- current-time - r.valid_from
age(n: NODE)                    -> DURATION      -- current-time - n.valid_from
age(n.property)                 -> DURATION      -- current-time - property's valid_from
age(r, at: TIMESTAMP)           -> DURATION      -- at - r.valid_from, overriding query AS OF
decay(rate: FLOAT, age: DURATION) -> FLOAT       -- exp(-rate * age_seconds)
decay(rate: FLOAT, age: DURATION, floor: FLOAT) -> FLOAT
                                                 -- max(floor, exp(-rate * age_seconds))
```

**Current time** is the query's `AS OF` timestamp when one is present,
otherwise `now()` at plan-submission time (captured at the coordinator
so streaming results are stable). Never derived per-row at evaluation
time — that would break determinism.

`DURATION` is stored as microseconds internally (matching
`valid_from` / `valid_to`). The `age_seconds` conversion inside `decay`
is fixed at 1e6 µs → 1 s.

## 2. Semantics

### 2.1 Mathematical form

```
decay(λ, Δt) = e^(-λ · Δt_seconds)
```

Continuous exponential decay, one parameter, no alternatives in v1.
Rationale: matches finance's continuous discount convention, is
parameter-symmetric (half-life = ln(2)/λ), and is the only form the
TRL fits.

### 2.2 Boundary behavior (property tests in Phase 5)

| Input                                | Result                              |
| ------------------------------------ | ----------------------------------- |
| `decay(0, t)` for any t              | `1.0`                               |
| `decay(λ, 0)` for any λ              | `1.0`                               |
| `decay(λ, t)` for t → ∞              | `0.0` (monotonic)                   |
| `decay(NaN, t)` or `decay(λ, NaN)`   | `NaN` (propagates, does not error)  |
| `decay(-λ, t)` (negative rate)       | `> 1`, allowed, flagged in EXPLAIN  |
| `age(r)` where `r.valid_from > AS_OF`| negative DURATION; user responsibility |

### 2.3 `AUTO` rate

```
decay(AUTO, age(r))
```

Resolves at plan time using this precedence:

1. **Query override.** `WITH DECAY RATE 0.1 FOR <edge_type>` clause.
2. **Admin pin.** Rate written via admin API into the `meta` CF.
3. **TRL-learned.** Most recent fit for the edge type.
4. **Default.** `1 / (365 · 86400)` (one-year half-life-ish), configurable
   in `chronosdb.yaml`.

The resolved rate and its source are captured in the plan and surfaced
in EXPLAIN (§5).

## 3. Grammar additions

Minimal diff to the Phase 1 ChronosQL grammar:

```
query           = regular-cypher [ decay-clause ] [ temporal-clause ]
decay-clause    = "WITH" "DECAY" "RATE" number "FOR" rel-type
                  { "," number "FOR" rel-type }
temporal-clause = ( as already defined in Phase 1 )

function-call  =/ "decay" "(" expr "," expr [ "," expr ] ")"
function-call  =/ "age"   "(" ( variable | property-ref )
                             [ "," timestamp ] ")"

rel-type        = IDENTIFIER
number          = integer / float / "AUTO"
```

`AUTO` is a keyword only within `decay()`'s first argument.

## 4. Examples

### 4.1 Weighted neighbors

```
MATCH (c:Customer {id: 'cust_000042'})-[r:PURCHASED]->(p:Product)
RETURN p.name, decay(0.01, age(r)) AS weight
ORDER BY weight DESC
LIMIT 10;
```

Returns the customer's ten most currently-relevant purchases, decayed
at λ = 0.01/s.

### 4.2 Current-relevance score for a fraud investigation

```
MATCH (a:Account)-[t:TRANSFER]->(b:Account)
WHERE b.id = 'target_account'
AS OF '2026-04-20T14:00:00Z'
RETURN a.id, SUM(decay(AUTO, age(t)) * t.amount) AS current_exposure
ORDER BY current_exposure DESC;
```

Same query with `AS OF` backdated for a "what did this look like last
Tuesday?" investigation. TRL-learned λ for `TRANSFER` is used.

### 4.3 Scenario rerun with an explicit rate

```
WITH DECAY RATE 0.5 FOR TRANSFER
MATCH (a:Account)-[t:TRANSFER]->(b:Account)
WHERE b.id = 'target_account'
RETURN a.id, SUM(decay(AUTO, age(t)) * t.amount) AS current_exposure;
```

Same shape, aggressive decay. Used for stress-scenario analysis.

### 4.4 Yield-curve-style report

```
RETURN edge_decay_rates();
```

Returns a table of `(edge_type, λ, fit_quality, sample_size, last_fit_at)`
for every type TRL has seen. Server-side only; no client support needed.

## 5. `EXPLAIN` surface

Example `EXPLAIN` output fragment for example 4.2:

```
Project        [a.id, current_exposure]
└── Aggregate  [SUM(decay_weight * t.amount) AS current_exposure]
    └── Map    [decay_weight = decay(λ, Δt),
                λ      = 0.00317 (source: trl, fit_at=2026-04-18T03:00Z,
                                  sample=12840, quality=0.91),
                Δt     = age(t) from AS_OF=2026-04-20T14:00Z]
        └── TraversalScan [TRANSFER edges → target=target_account]
            (shortcut considered: TRANSFER→target_account:w30d,
             accepted: yes, estimated_benefit=46ms)
```

Every `decay()` call gets a plan annotation identifying:

- The resolved numeric rate.
- The source: `user` (literal), `query` (WITH clause), `admin` (pinned),
  `trl` (learned), or `default`.
- For `trl`: `fit_at`, `sample_size`, `quality`.

This is the user's window into *why the system decayed the way it did*.

## 6. Planner integration

### 6.1 Expression evaluation

`decay()` is a **pure scalar function**, evaluated row-by-row in the
executor. No special operator.

### 6.2 Push-down

When `decay()` appears inside an aggregation that is pre-computed by a
PGI shortcut, the shortcut's materialization must have used the *same*
rate. Rate mismatch disqualifies the shortcut for that query.

Implementation sketch: shortcuts carry a `decay_profile` tuple of
`(edge_type, λ)` pairs; planner accepts the shortcut only if every
`decay()` call in the query resolves to a rate in that tuple.

### 6.3 Determinism under AS OF

`current_time` is bound at plan-submission, not per-row. Two executions
of the same query at the same `AS OF` must return identical results
forever. This is the Phase 1 bitemporal-reproducibility guarantee
extended to TVA.

### 6.4 Performance

- `age()` is O(1): direct field lookup + one subtraction.
- `decay()` is one `math.Exp` call per row.
- On a 100-hop / 1M-row aggregation, decay evaluation is ~100M `Exp`
  calls — measurable (~1–2s single-threaded). If the benchmark fails
  the Phase 5 p99 < 100ms target, options in order of preference:
  1. Parallel evaluation across CPU cores (the executor is already
     pull-based Volcano; parallel variant is straightforward).
  2. Quantize age to the nearest second, cache `Exp(-λ·k)` for small k.
  3. Push the decay + aggregation into the storage scan (last resort;
     violates separation of concerns).

## 7. Storage / metadata

A new `meta` CF row per edge type:

```
meta:decay:<edge_type> → {
  "rate": 0.00317,
  "source": "trl" | "admin",
  "fit_at": "2026-04-18T03:00Z",
  "sample_size": 12840,
  "quality": 0.91
}
```

No other storage changes. No new column family.

## 8. Admin API

```
POST /v1/admin/decay-rates
Body: { "edge_type": "TRANSFER", "rate": 0.005 }
Response: 200 OK, previous = { source: "trl", rate: 0.00317 }

GET /v1/admin/decay-rates
Response: [{ edge_type, rate, source, fit_at?, sample_size?, quality? }, ...]

DELETE /v1/admin/decay-rates/{edge_type}
(revert to TRL-learned value)
```

Bearer-token auth required (same as Phase 4 security story).

## 9. Open questions (resolve before Sprint 5.1)

1. **Do we allow `decay()` on node properties?** Spec currently allows
   `age(n.property)` but not `decay()` of a property-only path. Phase 5
   Sprint 5.1 kickoff confirms.

2. **Half-life parameterization as an alternative?** Users might prefer
   `decay_half_life(30d, age(r))` over picking λ. Add as a parser sugar
   that rewrites to `decay(ln(2)/half_life_seconds, age(r))`? Deferred
   to Sprint 5.3 pending user feedback.

3. **Floor parameter default.** Current signature allows a three-arg
   form with floor. Should the floor default be 0.0 or a small ε (to
   avoid zero-weight paths disappearing from ORDER BY)? Lean toward 0.0
   for purity; document the ORDER BY gotcha in the tutorial.

4. **Rate freshness.** How stale is "too stale" for a TRL rate? Emit a
   metric (`chronosdb_decay_rate_staleness_seconds`) and let operators
   alert on it. No automatic re-fit in v1.

## 10. Non-goals (repeat for emphasis)

- No second decay form.
- No rate inference or forecasting.
- No stochastic rates or yield-curve term structure.
- No per-node or per-edge (instance-level) rates — rates are per-type.
- No support for `decay()` in write queries; it is read-only by
  construction.
