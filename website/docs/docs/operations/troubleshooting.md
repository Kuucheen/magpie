# Troubleshooting

## `401 Unauthorized` on API calls

Possible causes:

- Missing `Authorization: Bearer <token>`
- Expired or invalid JWT
- Token signed with different `JWT_SECRET`
- Token revoked (logout/password change/password reset/account delete)

## `429 Too Many Requests` on auth routes

Auth endpoints are rate limited.

Check:

- `AUTH_*` rate-limit env values
- response `Retry-After` header
- whether multiple users are sharing a single untrusted proxy IP without proper `TRUSTED_PROXY_CIDRS`
- whether forgot-password/reset-password requests are hitting identifier-based throttles in addition to request-volume throttles

Common defaults:

- forgot-password per-email limit: `AUTH_FORGOT_PASSWORD_LIMIT_PER_EMAIL=1`
- forgot-password/reset-password request window: `AUTH_REQUEST_RATE_LIMIT_WINDOW_SECONDS=60`

## Password reset email not arriving

Check:

- mail envs: `MAIL_FROM_ADDRESS`, `MAIL_FROM_NAME`, `SMTP_HOST`, `SMTP_PORT`, `SMTP_USERNAME`, `SMTP_PASSWORD`
- `PUBLIC_APP_URL` is set correctly
- SMTP provider accepts the sender address and credentials
- outbox worker envs if mail is backing up or retrying (`EMAIL_OUTBOX_*`, `EMAIL_RETRY_BASE_SECONDS`, `EMAIL_MAX_ATTEMPTS`)
- backend logs for SMTP, outbox, or configuration errors

## Password reset link is invalid or expired

Possible causes:

- token already used
- token expired (`PASSWORD_RESET_TOKEN_TTL_MINUTES`)
- a newer forgot-password request replaced the previous token
- copied link was truncated or modified

## Registration fails in production

Check registration policy envs:

- `DISABLE_PUBLIC_REGISTRATION`
- `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP`

## `Registration failed: CORS origin is not allowed`

Magpie's default Docker stack only allows localhost frontend origins by default.

If you run Magpie on a NAS, server, or another machine and open the UI from a browser using a LAN IP or hostname, set:

- `CORS_ALLOWED_ORIGINS=http://<host-or-ip>:5050`

Examples:

- `CORS_ALLOWED_ORIGINS=http://192.168.6.143:5050`
- `CORS_ALLOWED_ORIGINS=http://nas.local:5050`

For multiple origins, use a comma-separated list.

If you intentionally want to allow any origin in a trusted environment, set:

- `CORS_ALLOWED_ORIGINS=*`

## `403 Forbidden` on `/healthz`, `/readyz`, or `/metrics`

Observability protection is active.

Use one of:

- loopback request path
- valid `X-Observability-Token`
- `ALLOW_PUBLIC_OBSERVABILITY_ENDPOINTS=true`

## `413 Request Entity Too Large`

Request exceeded configured limits.

Check:

- `API_UPLOAD_MAX_BODY_BYTES`
- `API_JSON_MAX_BODY_BYTES`
- `GRAPHQL_MAX_QUERY_BYTES`

## GraphQL request rejected before resolver

Possible guard violations:

- query depth exceeds `GRAPHQL_MAX_DEPTH`
- field count exceeds `GRAPHQL_MAX_FIELDS`
- introspection is disabled (`GRAPHQL_ALLOW_INTROSPECTION=false`)

## Rotating proxy creation fails

Possible causes:

- protocol not enabled for user
- name conflict
- missing auth fields when auth required
- no free listener ports in configured range
- selected instance unavailable or out of ports

## Scrape source rejected

Possible causes:

- URL appears in `website_blacklist`
- URL resolves to unsafe private/loopback/reserved target while `ALLOW_PRIVATE_NETWORK_EGRESS=false`

## Old data not decrypting after restart/update

Likely cause: `PROXY_ENCRYPTION_KEY` changed.

Fix: restart with previous key.

## Backend cannot connect to Redis/Postgres

Check effective env values:

- Redis: `REDIS_MODE`, `REDIS_URL` / `redisUrl`, plus sentinel vars in sentinel mode
- DB: `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USERNAME`, `DB_PASSWORD`, `DB_SSLMODE`

## Build or install dependency issues

Delete `node_modules` and lockfile in the affected workspace and reinstall.
