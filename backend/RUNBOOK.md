# Magpie Backend Production Runbook

_Last updated: 2026-03-04_

## First 15 Minutes Checklist

1. Confirm deployment target and release version.
2. Check liveness/readiness:
   - `curl -fsS http://<backend-host>:5656/healthz`
   - `curl -fsS http://<backend-host>:5656/readyz`
   - if observability protection is enabled, add `-H "X-Observability-Token: <OBSERVABILITY_TOKEN>"`
3. Validate dependencies:
   - PostgreSQL reachable and accepting queries.
   - Redis reachable (or readiness explicitly allowed to degrade via `READYZ_ALLOW_REDIS_DEGRADED=true`).
4. Review backend logs for:
   - panic recovery events,
   - elevated 5xx responses,
   - auth/login spikes.
5. If impact is user-facing, initiate rollback/mitigation within 15 minutes.

## Admin Bootstrap

Local default (without `-production`): when no users exist, the first registered user becomes admin automatically.

Example:

```bash
curl -X POST http://localhost:5656/api/register   -H "Content-Type: application/json"   -d '{"email":"admin@example.com","password":"ChangeMe123!"}'
```

Production mode (`-production` or runtime env markers `APP_ENV`/`ENVIRONMENT`/`GO_ENV`/`MAGPIE_ENV` = `prod`/`production`) defaults:
- `DISABLE_PUBLIC_REGISTRATION=true`
- `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=false`

That means public `/api/register` first-admin bootstrap is blocked by default in production.

For a controlled initial bootstrap window, set:
- `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=true`

Then call registration normally:

```bash
curl -X POST http://<backend-host>:5656/api/register \
  -H "Content-Type: application/json" \
  -d '{"email":"admin@example.com","password":"ChangeMe123!"}'
```

After first admin is created, immediately disable bootstrap (`ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=false`) and keep `DISABLE_PUBLIC_REGISTRATION=true` unless intentional public signups are required.

For load-balanced multi-instance deployments, apply the same values for
`DISABLE_PUBLIC_REGISTRATION` and
`ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP` on every backend instance.

## Probe Interpretation

### `/healthz` (liveness)
- `200`: process is running.

### `/readyz` (readiness)
Checks:
- DB ping
- Redis ping (`degraded` state possible when `READYZ_ALLOW_REDIS_DEGRADED=true`)
- startup queue bootstrap completion (or `degraded` when Redis degradation is explicitly allowed and Redis is unavailable)

Routing guidance:
- Route traffic only when `/readyz` returns `200`.
- `503` indicates instance should be removed from load balancer.

### Observability endpoint access control
- Endpoints: `/healthz`, `/readyz`, `/metrics`
- Local/non-production default: public.
- Production default: protected unless explicitly opened.
- Controls:
  - `ALLOW_PUBLIC_OBSERVABILITY_ENDPOINTS=true|false`
  - `OBSERVABILITY_TOKEN=<strong-random-token>` (send via `X-Observability-Token`)
- When protection is enabled:
  - loopback requests are allowed without token
  - non-loopback requests must provide `X-Observability-Token` matching `OBSERVABILITY_TOKEN`

## GraphQL Hardening

- `/api/graphql` now requires authentication (`Authorization: Bearer <JWT>`).
- Request guards are applied before execution:
  - max query bytes (`GRAPHQL_MAX_QUERY_BYTES`, default `16384`)
  - max depth (`GRAPHQL_MAX_DEPTH`, default `12`)
  - max field count (`GRAPHQL_MAX_FIELDS`, default `250`)
  - introspection disabled by default (`GRAPHQL_ALLOW_INTROSPECTION=false`)

## Common Failure Modes

### PostgreSQL unavailable
Symptoms:
- `/readyz` returns `503` with `database=down`
- API endpoints return 5xx / timeouts

Actions:
1. Validate DB host/port/credentials/network.
2. Check DB saturation (`max_connections`, long-running queries).
3. Restart backend after DB recovery if connection pool is stale.

