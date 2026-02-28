# Magpie Backend Production Readiness Gaps

_Last reviewed: 2026-02-28 (post auth-bootstrap revert audit)_

This is a refreshed, **remaining gaps only** checklist for `backend/` and current deployment config.

## Critical (fix before internet-facing production)

- [ ] **[CRITICAL][Security] Public first-user admin bootstrap is still takeover-prone**  
  **Evidence:** `POST /api/register` is public (`internal/app/server/routes.go`) and `createUserWithFirstAdminRole` grants admin when user count is `0` (`internal/app/server/route_user_handler.go`). `backend/RUNBOOK.md` still documents this bootstrap path.  
  **Risk:** Any actor who reaches the service first can permanently claim admin on a fresh environment.  
  **Recommended fix (exact):**
  1. Add one-time bootstrap protection for first admin creation (`ADMIN_BOOTSTRAP_TOKEN` header or dedicated local-only bootstrap command).
  2. In production mode, default to `DISABLE_PUBLIC_REGISTRATION=true` and require explicit override.
  3. Auto-disable bootstrap path once first admin exists.
  4. Update `RUNBOOK.md` with a secure admin-seed flow that does not rely on unauthenticated public registration.

## High

- [ ] **[HIGH][Security/Reliability] Client IP is taken from spoofable forwarding headers without trusted proxy boundaries**  
  **Evidence:** `getAuthClientIP` and `clientIPFromRequest` prioritize `X-Forwarded-For`/`X-Real-IP` unconditionally (`internal/app/server/auth_rate_limit.go`, `internal/app/server/observability.go`).  
  **Risk:** If backend is reachable directly, attackers can spoof IPs to dilute per-IP throttling and pollute forensic logs.  
  **Recommended fix (exact):**
  1. Introduce `TRUSTED_PROXY_CIDRS` (or equivalent) and only trust forwarded headers when request source is trusted.
  2. Otherwise derive client IP strictly from `RemoteAddr`.
  3. Add tests for direct-access spoofing and trusted-proxy forwarding behavior.

- [ ] **[HIGH][Deployment safety] Database schema changes run implicitly on app startup**  
  **Evidence:** `DB_AUTO_MIGRATE` defaults to `true` (`internal/database/database_handler.go`), and startup executes schema DDL helpers (e.g. column drops/unlogged/index changes in `ensureBlacklistSchema`).  
  **Risk:** Startup-triggered migrations can introduce unpredictable deploy times, accidental irreversible DDL, and rollback hazards during incidents.  
  **Recommended fix (exact):**
  1. Default `DB_AUTO_MIGRATE=false` for production.
  2. Move schema evolution to explicit, versioned migration steps (pre-deploy job).
  3. Add migration preflight in runbook: backup check, lock strategy, rollback/restore validation.

- [ ] **[HIGH][Secrets/Config] Secret validation is presence-only; weak/shared values are accepted**  
  **Evidence:** startup only enforces non-empty `JWT_SECRET` and `PROXY_ENCRYPTION_KEY` (`internal/auth/jwt_handler.go`, `internal/security/proxy_secret.go`).  
  **Risk:** Weak or placeholder secrets can pass startup and reach production, reducing resistance to token forgery or data compromise.  
  **Recommended fix (exact):**
  1. Enforce minimum secret quality (length + entropy/format checks) at startup.
  2. Reject known placeholder/default strings from `.env.example` patterns.
  3. Document rotation cadence and staged rotation procedures in `RUNBOOK.md`.

- [ ] **[HIGH][CI] Current CI validates code but not deployable artifacts/runtime wiring**  
  **Evidence:** `.github/workflows/backend-ci.yml` runs tests/vet/staticcheck/govulncheck/race, but does not build runtime image or run integration smoke with Postgres+Redis.  
  **Risk:** Changes can pass CI while container startup, runtime deps, or startup probes fail in deployment.  
  **Recommended fix (exact):**
  1. Add CI job to build `Dockerfile` image.
  2. Spin up ephemeral Postgres+Redis and run backend smoke (`/healthz`, `/readyz`, login/register happy path).
  3. Add container vulnerability scan in release pipeline.

## Medium

- [ ] **[MEDIUM][Security] Access token lifetime is fixed at 7 days**  
  **Evidence:** `jwtTTL` is hardcoded (`internal/auth/jwt_handler.go`).  
  **Risk:** Session duration cannot be tightened per environment or incident posture.  
  **Recommended fix (exact):**
  1. Add bounded env-based token lifetime (e.g., `JWT_TTL_MINUTES` with sane min/max).
  2. Prefer short access token + refresh flow for long sessions.

- [ ] **[MEDIUM][Deployment safety/Config] Shipped compose defaults are dev-friendly and unsafe if reused as production baseline**  
  **Evidence:** `docker-compose.yml` defaults `DB_SSLMODE=disable`, uses `redis:latest`, and no Redis auth/TLS settings by default.  
  **Risk:** Weak transport/auth posture and nondeterministic dependency upgrades in real deployments.  
  **Recommended fix (exact):**
  1. Provide a production compose/manifest profile with secure defaults (`DB_SSLMODE=require|verify-full`, pinned image tags, Redis auth/TLS).
  2. Mark current compose clearly as local/dev-only in docs.

- [ ] **[MEDIUM][Observability/Runbook] Alerting and SLO response guidance is still implicit**  
  **Evidence:** `/metrics` exists, but repository lacks concrete alert rules and runbook thresholds/actions tied to those metrics.  
  **Risk:** Slower detection/triage despite telemetry being available.  
  **Recommended fix (exact):**
  1. Add baseline alert rules (5xx ratio, p95 latency, readiness failures, DB/Redis dependency failures).
  2. Extend `RUNBOOK.md` with alert-to-action mappings (owner, first checks, escalation).

## Suggested implementation order

1. Close first-user admin takeover path and update bootstrap runbook.
2. Add trusted-proxy IP handling for rate-limits/logging.
3. Move DB migrations out of startup path and establish migration run procedure.
4. Enforce secret strength and finalize rotation policy.
5. Expand CI to build/test runtime artifacts.
6. Tighten token/config/deployment defaults and add concrete alert rules.
