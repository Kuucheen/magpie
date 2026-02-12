# Dashboard

Dashboard data is provided from both REST and GraphQL paths, including:

- total checks and total scraped
- week-window aggregates
- reputation breakdown
- country breakdown
- judge quality breakdown
- proxy history/snapshots (GraphQL)

## Data sources

- REST: `GET /api/getDashboardInfo`
- GraphQL: `viewer.dashboard`, `viewer.proxyHistory`, `viewer.proxySnapshots`

## Typical uses

- Track health trends over time
- Identify reputation distribution shifts
- Identify judge quality drift
- Monitor scrape/check throughput
