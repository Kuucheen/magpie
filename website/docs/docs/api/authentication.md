# Authentication

Magpie uses JWT bearer tokens for API authentication.

## Token lifecycle

- Public auth endpoints: `POST /api/register`, `POST /api/login`, `POST /api/forgotPassword`, `POST /api/resetPassword`
- Token rotation endpoint: `POST /api/refreshToken` (requires auth)
- Logout endpoint: `POST /api/logout` (requires auth; revokes current token)
- JWTs contain `user_id`, `role`, `exp`, `iat`, and `jti` claims.
- Signing algorithm: `HS256`
- Secret source: `JWT_SECRET` (required)
- TTL source: `JWT_TTL_MINUTES` (default `10080`, range `15..10080` minutes)

## Authorization header

Send the token on protected endpoints:

```http
Authorization: Bearer <token>
```

If the header is missing, malformed, or invalid, protected endpoints return `401 Unauthorized`.

## Route protection model

- `RequireAuth`: endpoint requires a valid JWT.
- `IsAdmin`: endpoint requires a valid JWT and `role=admin`.
- Auth checks are applied to REST routes in `backend/internal/app/server/routes.go`.

## Revocation behavior

- Tokens are revocable by token id (`jti`) and by user-wide revoke cutoff (used for password change/account deletion/session revocation).
- Revocation state is stored in Redis.
- Password change and successful password reset revoke active sessions under normal revocation-store operation.
- `AUTH_REVOCATION_FAIL_OPEN` controls outage behavior when Redis is unavailable:
  - default: `true` (availability-first)
  - when `false`: revocation-store outages cause strict auth failures.

## Password recovery

- Password recovery uses short-lived, single-use reset tokens stored only as hashes in the database.
- Reset links are generated from `PUBLIC_APP_URL` rather than request headers.
- Reset requests are throttled both by request volume and by target account identifier.
- Default per-email forgot-password throttle is 1 request per 60 seconds.

## GraphQL authentication

- GraphQL endpoint: `POST /api/graphql`
- Uses the same bearer token header.
- If token is missing/invalid, endpoint returns `401 Unauthorized`.

## Quick verification endpoint

- `GET /api/checkLogin`
- Requires auth.
- Returns `200` if token is valid.
