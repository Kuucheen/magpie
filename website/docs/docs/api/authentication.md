# Authentication

Magpie uses JWT bearer tokens for API authentication.

## Token lifecycle

- Public auth endpoints: `POST /api/register`, `POST /api/login`
- Both endpoints return a signed JWT on success.
- JWTs contain `user_id`, `role`, and `exp` claims.
- Token expiry is 7 days from issuance.
- Signing algorithm: `HS256`
- Secret source: `JWT_SECRET` (fallback default in code if not set)

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

## GraphQL authentication

- GraphQL endpoint: `POST /api/graphql`
- Uses the same bearer token header.
- If token is missing/invalid, authenticated GraphQL resolvers return an unauthenticated GraphQL error.

## Quick verification endpoint

- `GET /api/checkLogin`
- Requires auth.
- Returns `200` if token is valid.
