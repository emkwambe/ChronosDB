# Phase 5 (Candidate, Post-v1.0) — Temporal Value Analytics

**Status:** Candidate. **Not** part of the v1.0 commitment in
[`ROADMAP.md`](../ROADMAP.md). This document exists so the architectural
foundation laid in Phases 1–4 does not accidentally foreclose this
extension — and so the extension does not sneak back into v1.0 scope.

Promotion from "candidate" to "committed" requires: (a) Phase 4 exit
criteria green; (b) a signed-off PRD amendment; (c) a paying or
sufficiently-interested user for at least two of the listed use cases.

**Inspiration.** Finance has a single, universally-used primitive —
*time value of money* (TVM) — that says a future cashflow is worth less
than a present one, at some discount rate. We believe the same is true
for relationships: a friendship from eight years ago is not worth the
same as one from last week; a supplier edge active in 2019 is not a
reliable predictor of 2026 behavior. Temporal Value Analytics (TVA) makes
this a first-class database primitive. The analogy is *suggestive*, not
rigorous — see the honesty clause below.

See the companion function design:
[`chronosql-decay.md`](./chronosql-decay.md).

---

## 1. Thesis

Every fact in the graph has an age at query time. Most questions users
actually want to ask ("who is the customer's closest influence *now*?",
"what fraud ring is *currently* active?") are answered better by
age-weighted aggregations than by hard time filters. Today those answers
require hand-rolled decay math in application code, which:

1. Spreads the decay logic across many clients inconsistently.
2. Can't benefit from PGI shortcuts (the DB can't reason about code it
   never sees).
3. Prevents apples-to-apples comparison between queries at different
   discount rates.

A built-in `decay()` function, learned per-edge-type decay rates, and a
scenario-rerun clause put all three problems in the database.

## 2. Scope — what ships

| # | Capability                                                         | Owner          |
| - | ------------------------------------------------------------------ | -------------- |
| 1 | `decay(rate, age)` built-in scalar function in ChronosQL           | Query engine   |
| 2 | `age(r)` and `age(n.prop)` helpers resolving against `AS OF` time  | Query engine   |
| 3 | Weighted aggregations, e.g. `SUM(decay(λ, age(r)) * r.amount)`     | Query engine   |
| 4 | Temporal Relevance Learner (TRL) — per-edge-type λ fit             | TRL (new)      |
| 5 | `WITH DECAY RATE <λ> FOR <edge_type>` query prefix (override)      | Parser         |
| 6 | `EXPLAIN` surfaces rate source (user / admin / learned / default)  | Planner        |
| 7 | Admin API to pin / override λ per edge type                        | REST + gRPC    |
| 8 | "Decay yield curve" report (λ vs edge type + age distribution)     | Metrics report |

## 3. Scope — what does **not** ship

These are expressly cut. Re-adding requires a new PRD amendment, not
a Phase 5 sprint slip.

- Multi-parameter decay (double exponential, Weibull, gamma). One
  parameter per edge type, exponential form. If data needs a different
  shape, that's Phase 6 material.
- Term-structure models, stochastic decay rates, or any "rate-of-change-
  of-rates" concept. Rates change only when retrained.
- Decay on **node** properties beyond a single time field per query.
- Any ML model more sophisticated than least-squares fit of a single λ.
  **No PAL revival under a new name.**
- Automatic rate selection based on query intent. If the user doesn't
  specify a rate, TRL's last-fit is used. No inference, no forecasts.

## 4. Why this fits the existing architecture (no new subsystem)

TVA reuses plumbing Phases 1–4 already build:

| TVA piece              | Reused from      | Why it fits                                                        |
| ---------------------- | ---------------- | ------------------------------------------------------------------ |
| `decay()` evaluation   | Phase 1 executor | Pure scalar over row; Volcano executor handles it for free.        |
| `age()` helper         | Phase 1 storage  | Every record already has `valid_from`; `AS OF` time is in context. |
| Workload observation   | Phase 2 PGI      | Same sampling ring buffer; TRL consumes a different projection.    |
| Lightweight model fit  | Phase 2 PGI      | PGI fits count-min + EWMA; TRL fits single-parameter exponential.  |
| Metadata store         | Phase 1 `meta`   | λ per edge type is a tiny key-value row.                           |
| EXPLAIN integration    | Phase 2 planner  | One more field ("rate_source") in the plan tree.                   |
| Shortcut compatibility | Phase 2 PGI      | Shortcuts store pre-aggregated paths; decay-weighted shortcuts are |
|                        |                  | a natural extension (optional — see sprint 5.3).                   |

