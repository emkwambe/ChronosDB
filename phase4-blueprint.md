# Phase 4 Blueprint: Predictive Analytics Layer (PAL)

**Sequence:** 4  
**Goal:** Enable forecasting of future graph states—properties, relationships, and structure—using machine learning models trained on temporal data.

## Key Components
- **Model Repository**: Versioned storage of models (ONNX, PMML, native).
- **Training Pipeline**: Scheduled retraining with backtesting.
- **Inference Engine**: Low‑latency forecasting with confidence intervals.
- **Feature Store**: Cached time‑series aggregates.
- **External Parameter Injection**: For what‑if scenarios.
- **Explanation Generator**: Feature importance, SHAP.
- **PGI Integration**: Pre‑create shortcuts based on forecasts.

## Implementation Tasks
1. Design model repository:
   - Store model metadata (type, features, training period, metrics).
   - Support ONNX runtime for cross‑language models.
2. Implement training pipeline:
   - Periodically read historical data from TKG.
   - Compute features (rolling averages, seasonality).
   - Train models (ARIMA, Prophet, LSTM) using external libraries.
   - Save model and metadata.
3. Implement inference engine:
   - Load model on demand, cache in memory.
   - Accept forecast requests with optional external params.
   - Return point estimate + confidence intervals.
4. Build feature store:
   - Pre‑compute and cache common features.
   - Update incrementally as new data arrives.
5. Extend ChronosQL with `FORECAST` clause:
   - Syntax: `FORECAST property OVER duration [GIVEN {...}] [WITH CONFIDENCE n]`.
   - Support for relationship probability.
6. Add explanation capability: `EXPLAIN FORECAST` returning top factors.
7. Integrate with PGI:
   - PGI subscribes to forecast results for high‑value predictions.
   - Pre‑create shortcuts for likely future queries.
8. Implement security: model access tied to data permissions.
9. Write tests for forecast accuracy and latency.

## Verification Checklist
| # | Item | Status |
|---|------|--------|
| 1 | `FORECAST property OVER duration` returns a point estimate. | |
| 2 | Forecast can include confidence intervals (e.g., `WITH CONFIDENCE 0.95`). | |
| 3 | External parameters (`GIVEN`) influence forecast (e.g., interest rate). | |
| 4 | Relationship existence probability can be forecast (e.g., `FORECAST r.probability`). | |
| 5 | Models are automatically retrained daily (configurable) on latest data; new version stored. | |
| 6 | Multiple model types (ARIMA, Prophet, LSTM, GNN) are available and switchable. | |
| 7 | Feature store pre‑computes rolling averages and caches them; inference uses cache for speed. | |
| 8 | Forecast latency for single entity <200ms P99. | |
| 9 | Batch forecast for 10,000 entities completes within 5 seconds per node. | |
| 10 | Explanation of forecast (top contributing factors) can be retrieved via `EXPLAIN FORECAST`. | |
| 11 | Models are versioned; rollback to previous version supported. | |
| 12 | Security: model access restricted to users with data access. | |
| 13 | Integration with PGI: PGI can automatically create shortcuts for predicted future queries. | |
| 14 | Forecast accuracy can be evaluated via backtesting against withheld historical data (tooling provided). | |

## Sprint Execution Prompt
Read: docs/architecture-guardrails.md, docs/phase3-blueprint.md (as reference), docs/phase4-blueprint.md  
Execute Sprint 4 — Predictive Analytics Layer.  
Implement all PAL components. Build on Phase 3 codebase.  
After implementation, run the verification checklist (14 items) and report score as X/14 PASS or FAIL with details.  
Commit to branch `phase4` with message `"feat: add predictive analytics layer"`.