### Redis unavailable
Symptoms:
- `/readyz` reports `redis=down` or `redis=degraded`
- redis probe details include mode/error class context, e.g. `mode=sentinel; error_class=connect_failed; retry_after=...`
- queue operations may fail
- JWT signature/expiry validation continues, and revocation checks fail open by default (`AUTH_REVOCATION_FAIL_OPEN=true`)

Actions:
1. Verify Redis endpoint and connectivity.
2. Ensure persistence/replication status is healthy.
3. If running in degraded mode, restore Redis ASAP and disable degraded mode after recovery.
4. If checker throughput drops due stats-ingest pressure, tune `PROXY_STATISTICS_PRODUCER_BLOCK_TIMEOUT_MS` lower to reduce producer-side waiting.

### Redis Sentinel HA configuration

Use sentinel mode for multi-node Redis:

```bash
REDIS_MODE=sentinel
REDIS_MASTER_NAME=magpie-master
REDIS_SENTINEL_ADDRS=redis-sentinel-1:26379,redis-sentinel-2:26379,redis-sentinel-3:26379

# Optional (when Redis/Sentinel auth is enabled)
REDIS_PASSWORD=<redis-password>
REDIS_SENTINEL_PASSWORD=<sentinel-password>
```

Single-node fallback remains available:

```bash
REDIS_MODE=single
redisUrl=redis://redis:6379
```

### Redis Sentinel failover procedure

1. Verify current readiness details include `mode=sentinel`:
   - `curl -fsS http://<backend-host>:5656/readyz`
2. Identify current Redis master:
   - `redis-cli -p 26379 SENTINEL get-master-addr-by-name <REDIS_MASTER_NAME>`
3. Simulate primary failure (example with Docker):
   - `docker pause magpie_redis_primary`
4. Wait for sentinel promotion (usually within seconds) and verify new master:
   - `redis-cli -p 26379 SENTINEL get-master-addr-by-name <REDIS_MASTER_NAME>`
5. Confirm backend recovery:
   - `curl -fsS http://<backend-host>:5656/readyz`
   - verify queue/auth/stat writes succeed in logs
6. Restore failed node and confirm it rejoins as replica.
   - `docker unpause magpie_redis_primary`

Automated failover integration test:

```bash
go test -tags integration ./internal/support -run TestRedisSentinelFailover_RecoversWritesAfterPrimaryDown -count=1
```

### Startup queue bootstrap stuck
Symptoms:
- `/readyz` shows `startup_queue_bootstrap=starting`

Actions:
1. Check DB and Redis connectivity.
2. Inspect backend logs for queue bootstrap retry errors.
3. Restart instance only after root cause is addressed.

## Scraper Egress Guard

- Scraping and robots checks reject private/special-use network targets by default.
- This prevents authenticated SSRF to localhost/internal addresses.
- To explicitly allow private egress (local/testing only), set:
  - `ALLOW_PRIVATE_NETWORK_EGRESS=true`

## Rotating Proxy HTTP/3 TLS Certificates

- HTTP/3 rotator listeners require a configured TLS certificate and key.
- Set both:
  - `ROTATING_PROXY_HTTP3_TLS_CERT_FILE=/path/to/cert.pem`
  - `ROTATING_PROXY_HTTP3_TLS_KEY_FILE=/path/to/key.pem`
- Use a certificate whose SAN matches the hostname clients use for the rotator endpoint.
- If either variable is missing or invalid, HTTP/3 rotator startup fails fast with a configuration error.

## Token/Secret Rotation

### JWT secret (`JWT_SECRET`)
- Rotate during maintenance window.
- Expect all existing tokens to become invalid; users must re-authenticate.
- With `STRICT_SECRET_VALIDATION=true`, weak/placeholder values are rejected at startup.
- If `STRICT_SECRET_VALIDATION` is unset, strict mode defaults on when `-production` is enabled or runtime env indicates production (`APP_ENV`/`ENVIRONMENT`/`GO_ENV`/`MAGPIE_ENV` = `prod`/`production`).
- Default local Docker Compose fallback: `magpie-local-compose-jwt-secret-2026` when `JWT_SECRET` is unset.
- Override that fallback for shared or internet-exposed deployments.

