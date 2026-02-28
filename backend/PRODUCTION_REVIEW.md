# Production Hardening Review (Independent)

Date: 2026-02-28
Reviewer: subagent independent review (`magpie-nonprod-review`)
Reviewed commits: `3d13eb6`, `36037b1`
Reference checklist: `backend/PRODUCTION_GAPS.md`

## Hard constraints check

- `git rev-parse --show-toplevel` => `/home/kuchen/IdeaProjects/magpie` ✅
- `git rev-parse --abbrev-ref HEAD` => `production-changes` ✅

## Verdict

**PASS (correct enough for this hardening cycle) with follow-ups**

The reviewed commits materially close the targeted production-hardening gaps without breaking local-first setup/registration behavior. Residual risk remains around first-admin bootstrap window handling (see Risks/Follow-ups).

## Itemized review (pass/fail)

### 1) Critical: public first-user admin takeover path

**Result: PASS (with caveat)**

What was implemented:
- Production-mode defaults now harden registration behavior:
  - `DISABLE_PUBLIC_REGISTRATION=true` by default in production mode.
  - `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=false` by default in production mode.
- First-user admin grant now checks policy and blocks public bootstrap when production default is active.
- After first user exists, default public registration remains disabled.
- Runbook/docs updated to describe controlled bootstrap window.

Evidence:
- `backend/internal/app/server/route_user_handler.go`
- `backend/RUNBOOK.md`, `README.md`, `.env.example`

Caveat:
- Recommendation suggested one-time token/local-only command. Current implementation uses explicit env-gated bootstrap window instead of tokenized bootstrap. This is a meaningful hardening improvement, but not the strongest possible first-admin control.

---

### 2) High: spoofable forwarded headers (trust proxy boundaries)

**Result: PASS**

What was implemented:
- Added `TRUSTED_PROXY_CIDRS` support.
- Forwarded headers (`X-Forwarded-For`/`X-Real-IP`) are trusted only when immediate `RemoteAddr` is in trusted proxy CIDRs.
- Otherwise backend uses `RemoteAddr` directly.
- Shared logic now used by auth rate limit path and observability access log path.
- Unit tests added for both trusted and untrusted source behavior.

Evidence:
- `backend/internal/app/server/client_ip.go`
- `backend/internal/app/server/auth_rate_limit.go`
- `backend/internal/app/server/observability.go`
- `backend/internal/app/server/client_ip_test.go`

---

### 3) High: startup implicit schema migration safety

**Result: PASS**

What was implemented:
- `DB_AUTO_MIGRATE` default now follows runtime mode:
  - local default: `true`
  - production mode default: `false`
- Explicit env override still supported.
- Runbook/docs updated.
- Unit tests added for default/override behavior.

Evidence:
- `backend/internal/database/database_handler.go`
- `backend/internal/database/database_handler_config_test.go`
- `backend/RUNBOOK.md`, `README.md`, `.env.example`

---

### 4) High: secret validation quality (presence-only)

**Result: PASS**

What was implemented:
- Added `STRICT_SECRET_VALIDATION` mode (default false locally, true in production mode).
- Enforced minimum length and placeholder rejection for:
  - `JWT_SECRET`
  - `PROXY_ENCRYPTION_KEY`
- Startup secret checks now run after parsing `-production`, so strict defaults are correctly mode-aware.
- Unit tests added for strict defaults and override behavior.

Evidence:
- `backend/internal/config/runtime_security.go`
- `backend/internal/auth/jwt_handler.go`, `backend/internal/auth/jwt_handler_test.go`
- `backend/internal/security/proxy_secret.go`, `backend/internal/security/proxy_secret_test.go`
- `backend/internal/app/app.go`

---

### 5) Multi-instance / load-balancing consistency

**Result: PASS (documented and code-compatible)**

What was implemented:
- Added explicit docs that hardening env values must be consistent across backend instances in load-balanced deployments:
  - `DISABLE_PUBLIC_REGISTRATION`
  - `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP`
  - `DB_AUTO_MIGRATE`
  - `STRICT_SECRET_VALIDATION`
  - `TRUSTED_PROXY_CIDRS`
- Existing first-user admin transaction lock behavior remains DB-backed and multi-instance compatible.

Evidence:
- commit `36037b1` (`README.md`, `backend/RUNBOOK.md`, `.env.example`)
- `backend/internal/app/server/route_user_handler.go`

## Local-user simplicity verification

**Result: PASS**

- Local default behavior remains simple:
  - public registration enabled by default outside production mode
  - first local user still becomes admin
  - no new mandatory bootstrap token/technical pre-step for first local setup

Evidence/tests:
- `TestResolveUserRegistrationPolicy_LocalDefaultsRemainOpen`
- `TestCreateUserWithFirstAdminRole_AssignsAdminToFirstUser`
- `TestCreateUserWithFirstAdminRole_RespectsPublicRegistrationFlagAfterBootstrap`

## Test / verification results

Commands run:

1. Branch/repo hard constraints:
- `git rev-parse --show-toplevel` ✅
- `git rev-parse --abbrev-ref HEAD` ✅

2. Targeted package tests (needed `GOTOOLCHAIN=auto` because local `go` default is 1.25.7 while module requires 1.26):
- `GOTOOLCHAIN=auto go test ./internal/app/server ./internal/auth ./internal/security ./internal/database ./internal/config` ✅

3. Full backend test suite:
- `GOTOOLCHAIN=auto go test ./...` ✅

## Residual risks

1. **Bootstrap-window race risk remains if operators enable `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=true` on an internet-exposed service before intended admin registration.**
   - Mitigation today: controlled window + keep registration disabled by default.
   - Stronger follow-up: add one-time bootstrap token/header or dedicated local/bootstrap CLI flow.

2. **Forwarded header security still depends on correct reverse-proxy behavior** (proxy should overwrite/sanitize forwarding headers, and `TRUSTED_PROXY_CIDRS` must be accurate/consistent across instances).

## Follow-ups

1. Consider implementing tokenized or local-only first-admin seed path for stronger protection than env-window gating.
2. Continue with remaining open checklist items from `PRODUCTION_GAPS.md` not addressed in these commits (CI runtime artifact smoke, JWT TTL configurability, production compose profile hardening, alert rules/SLO mappings).
