# Performance Validation

## Goal

Prove million-scale behavior with repeatable load and soak tests, not assumptions.

Validation should cover:

- latency SLOs (p95/p99)
- error rate under sustained load
- queue depth/backlog behavior
- DB row/size growth during soak
- dropped statistics budget

## Test matrix

| Suite | Script | Default duration | Primary focus | Gate criteria |
| --- | --- | --- | --- | --- |
| Read-heavy | `scripts/perf/k6/read-path.js` | `30m` | proxy pages, filters, dashboard, GraphQL, proxy statistics reads | per-scenario thresholds in script (p95/p99 + error rate) |
| Write-heavy | `scripts/perf/k6/write-path.js` | `30m` | sustained `POST /api/addProxies` ingestion | per-scenario thresholds in script (p95/p99 + error rate) |
| Mixed soak | `scripts/perf/k6/mixed-soak.js` | `2h` (set to `24h`/`72h` for release proof) | long-running mixed reads + writes | per-scenario thresholds in script + growth/drop budgets below |
| Growth budget | `scripts/perf/assert-snapshot-delta.sh` | N/A | queue and DB growth across test window | queue/db deltas within configured limits |
| Drop budget | `scripts/perf/check-drop-budget.sh` | N/A | proxy statistics queue drop protection | `dropped_total` must be less than or equal to configured budget |

## Release gate procedure

Run from repo root:

```bash
cd scripts/perf
./run-gate.sh
```

For full soak proof:

```bash
cd scripts/perf
PERF_SOAK_DURATION=24h ./run-gate.sh
```

For extended soak:

```bash
cd scripts/perf
PERF_SOAK_DURATION=72h ./run-gate.sh
```

## Default growth/drop budgets

Configured in `scripts/perf/assert-snapshot-delta.sh` and `scripts/perf/check-drop-budget.sh`:

- `PERF_MAX_PROXY_QUEUE_DEPTH_DELTA=50000`
- `PERF_MAX_SCRAPESITE_QUEUE_DEPTH_DELTA=5000`
- `PERF_MAX_PROXY_STATISTICS_ROW_DELTA=25000000`
- `PERF_MAX_PROXY_STATISTICS_RESPONSE_ROWS_DELTA=250000`
- `PERF_MAX_PROXY_STATISTICS_BYTES_DELTA=16106127360`
- `PERF_MAX_DATABASE_BYTES_DELTA=21474836480`
- `PERF_PROXY_STAT_DROPS_BUDGET=0`

Tune these to your hardware/profile and enforce them in CI/CD.

## Inputs for auth

Provide one of:

- `MAGPIE_TOKEN`
- `MAGPIE_USER_EMAIL` and `MAGPIE_USER_PASSWORD`

If credentials are missing and `MAGPIE_REGISTER_IF_MISSING=true`, the k6 harness auto-registers a user for test execution.

## Running with local backend (no backend container)

You can run Postgres/Redis in Docker Compose and run backend from source (`go run`) for pre-push validation.

In that setup, pass backend logs to the drop-budget check:

```bash
# terminal 1
cd backend
go run ./cmd/magpie 2>&1 | tee /tmp/magpie-backend.log
```

```bash
# terminal 2
cd scripts/perf
PERF_BACKEND_LOG_FILE=/tmp/magpie-backend.log ./run-gate.sh
```

Drop-budget log source can be controlled with:

- `PERF_DROP_BUDGET_SOURCE=auto` (default)
- `PERF_DROP_BUDGET_SOURCE=compose`
- `PERF_DROP_BUDGET_SOURCE=file`
- `PERF_DROP_BUDGET_SOURCE=skip`
