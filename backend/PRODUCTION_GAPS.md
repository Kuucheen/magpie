# Magpie Backend Production Readiness Gaps

_Last reviewed: 2026-02-28 (final-cycle audit, pre-implementation)_

This checklist contains **remaining** production-readiness gaps after the previous hardening cycles.

Product constraints kept in scope:
- local setup must stay simple (no mandatory technical bootstrap for local users)
- multi-instance/load-balanced deployments remain first-class (no single-instance assumptions)

## Critical (fix before internet-facing production)

- [ ] **[CRITICAL][Security] First-admin bootstrap in production still lacks request-level proof-of-intent**  
  **Evidence:** Production defaults now disable public registration/bootstrap, but if operators temporarily set `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=true`, first-admin creation still relies only on env toggles (no per-request bootstrap secret/token in `POST /api/register`, `backend/internal/app/server/route_user_handler.go`).  
  **Risk:** During a bootstrap window, whichever request reaches the service first can still claim admin.  
  **Recommended fix (exact):**
  1. Add `ADMIN_BOOTSTRAP_TOKEN` support and require `X-Admin-Bootstrap-Token` (or equivalent) for first-admin registration in production bootstrap mode.
  2. Keep local defaults unchanged (first local user can still become admin without extra bootstrap mechanics).
  3. Document bootstrap-window procedure for multi-instance deployments (shared bootstrap token, short window, disable immediately after first admin).

## High

- [ ] **[HIGH][CI] CI still does not validate deployable backend artifact + runtime dependency wiring**  
  **Evidence:** `.github/workflows/backend-ci.yml` currently runs tests/static checks but does not build the backend container image or run an integration smoke against Postgres+Redis.  
  **Risk:** Code can pass CI while image build, startup command/env wiring, readiness probes, or auth bootstrap path fail at deploy time.  
  **Recommended fix (exact):**
  1. Add CI job to build `Dockerfile` image for backend runtime.
  2. Start ephemeral Postgres+Redis and run smoke checks (`/healthz`, `/readyz`, register/login happy-path).
  3. Fail CI if image does not start cleanly in this dependency-backed smoke environment.

## Medium

- [ ] **[MEDIUM][Security] Access-token lifetime is fixed at 7 days**  
  **Evidence:** `jwtTTL` is hardcoded in `backend/internal/auth/jwt_handler.go`.  
  **Risk:** Cannot tighten session lifetime per environment or incident posture.  
  **Recommended fix (exact):**
  1. Add bounded env-based token lifetime (`JWT_TTL_MINUTES`) with sane min/max.
  2. Keep local-friendly default at current value to avoid UX regressions.

- [ ] **[MEDIUM][Deployment safety/Config] Shipped compose defaults are intentionally local/dev-oriented and unsafe as production baseline**  
  **Evidence:** `docker-compose.yml` defaults `DB_SSLMODE=disable`, `redis:latest`, and no Redis auth/TLS settings by default.  
  **Risk:** Weak transport/auth posture and nondeterministic dependency upgrades if reused in production.  
  **Recommended fix (exact):**
  1. Keep current file local/dev-friendly, but provide a dedicated production profile/manifest with secure defaults (`DB_SSLMODE=require|verify-full`, pinned Redis tag, auth/TLS guidance).
  2. Explicitly label current compose as local/dev only in docs.

- [ ] **[MEDIUM][Observability/Operations] Alert rules and SLO response mapping are still implicit**  
  **Evidence:** Metrics endpoint exists, but repository still lacks concrete baseline alert rules + runbook threshold/action mapping.  
  **Risk:** Slower triage and inconsistent incident response despite telemetry being present.  
  **Recommended fix (exact):**
  1. Add starter alert rules (5xx ratio, p95 latency, readiness failures, dependency outages).
  2. Extend `RUNBOOK.md` with alert-to-action owner/escalation guidance.

## Suggested implementation order for this final cycle

1. Close first-admin bootstrap window takeover risk with request-level bootstrap token.
2. Add CI runtime artifact build + dependency-backed smoke checks.
3. Add configurable JWT TTL (bounded, default preserved).
4. Leave compose-production profile and alert-pack work as next follow-up if time is exhausted in this cycle.
