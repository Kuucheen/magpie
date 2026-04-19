# Security

## Secrets and startup validation

- Always set strong, unique `PROXY_ENCRYPTION_KEY` and `JWT_SECRET`.
- Prefer strict validation in production (`STRICT_SECRET_VALIDATION=true`).
- Keep secret values stable across restarts and restore operations.

## Auth and registration hardening

- Protected endpoints use JWT bearer auth.
- Admin-only routes require `role=admin`.
- Restrict account creation in production:
  - `DISABLE_PUBLIC_REGISTRATION=true`
  - enable `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=true` only during controlled bootstrap windows
- Enforce auth-rate limiting defaults unless you have measured reasons to relax them.
- Registration, password reset, and password change all enforce the same strong password policy.
- User email identity is normalized and stored/enforced case-insensitively.

## Password recovery hardening

- Password reset links are generated from fixed `PUBLIC_APP_URL` rather than request headers.
- Reset tokens are random, single-use, short-lived, and stored only as hashes.
- Successful password reset removes all outstanding reset tokens for the user.
- Forgot-password and reset-password are throttled by both request volume and account identifier.
- Default forgot-password per-email throttle is 1 request per minute.
- Reset and confirmation emails are written to the durable DB outbox and retried asynchronously across backend instances.

## SMTP and transport hardening

- SMTP delivery fails closed unless TLS is established.
- Port `587` requires successful `STARTTLS` before auth or message submission.
- Port `465` uses implicit TLS.
- Set `SMTP_USERNAME` and `SMTP_PASSWORD` together or leave both unset.

## JWT revocation behavior

- Logout, password-change, and account-delete flows revoke existing sessions.
- Successful password reset also revokes active sessions under normal revocation-store operation.
- Revocation state lives in Redis.
- Default `AUTH_REVOCATION_FAIL_OPEN=true` favors availability during Redis outage.
- For strict security posture, set `AUTH_REVOCATION_FAIL_OPEN=false` and ensure Redis HA.

## Observability endpoint protection

`/healthz`, `/readyz`, and `/metrics` are protected in production by default.

Allow access through one of:

- loopback source IP
- `X-Observability-Token` matching `OBSERVABILITY_TOKEN`
- explicit `ALLOW_PUBLIC_OBSERVABILITY_ENDPOINTS=true`

## Network and outbound safeguards

- Restrict backend/admin API exposure behind trusted ingress.
- Keep Postgres and Redis on private networks.
- Outbound URL/IP validation blocks private/loopback/reserved targets by default.
- Only set `ALLOW_PRIVATE_NETWORK_EGRESS=true` in controlled trusted environments.

## Proxy header trust

- Configure `TRUSTED_PROXY_CIDRS` so client IP extraction trusts forwarding headers only from known reverse proxies.
- Avoid wildcard trust ranges such as `0.0.0.0/0`.

## Browser and transport hardening

- Enable security headers (`SECURITY_HEADERS_ENABLED=true`, default).
- Keep `CORS_ALLOWED_ORIGINS` specific; do not use `*` on internet-facing deployments.
- Terminate TLS at ingress/reverse proxy and use hardened TLS configs.

## Website blacklist controls

Website blacklist helps prevent usage of blocked judge/scrape/blacklist URLs and can remove existing blocked relations.
