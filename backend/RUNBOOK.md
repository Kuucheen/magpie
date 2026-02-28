# Magpie Backend Production Runbook

_Last updated: 2026-02-28_

## First 15 Minutes Checklist

1. Confirm deployment target and release version.
2. Check liveness/readiness:
   - `curl -fsS http://<backend-host>:5656/healthz`
   - `curl -fsS http://<backend-host>:5656/readyz`
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

Production mode (`-production`) defaults:
- `DISABLE_PUBLIC_REGISTRATION=true`
- `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=false`

That means public `/api/register` first-admin bootstrap is blocked by default in production.

For a controlled initial bootstrap window, set:
- `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=true`

After first admin is created, keep bootstrap disabled and keep `DISABLE_PUBLIC_REGISTRATION=true` unless intentional public signups are required.

## Probe Interpretation

### `/healthz` (liveness)
- `200`: process is running.

### `/readyz` (readiness)
Checks:
- DB ping
- Redis ping (`degraded` state possible when `READYZ_ALLOW_REDIS_DEGRADED=true`)
- startup queue bootstrap completion

Routing guidance:
- Route traffic only when `/readyz` returns `200`.
- `503` indicates instance should be removed from load balancer.

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
- auth/token validation and queue operations may fail

Actions:
1. Verify Redis endpoint and connectivity.
2. Ensure persistence/replication status is healthy.
3. If running in degraded mode, restore Redis ASAP and disable degraded mode after recovery.

### Startup queue bootstrap stuck
Symptoms:
- `/readyz` shows `startup_queue_bootstrap=starting`

Actions:
1. Check DB and Redis connectivity.
2. Inspect backend logs for queue bootstrap retry errors.
3. Restart instance only after root cause is addressed.

## Token/Secret Rotation

### JWT secret (`JWT_SECRET`)
- Rotate during maintenance window.
- Expect all existing tokens to become invalid; users must re-authenticate.
- With `STRICT_SECRET_VALIDATION=true` (default in production mode), weak/placeholder values are rejected at startup.

### Proxy encryption key (`PROXY_ENCRYPTION_KEY`)
- Rotate only with explicit migration/export plan.
- Changing key without migration breaks decryption of stored proxy secrets.
- With `STRICT_SECRET_VALIDATION=true` (default in production mode), weak/placeholder values are rejected at startup.

### Secret validation mode (`STRICT_SECRET_VALIDATION`)
- Local default: `false`
- Production mode default: `true`
- Explicit env override (`true` or `false`) always wins.


## TLS / Reverse Proxy Requirement

- Run backend behind a TLS-terminating reverse proxy (Nginx, Traefik, Caddy, ALB, etc.).
- Do **not** expose plaintext backend traffic directly to the internet.
- Set `TRUSTED_PROXY_CIDRS` to the CIDRs of your reverse proxies.
- `X-Forwarded-For` / `X-Real-IP` are only trusted when immediate `RemoteAddr` is in `TRUSTED_PROXY_CIDRS`; otherwise backend uses `RemoteAddr` directly.
- Ensure proxy forwards `X-Forwarded-For` and request IDs (`X-Request-ID`) for traceability.

## Migration Strategy

- `DB_AUTO_MIGRATE` local default: `true`
- `DB_AUTO_MIGRATE` production mode default: `false`
- Explicit `DB_AUTO_MIGRATE` env override always wins.

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
