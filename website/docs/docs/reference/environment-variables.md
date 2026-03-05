# Environment Variables

## Core backend

- `PROXY_ENCRYPTION_KEY`: required encryption key for stored proxy secrets.
- `JWT_SECRET`: required JWT signing key.
- `JWT_TTL_MINUTES` (default `10080`, allowed `15..10080`): access token lifetime.
- `LOG_LEVEL` (default `info`): backend logger level (`debug`, `info`, `warn`, `error`, `fatal`).
- `BACKEND_PORT` (default `5656`): API listen port.
- `backend-port`: legacy fallback env for API listen port.

## Runtime mode and strict validation

Production mode can be inferred from any of:

- `APP_ENV`
- `ENVIRONMENT`
- `GO_ENV`
- `MAGPIE_ENV`

Values `prod` or `production` are treated as production.

- `STRICT_SECRET_VALIDATION`: override strict startup validation for `JWT_SECRET` and `PROXY_ENCRYPTION_KEY`.

## API hardening

- `CORS_ALLOWED_ORIGINS` (default `http://localhost:5050,http://127.0.0.1:5050,http://localhost:4200,http://127.0.0.1:4200`): comma-separated CORS origin allowlist. Use `*` only in trusted environments.
- `SERVER_READ_TIMEOUT_SECONDS` (default `30`)
- `SERVER_READ_HEADER_TIMEOUT_SECONDS` (default `10`)
- `SERVER_WRITE_TIMEOUT_SECONDS` (default `30`)
- `SERVER_IDLE_TIMEOUT_SECONDS` (default `120`)
- `SERVER_SHUTDOWN_TIMEOUT_SECONDS` (default `20`): graceful shutdown timeout used by the API server.
- `API_UPLOAD_MAX_BODY_BYTES` (default `10485760`, 10 MiB): max request size for multipart upload endpoints.
- `API_JSON_MAX_BODY_BYTES` (default `1048576`, 1 MiB): max request size for JSON endpoints.
- `API_MULTIPART_MEMORY_BYTES` (default `1048576`, 1 MiB): in-memory budget used by multipart parsing before temp-file spillover.
- `SECURITY_HEADERS_ENABLED` (default `true`): enables base response security headers.

## Trusted proxy and outbound safety

- `TRUSTED_PROXY_CIDRS`: comma-separated trusted reverse-proxy CIDRs used for `X-Forwarded-For` / `X-Real-IP` parsing.
- `ALLOW_PRIVATE_NETWORK_EGRESS` (default `false`): allows outbound HTTP targets in private/loopback/reserved ranges (disabled by default for safety).

## Auth and registration controls

- `DISABLE_PUBLIC_REGISTRATION`: disable public `/api/register`.
- `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP`: allow first-admin creation through public registration.
- `ADMIN_BOOTSTRAP_TOKEN`: required token value for first-admin bootstrap when public bootstrap is enabled.
- `ALLOW_INSECURE_REGISTRATION_DEFAULTS`: local-only helper for relaxed registration defaults.
- `AUTH_REVOCATION_FAIL_OPEN` (default `true`): when Redis revocation store is unavailable, allow already-signed/non-expired JWTs.

Auth route rate limiting:

- `AUTH_REQUEST_RATE_LIMIT_WINDOW_SECONDS` (default `60`)
- `AUTH_LOGIN_RATE_LIMIT_PER_WINDOW` (default `60`)
- `AUTH_REGISTER_RATE_LIMIT_PER_WINDOW` (default `20`)
- `AUTH_LOGIN_FAILURE_WINDOW_SECONDS` (default `900`)
- `AUTH_LOGIN_FAILURE_LIMIT_PER_IP` (default `30`)
- `AUTH_LOGIN_FAILURE_LIMIT_PER_EMAIL` (default `10`)
- `AUTH_RATE_LIMIT_LOCAL_FALLBACK_MAX_KEYS` (default `10000`)

## Observability and health

- `ALLOW_PUBLIC_OBSERVABILITY_ENDPOINTS` (default non-production `true`, production `false`): controls public access to `/healthz`, `/readyz`, `/metrics`.
- `OBSERVABILITY_TOKEN`: token accepted via `X-Observability-Token` when public observability access is disabled.
- `READYZ_ALLOW_REDIS_DEGRADED` (default `false`): allows `readyz` to return degraded-ready status when Redis is unavailable.