**There is no new subsystem.** TVA is a query-language feature plus a
TRL module that is a near-copy of PGI's pattern-detector skeleton.

## 5. Exit criteria (Phase 5)

- [ ] `decay(rate, age)` parses, plans, and executes correctly; null-safe,
      NaN-safe, monotonic in age (property tests).
- [ ] `age(r)` and `age(n.prop)` resolve under `AS OF <t>`, `BETWEEN`,
      and unbounded queries. Property test: `age(r)` at `t` equals
      `t - r.valid_from`.
- [ ] TRL fits a per-edge-type λ from observed history; fit converges
      within 100ms on reference dataset.
- [ ] `WITH DECAY RATE 0.1 FOR PURCHASED` overrides TRL for the query.
- [ ] `EXPLAIN` surfaces the rate source for every use of `decay()`.
- [ ] **Correctness guardrail:** for any query Q, the result of Q using
      a PGI shortcut is identical to the result of Q without shortcuts,
      including all `decay()`-weighted aggregations. (Same invariant as
      Phase 2; TVA must not weaken it.)
- [ ] Benchmark: decay-weighted 2-hop traversal over 1M edges completes
      in p99 < 100ms on reference hardware.
- [ ] Admin API returns the "decay yield curve" — a JSON report of
      `(edge_type, λ, fit_quality, sample_size)` across the graph.
- [ ] One end-to-end tutorial — "current-relevance fraud investigation"
      — runs from a fresh container.

## 6. Sprint sketch (4 sprints, 8 weeks)

| Sprint | Title                        | Key stories                                                              |
| ------ | ---------------------------- | ------------------------------------------------------------------------ |
| 5.1    | `decay()` and `age()`        | Parser, planner expression, executor, property tests, docs.              |
| 5.2    | Temporal Relevance Learner   | Sampling projection, single-parameter fit, persistence to `meta` CF.     |
| 5.3    | Overrides + EXPLAIN + admin  | `WITH DECAY RATE` clause, EXPLAIN surface, admin pin API, yield report.  |
| 5.4    | Benchmarks + tutorial        | Perf targets; "current-relevance" tutorial; Phase 5 release notes.       |

Each sprint is 2 weeks, same ceremony as Phases 0–4.

## 7. Risks and mitigations

- **Scope creep back into full PAL.** Mitigated by the explicit cut-list
  in §3 and by keeping TRL to a single-parameter exponential fit. Any
  PR that introduces a second parameter, a second model, or a training
  pipeline heavier than least-squares is rejected.

- **Over-selling the TVM analogy.** We name the feature "Temporal Value
  Analytics," reference TVM in the launch blog, but the docs and API use
  neutral terms (*decay*, *rate*, *age*, *relevance*). No promises of
  arbitrage-free pricing or probabilistic guarantees.

- **Correctness under shortcuts.** The Phase 2 guardrail ("shortcuts are
  hints; planner must always be able to answer without them") extends to
  decay-weighted queries. Property tests must confirm: for every shape of
  decay query, `result(Q with shortcuts) == result(Q without)`.

- **Performance on large traversals.** Decay evaluation is O(rows); a
  100-hop query over 1M edges is 100M evaluations. Mitigation: push the
  `decay()` down into the scan operator where possible; cache `age()`
  within a single query plan.

- **User confusion about what λ means.** Docs must ship with a
  concrete recipe: "pick λ such that `decay(λ, half-life) = 0.5` — for a
  half-life of 30 days, λ ≈ 0.023 / day." Avoid hand-wavy "tune it."

## 8. Honesty clause

The TVM analogy is a **framing**, not a theorem. Finance's discount rates
are grounded in arbitrage-free pricing; graph-relationship decay rates
have no equivalent foundation. They are empirical fits over observed
data. Publications, marketing, and API docs must state this plainly.
The value of TVA lies in consistency, queryability, and a shared
vocabulary — **not** in a claim of mathematical rigor transplanted from
finance.

## 9. Promotion criteria (candidate → committed)

1. Phase 4 exit criteria all green.
2. Signed PRD amendment covering §§1–8 of this document.
3. At least **two** of: compliance, fraud, MLOps feature store, incident
   forensics, SCD analytics — have confirmed willingness to adopt TVA in
   pilot.
4. No active work is in-flight on a competing v1.1 feature.

Until all four are met, this is a design sketch. The architecture
guarantees it remains *possible* to build; it does not promise that it
will be.
