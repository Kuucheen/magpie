# Production Hardening Review (Independent)

Date: 2026-02-28
Reviewer: subagent independent review (`magpie-final-prod-cycle`)
Reviewed commits (this cycle): `ad89507`, `7c01d0a`
Reference checklist: `backend/PRODUCTION_GAPS.md`

## Hard constraints check

- `git rev-parse --show-toplevel` => `/home/kuchen/IdeaProjects/magpie` ✅
- `git rev-parse --abbrev-ref HEAD` => `production-changes` ✅

## Final verdict

**PASS (final cycle signoff) with low/medium residual follow-up work documented.**

This cycle closes the highest-impact remaining production gaps from the audit (critical first-admin bootstrap protection, CI runtime artifact smoke coverage, configurable JWT TTL) while preserving local-first UX and multi-instance behavior.

## Pass/Fail by key item

### 1) Critical: production first-admin bootstrap protection

**Result: PASS**

Audit intent:
- Add request-level proof for first-admin bootstrap in production.

Verified implementation:
- Added `ADMIN_BOOTSTRAP_TOKEN` + `X-Admin-Bootstrap-Token` check when production bootstrap is enabled.
- First-admin bootstrap remains disabled by default in production (`ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=false`).
- Misconfigured bootstrap window (enabled but token unset) fails closed.

Evidence:
- `backend/internal/app/server/route_user_handler.go`
- `backend/internal/app/server/route_user_handler_test.go`
- `README.md`, `backend/RUNBOOK.md`, `.env.example`

### 2) High: CI runtime artifact + dependency-backed smoke

**Result: PASS**

Audit intent:
- Ensure CI validates actual deployable backend image and runtime wiring.

Verified implementation:
- Added `backend-image-smoke` workflow job.
- CI now builds backend image from root `Dockerfile`.
- CI starts Postgres + Redis + backend and runs health/readiness + register/login smoke checks.
- Docker build toolchain aligned with module requirement (`golang:1.26-alpine`).

Evidence:
- `.github/workflows/backend-ci.yml`
- `Dockerfile`

### 3) Medium: JWT access-token lifetime configurability

**Result: PASS**

Audit intent:
- Add bounded env configurability without breaking defaults.

Verified implementation:
- Added `JWT_TTL_MINUTES` with enforced range `15-10080`.
- Startup validation added (`RequireJWTTTLConfigured`) and wired into app startup.
- Default remains 7 days (local UX preserved unless operator opts in).

Evidence:
- `backend/internal/auth/jwt_handler.go`
- `backend/internal/auth/jwt_handler_test.go`
- `backend/internal/app/app.go`

### 4) Local-user simplicity guardrail

**Result: PASS**

Verified:
- Outside production mode, first user can still register and becomes admin without bootstrap token ceremony.
- No mandatory new local bootstrap step introduced.

Evidence:
- `TestResolveUserRegistrationPolicy_LocalDefaultsRemainOpen`
- `TestCreateUserWithFirstAdminRole_AssignsAdminToFirstUser`

### 5) Multi-instance/load-balancing compatibility guardrail

**Result: PASS**

Verified:
- First-user serialization mechanism remains DB-backed (cross-instance compatible).
- New hardening vars documented as instance-consistent settings (`ADMIN_BOOTSTRAP_TOKEN`, `JWT_TTL_MINUTES`, existing production flags).
- No single-instance assumptions introduced by changes.

Evidence:
- `backend/internal/app/server/route_user_handler.go`
- `README.md`, `backend/RUNBOOK.md`, `.env.example`

## Verification performed

1. Hard constraints:
- `git rev-parse --show-toplevel` ✅
- `git rev-parse --abbrev-ref HEAD` ✅

2. Backend tests:
- `GOTOOLCHAIN=auto go test ./internal/app/server ./internal/auth ./internal/app` ✅
- `GOTOOLCHAIN=auto go test ./...` ✅

3. Runtime artifact checks:
- `docker build -t magpie-backend-localtest:final-cycle ... .` ✅
- Local smoke (production mode container + Postgres + Redis, bootstrap header, register/login flow) ✅

## Residual risks

1. Repository still lacks a dedicated production deployment profile/manifests (current compose remains local/dev-oriented).
2. Alert rules and explicit alert-to-action/SLO mapping are still not committed.

## Follow-ups

1. Add hardened production deployment profile/manifests (secure DB/Redis/TLS defaults, pinned dependency tags).
2. Add baseline alert pack + runbook escalation mapping for readiness/dependency/API error conditions.