## GraphQL guards

- `GRAPHQL_MAX_DEPTH` (default `12`)
- `GRAPHQL_MAX_FIELDS` (default `250`)
- `GRAPHQL_MAX_QUERY_BYTES` (default `16384`)
- `GRAPHQL_ALLOW_INTROSPECTION` (default `false`)

## Redis

- `REDIS_MODE` (default `single`): `single` or `sentinel`.
- `REDIS_URL` (default `redis://localhost:8946`): Redis URL for single mode.
- `redisUrl`: legacy `REDIS_URL` fallback.
- `REDIS_PASSWORD`: optional Redis password.
- `REDIS_CONNECT_RETRY_BACKOFF_MS` (default `5000`): reconnect defer/backoff duration.

Sentinel mode:

- `REDIS_MASTER_NAME`
- `REDIS_SENTINEL_ADDRS`
- `REDIS_SENTINEL_PASSWORD`

In default Docker Compose single-mode deployment, backend points to `redis://redis:6379`.

## Database

- `DB_HOST` (default `localhost`)
- `DB_PORT` (default `5434`)
- `DB_NAME` (default `magpie`)
- `DB_USERNAME` (default `magpie_user` in Docker Compose): PostgreSQL username.
- `DB_PASSWORD` (default `ChangeMeToAStrongDbPassword` in Docker Compose): PostgreSQL password.
- `DB_SSLMODE` (default `require`): PostgreSQL TLS mode (`disable`, `allow`, `prefer`, `require`, `verify-ca`, `verify-full`).
- `DB_AUTO_MIGRATE` (default local `true`, production-oriented deployments often set `false`)
- `DB_MAX_OPEN_CONNS` (default `32`)
- `DB_MAX_IDLE_CONNS` (default `DB_MAX_OPEN_CONNS`)
- `DB_CONN_MAX_LIFETIME` seconds (default `300`)
- `DB_CONN_MAX_IDLE_TIME` seconds (default `60`)

## Rotating proxy listeners

- `ROTATING_PROXY_PORT_START` (default `20000`)
- `ROTATING_PROXY_PORT_END` (default `20100`)
- `ROTATING_PROXY_SYNC_INTERVAL_SECONDS` (default `10`): interval used by each backend instance to reconcile local rotating listeners.
- `ROTATING_PROXY_UPSTREAM_TIMEOUT_MS`
- `ROTATING_PROXY_HANDSHAKE_TIMEOUT_MS`
- `ROTATING_PROXY_MAX_REQUEST_BODY_BYTES`
- `ROTATING_PROXY_SOCKS_MAX_CONCURRENT_CONNECTIONS`

Multi-instance identity:

- `MAGPIE_INSTANCE_ID` (default hostname): stable identifier for backend instance ownership.
- `MAGPIE_INSTANCE_ID_FILE`: optional file path fallback source for instance id.
- `MAGPIE_INSTANCE_NAME` (default `MAGPIE_INSTANCE_ID`): human-readable instance label.
- `MAGPIE_INSTANCE_REGION` (default `Unknown`): region label.
- `MAGPIE_INSTANCE_SCOPE`: optional scope label.

Optional HTTP/3 TLS files for rotating listeners:

- `ROTATING_PROXY_HTTP3_TLS_CERT_FILE`
- `ROTATING_PROXY_HTTP3_TLS_KEY_FILE`

## Queue and startup toggles

- `PROXY_QUEUE_SHARDS`
- `SCRAPE_QUEUE_SHARDS`
- `STARTUP_QUEUE_BOOTSTRAP_ASYNC`

## Proxy statistics and timeline maintenance

Statistics ingestion/stream/retention controls include:

