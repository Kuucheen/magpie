# Magpie Backend Production Readiness Gaps

_Last reviewed: 2026-02-28 (final-cycle post-implementation refresh)_

Product constraints preserved:
- local setup remains simple (no mandatory technical bootstrap for local users)
- multi-instance/load-balanced deployments remain first-class

## Closed in this cycle

- [x] **[CRITICAL][Security] First-admin bootstrap now requires request-level proof in production bootstrap mode**  
  **Implemented:** `ADMIN_BOOTSTRAP_TOKEN` + `X-Admin-Bootstrap-Token` enforcement for first-admin creation when production bootstrap is enabled.  
  **Evidence:**
  - `backend/internal/app/server/route_user_handler.go`
  - `backend/internal/app/server/route_user_handler_test.go`
  - `backend/RUNBOOK.md`, `README.md`, `.env.example`

- [x] **[HIGH][CI] CI now validates backend runtime image and dependency-backed smoke path**  
  **Implemented:** backend image build + Postgres/Redis-backed smoke (`/healthz`, `/readyz`, bootstrap/register/login) in GitHub Actions.  
  **Evidence:**
  - `.github/workflows/backend-ci.yml`
  - `Dockerfile` (Go toolchain aligned to module requirement)

- [x] **[MEDIUM][Security] Access-token lifetime now configurable with bounded validation**  
  **Implemented:** `JWT_TTL_MINUTES` with enforced range `15-10080` and startup-time validation (`RequireJWTTTLConfigured`).  
  **Evidence:**
  - `backend/internal/auth/jwt_handler.go`
  - `backend/internal/auth/jwt_handler_test.go`
  - `backend/internal/app/app.go`
  - `backend/RUNBOOK.md`, `README.md`, `.env.example`

## Remaining gaps

- [ ] **[MEDIUM][Deployment safety/Config] Production-focused compose/profile still missing**  
  Current shipped `docker-compose.yml` remains intentionally local/dev-oriented (`DB_SSLMODE=disable`, no Redis auth/TLS defaults, mutable image tags).

- [ ] **[MEDIUM][Observability/Operations] Alert rules + SLO/runbook mapping still implicit**  
  Metrics are present, but concrete baseline alert rules and alert-to-action playbooks are not yet committed.

## Next follow-up order

1. Add production deployment profile/manifests (secure DB/Redis/TLS defaults, pinned dependency tags).
2. Add baseline alerts and runbook response mapping for readiness/dependency/API error conditions.