### JWT access token lifetime (`JWT_TTL_MINUTES`)
- Optional bounded override for access token lifetime.
- Allowed range: `15-10080` minutes (default `10080`, i.e. 7 days).
- Startup fails fast if the configured value is outside range or invalid.
- In multi-instance deployments, keep the same value on all instances.

### JWT revocation outage mode (`AUTH_REVOCATION_FAIL_OPEN`)
- Default: `true` (prefer availability when Redis is unavailable).
- When `true`, if Redis revocation store is unavailable, token validation accepts signature/expiry-valid tokens.
- Set to `false` to fail closed during a revocation-store outage.
- In multi-instance deployments, keep this value identical on all instances.

### Outbound email settings
- Use these env vars for password recovery or other outbound email:
  - `MAIL_FROM_ADDRESS`
  - `MAIL_FROM_NAME` (optional)
  - `SMTP_HOST`
  - `SMTP_PORT` (optional, defaults to `587`)
  - `SMTP_USERNAME` and `SMTP_PASSWORD` (optional; set both or neither)
- Startup fails fast if email env vars are partially configured or invalid.
- In multi-instance deployments, keep the same mail settings on all instances.

### Proxy encryption key (`PROXY_ENCRYPTION_KEY`)
- Rotate only with explicit migration/export plan.
- Changing key without migration breaks decryption of stored proxy secrets.
- With `STRICT_SECRET_VALIDATION=true`, weak/placeholder values are rejected at startup.

### Secret validation mode (`STRICT_SECRET_VALIDATION`)
- Local default: `false`
- Production default: `true` when `-production` is enabled or runtime env indicates production (`APP_ENV`/`ENVIRONMENT`/`GO_ENV`/`MAGPIE_ENV` = `prod`/`production`)
- Explicit env override (`true` or `false`) always wins.


## TLS / Reverse Proxy Requirement

- Run backend behind a TLS-terminating reverse proxy (Nginx, Traefik, Caddy, ALB, etc.).
- Do **not** expose plaintext backend traffic directly to the internet.
- App-level security headers are enabled by default (`SECURITY_HEADERS_ENABLED=true`).
- Set `TRUSTED_PROXY_CIDRS` to the CIDRs of your reverse proxies.
- `X-Forwarded-For` / `X-Real-IP` are only trusted when immediate `RemoteAddr` is in `TRUSTED_PROXY_CIDRS`; otherwise backend uses `RemoteAddr` directly.
- In multi-instance deployments behind the same load balancer/reverse proxy tier, keep `TRUSTED_PROXY_CIDRS` consistent on all instances.
- Ensure proxy forwards `X-Forwarded-For` and request IDs (`X-Request-ID`) for traceability.

## Migration Strategy

- `DB_AUTO_MIGRATE` local default: `true`
- `DB_AUTO_MIGRATE` production mode default: `false`
- Explicit `DB_AUTO_MIGRATE` env override always wins.
- In multi-instance deployments, keep `DB_AUTO_MIGRATE` identical on all instances (recommended: `false` + explicit migration job).

## Rollback

1. Deploy previous known-good backend image.
2. Confirm `/healthz` and `/readyz`.
3. Verify login and proxy operations.
4. Capture incident notes and timeline before closing.

## Container Runtime Hardening

Recommended runtime settings:
- run as non-root (image uses UID/GID `65532`)
- `readOnlyRootFilesystem: true` where possible
- drop Linux capabilities (`capDrop: ["ALL"]`)
- disallow privilege escalation

Kubernetes example snippet:

```yaml
securityContext:
  runAsNonRoot: true
  runAsUser: 65532
  runAsGroup: 65532
  allowPrivilegeEscalation: false
  readOnlyRootFilesystem: true
  capabilities:
    drop: ["ALL"]
```
