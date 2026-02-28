# Magpie Backend Production Readiness Gaps

_Last reviewed: 2026-02-28_

This checklist focuses on high-impact production gaps found in `backend/` and related deployment config.

## Critical (fix before production exposure)

- [ ] **[CRITICAL] Unauthenticated first-user admin bootstrap can be hijacked**  
  **Evidence:** `POST /api/register` is public and first created user is granted admin role (`internal/app/server/route_user_handler.go`, `createUserWithFirstAdminRole`).  
  **Risk:** If the instance is reachable before the intended operator registers, an attacker can claim admin permanently.  
  **Recommended fix (exact):**
  1. Add `ADMIN_BOOTSTRAP_TOKEN` (required when no admin exists).
  2. Require header `X-Magpie-Bootstrap-Token` on first admin registration and compare in constant time.
  3. After first admin is created, disable token path automatically.
  4. Add fallback option `DISABLE_PUBLIC_REGISTRATION=true` for production and document admin seed flow.

- [ ] **[CRITICAL] No authenticated health/readiness endpoints for orchestration safety**  
  **Evidence:** No `/healthz`/`/readyz` endpoint exists in server routes (`internal/app/server/routes.go`).  
  **Risk:** Orchestrators can’t reliably detect bad startup states (DB/Redis unavailable, async queue bootstrap incomplete), causing traffic to unhealthy instances.  
  **Recommended fix (exact):**
  1. Add `GET /healthz` (liveness: process up).
  2. Add `GET /readyz` (readiness: DB ping ok, Redis reachable or explicitly degraded, startup queue bootstrap status exposed).
  3. Return JSON payload with component states + version/instance id.
  4. Use these endpoints in deployment probes before routing traffic.

## High

- [ ] **[HIGH] No global panic recovery middleware for HTTP handlers**  
  **Evidence:** Router stack in `OpenRoutes` applies CORS/body limits only; no top-level `recover` middleware.  
  **Risk:** Panic in handler path can crash request path and may terminate process depending on context; inconsistent 500 behavior and poor incident visibility.  
  **Recommended fix (exact):**
  1. Add `recoverMiddleware(next)` around entire router.
  2. On panic: log structured event with request id/path/method and stack trace.
  3. Return sanitized `500 {"error":"Internal server error"}`.

- [ ] **[HIGH] Missing standardized request correlation and structured access logging**  
  **Evidence:** App logs events, but there is no request-id middleware/access log middleware in server pipeline.  
  **Risk:** Difficult incident triage and cross-service tracing in production.  
  **Recommended fix (exact):**
  1. Add `X-Request-ID` middleware (accept incoming or generate UUID).
  2. Include request id in all handler logs and error logs.
  3. Emit one structured access log per request (method, path template, status, latency, user id if authenticated, remote ip).

- [ ] **[HIGH] No metrics/telemetry endpoint for observability**  
  **Evidence:** No Prometheus/OpenTelemetry instrumentation and no `/metrics` endpoint in backend routes.  
  **Risk:** No SLO tracking, alerting blind spots, delayed outage detection.  
  **Recommended fix (exact):**
  1. Add Prometheus metrics endpoint (request rate/latency/error ratio, DB query duration, queue depth, goroutines, GC).
  2. Add counters for auth failures, rate-limit blocks, rotating proxy errors.
  3. Define baseline alerts: API 5xx rate, p95 latency, DB connection saturation, queue backlog.

- [ ] **[HIGH] CI production gate is missing**  
  **Evidence:** No GitHub workflow file found under `.github/workflows/`.  
  **Risk:** Regressions can ship without automated tests/lint/build/security scanning.  
  **Recommended fix (exact):**
  1. Add CI workflow running: `go test ./...`, `go vet ./...`, staticcheck, govulncheck, and race tests on critical packages.
  2. Fail build on test/lint/security errors.
  3. Add dependency review and container image scan step for release builds.

- [ ] **[HIGH] Container runtime hardening is incomplete**  
  **Evidence:** Root `Dockerfile` uses distroless base but does not set non-root user; `EXPOSE 8082` does not match backend default port 5656.  
  **Risk:** Larger blast radius on container breakout and deployment misconfiguration/confusion.  
  **Recommended fix (exact):**
  1. Set explicit non-root runtime user (`USER 65532:65532` or distroless nonroot variant).
  2. Align exposed port with app (`EXPOSE 5656`) or make runtime port configurable and consistent.
  3. Add `readOnlyRootFilesystem` + dropped Linux capabilities in deployment manifests.

## Medium

- [ ] **[MEDIUM] Production TLS termination expectations are undocumented and unenforced**  
  **Evidence:** Server uses `ListenAndServe` (plain HTTP) in `internal/app/server/routes.go`; no explicit HTTPS mode or strict proxy requirement.  
  **Risk:** Misdeployment can expose auth tokens over plaintext networks.  
  **Recommended fix (exact):**
  1. Document hard requirement: backend must be behind TLS-terminating reverse proxy in production.
  2. Add startup warning/fail-fast when `PRODUCTION=true` and trusted proxy/TLS headers are missing configuration.
  3. Optionally provide native TLS mode envs for direct deployments.

- [ ] **[MEDIUM] Authentication token lifetime/rotation policy is fixed and not externally configurable**  
  **Evidence:** JWT TTL is hardcoded to 7 days (`internal/auth/jwt_handler.go`, `jwtTTL = 24*7h`).  
  **Risk:** Hard to adjust session lifetime for stricter environments and incident response.  
  **Recommended fix (exact):**
  1. Add `JWT_TTL_MINUTES` env with sane bounds (e.g., min 5m, max 10080m).
  2. Enforce short access token + refresh-token pattern if long sessions are required.
  3. Document emergency revocation/secret rotation playbook.

- [ ] **[MEDIUM] No explicit operational runbook for backend incidents**  
  **Evidence:** Repository has general README but no backend production runbook for recovery actions.  
  **Risk:** Slow and inconsistent incident handling (DB outage, Redis outage, key rotation, degraded queues).  
  **Recommended fix (exact):**
  1. Add `backend/RUNBOOK.md` covering: startup checks, probe interpretation, DB/Redis failure modes, token/key rotation, rollback steps, backup/restore validation.
  2. Include “first 15 minutes” incident checklist and commands.

- [ ] **[MEDIUM] Secrets management relies on env vars without documented rotation cadence**  
  **Evidence:** `JWT_SECRET` and `PROXY_ENCRYPTION_KEY` are required and loaded from env; no formal rotation cadence/runbook in backend docs.  
  **Risk:** Secret sprawl and delayed rotation during compromise response.  
  **Recommended fix (exact):**
  1. Integrate with secret manager (Vault/KMS/SSM) for production deployments.
  2. Define rotation cadence and emergency rotation procedure.
  3. Add startup validation for minimum secret length/entropy.

## Quick implementation order (suggested)

1. Fix admin bootstrap takeover path.
2. Add health/readiness probes + recover middleware.
3. Add CI gate and baseline telemetry.
4. Harden container runtime + document TLS/proxy requirements.
5. Publish backend runbook and secret/token operational policies.
