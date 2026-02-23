# Environment Variables

## Core backend

- `PROXY_ENCRYPTION_KEY`: encryption key for stored proxy secrets
- `JWT_SECRET`: JWT signing key
- `BACKEND_PORT` (default `5656`): API listen port
- `backend-port`: legacy port env fallback
- `CORS_ALLOWED_ORIGINS` (default `http://localhost:5050,http://127.0.0.1:5050`): comma-separated CORS origin allowlist.
- `SERVER_READ_TIMEOUT_SECONDS` (default `30`)
- `SERVER_READ_HEADER_TIMEOUT_SECONDS` (default `10`)
- `SERVER_WRITE_TIMEOUT_SECONDS` (default `30`)
- `SERVER_IDLE_TIMEOUT_SECONDS` (default `120`)
- `API_UPLOAD_MAX_BODY_BYTES` (default `10485760`, 10 MiB): max request size for multipart upload endpoints.
- `API_JSON_MAX_BODY_BYTES` (default `1048576`, 1 MiB): max request size for JSON endpoints.
- `API_MULTIPART_MEMORY_BYTES` (default `1048576`, 1 MiB): in-memory budget used by multipart parsing before temp-file spillover.

## Redis

- `redisUrl` (default `redis://localhost:8946`)

In default Docker Compose this is set to `redis://redis:6379`.

## Database

- `DB_HOST` (default `localhost`)
- `DB_PORT` (default `5434`)
- `DB_NAME` (default `magpie`)
- `DB_USERNAME` (default `admin`)
- `DB_PASSWORD` (default `admin`)
- `DB_AUTO_MIGRATE` (default `true`)
- `DB_MAX_OPEN_CONNS` (default `32`)
- `DB_MAX_IDLE_CONNS` (default `DB_MAX_OPEN_CONNS`)
- `DB_CONN_MAX_LIFETIME` seconds (default `300`)
- `DB_CONN_MAX_IDLE_TIME` seconds (default `60`)

## Rotating proxy listeners

- `ROTATING_PROXY_PORT_START` (default `20000`)
- `ROTATING_PROXY_PORT_END` (default `20100`)
- `ROTATING_PROXY_SYNC_INTERVAL_SECONDS` (default `10`): interval used by each backend instance to reconcile local rotating listeners.
- `MAGPIE_INSTANCE_ID` (default hostname): stable identifier for the backend instance that owns/listens for its rotators.
- `MAGPIE_INSTANCE_NAME` (default `MAGPIE_INSTANCE_ID`): human-readable instance label returned by the rotating instances API.
- `MAGPIE_INSTANCE_REGION` (default `Unknown`): region label returned by the rotating instances API.

## Maintenance

- `PROXY_ORPHAN_CLEAN_INTERVAL` duration string (example `30m`)
- `PROXY_ORPHAN_CLEAN_INTERVAL_MINUTES` integer fallback (default `60`)
- `PROXY_STATISTICS_RETENTION_DAYS` (default `30`): delete `proxy_statistics` rows older than this many days. Set `0` to disable row deletion.
- `PROXY_STATISTICS_RESPONSE_RETENTION_DAYS` (default `7`): clear old `proxy_statistics.response_body` values. Set `0` to disable body pruning.
- `PROXY_STATISTICS_RETENTION_INTERVAL` duration string (example `1h`): preferred retention routine schedule.
- `PROXY_STATISTICS_RETENTION_INTERVAL_MINUTES` integer fallback (default `60`)
- `PROXY_STATISTICS_RETENTION_BATCH_SIZE` integer per batch (default `5000`)
- `PROXY_STATISTICS_RETENTION_MAX_BATCHES` integer per run (default `12`)

## Releases endpoint

- `GITHUB_TOKEN` optional token for higher GitHub API quota on `/api/releases`

## Installer/updater script envs

- `MAGPIE_INSTALL_DIR`
- `MAGPIE_IMAGE_TAG`
- `MAGPIE_REPO_OWNER`
- `MAGPIE_REPO_NAME`
- `MAGPIE_REPO_REF`
- `MAGPIE_COMPOSE_URL`
- `MAGPIE_ENV_EXAMPLE_URL`
- `MAGPIE_FORCE`