- `PROXY_STATISTICS_RETENTION_DAYS`
- `PROXY_STATISTICS_RESPONSE_RETENTION_DAYS`
- `PROXY_STATISTICS_RETENTION_INTERVAL`
- `PROXY_STATISTICS_RETENTION_INTERVAL_MINUTES`
- `PROXY_STATISTICS_RETENTION_BATCH_SIZE`
- `PROXY_STATISTICS_RETENTION_MAX_BATCHES`
- `PROXY_STATISTICS_RETENTION_WORKERS`
- `PROXY_STATISTICS_RETENTION_MAX_RUN_DURATION`
- `PROXY_STATISTICS_RETENTION_DROP_PARTITIONS`
- `PROXY_STATISTICS_RETENTION_MAX_PARTITION_DROPS`
- `PROXY_STATISTICS_AUTO_PARTITION_MIGRATION`
- `PROXY_STATISTICS_PARTITION_PRECREATE_MONTHS`
- `PROXY_STATISTICS_PARTITION_PAST_MONTHS`
- `PROXY_STATISTICS_INGEST_WORKERS`
- `PROXY_STATISTICS_REDIS_STREAM_ENABLED`
- `PROXY_STATISTICS_REDIS_STREAM_KEY`
- `PROXY_STATISTICS_REDIS_STREAM_GROUP`
- `PROXY_STATISTICS_REDIS_STREAM_MAXLEN`
- `PROXY_STATISTICS_REDIS_STREAM_OVERLOAD_POLICY`
- `PROXY_STATISTICS_TENANT_OVERLOAD_POLICIES`
- `PROXY_STATISTICS_PRODUCER_BLOCK_TIMEOUT_MS`

History/snapshot retention controls:

- `PROXY_HISTORY_RETENTION_DAYS`
- `PROXY_SNAPSHOT_RETENTION_DAYS`
- `PROXY_TIMELINE_RETENTION_INTERVAL`
- `PROXY_TIMELINE_RETENTION_INTERVAL_MINUTES`
- `PROXY_TIMELINE_RETENTION_BATCH_SIZE`
- `PROXY_TIMELINE_RETENTION_MAX_BATCHES`
- `PROXY_TIMELINE_RETENTION_MAX_RUN_DURATION`

Orphan cleanup controls:

- `PROXY_ORPHAN_CLEAN_INTERVAL` duration string (example `30m`)
- `PROXY_ORPHAN_CLEAN_INTERVAL_MINUTES` integer fallback (default `60`)

## Worker/tuning envs

- `SCRAPER_PAGE_POOL_MIN_CAPACITY`
- `SCRAPER_PAGE_POOL_MAX_CAPACITY`
- `SCRAPER_POST_PROCESS_QUEUE_CAPACITY`
- `SCRAPER_POST_PROCESS_WORKERS`
- `SCRAPER_CAPTURED_MAX_RESPONSE_BODY_BYTES`
- `SCRAPER_FALLBACK_MAX_RESPONSE_BODY_BYTES`
- `CHECKER_DEFAULT_REQUEST_TIMEOUT_MS`
- `CHECKER_MAX_RESPONSE_BODY_BYTES`

## Releases endpoint

- `GITHUB_TOKEN`: optional token for higher GitHub API quota on `/api/releases`.

## Docker Compose resources

- `BACKEND_CPU_LIMIT` (default `2.00`)
- `BACKEND_MEMORY_LIMIT` (default `1G`)
- `BACKEND_CPU_RESERVATION` (default `0.50`)
- `BACKEND_MEMORY_RESERVATION` (default `256M`)
- `FRONTEND_CPU_LIMIT` (default `1.00`)
- `FRONTEND_MEMORY_LIMIT` (default `512M`)
- `FRONTEND_CPU_RESERVATION` (default `0.25`)
- `FRONTEND_MEMORY_RESERVATION` (default `128M`)
- `POSTGRES_CPU_LIMIT` (default `1.50`)
- `POSTGRES_MEMORY_LIMIT` (default `1G`)
- `POSTGRES_CPU_RESERVATION` (default `0.50`)
- `POSTGRES_MEMORY_RESERVATION` (default `256M`)
- `REDIS_CPU_LIMIT` (default `1.00`)
- `REDIS_MEMORY_LIMIT` (default `512M`)
- `REDIS_CPU_RESERVATION` (default `0.25`)
- `REDIS_MEMORY_RESERVATION` (default `128M`)

## Installer/updater script envs

- `MAGPIE_INSTALL_DIR`
- `MAGPIE_IMAGE_TAG`
- `MAGPIE_REPO_OWNER`
- `MAGPIE_REPO_NAME`
- `MAGPIE_REPO_REF`
- `MAGPIE_COMPOSE_URL`
- `MAGPIE_ENV_EXAMPLE_URL`
- `MAGPIE_FORCE`
