# Data Storage

## PostgreSQL

Primary persistent store for:

- users and roles
- proxies and stats
- proxy reputation snapshots
- scrape sources and relations
- rotating proxies

Default Docker setup persists Postgres data via named volume `postgres_data`.

## Redis

Used for:

- queue/sync behavior across routines
- runtime distribution features
- leadership lock coordination

## File-based settings

Global settings are read/written at `backend/data/settings.json` in local runs.

In containerized runs, persist this path with a volume if you require file-level durability beyond the container lifecycle.
