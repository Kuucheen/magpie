# First Login

## Default local behavior

1. Open `http://localhost:5050`.
2. Register a user account.
3. The first user becomes `admin` automatically.

## Production bootstrap behavior

In production-oriented deployments, public registration is often disabled by default.

For controlled first-admin bootstrap windows:

1. Set `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=true`.
2. Set a strong `ADMIN_BOOTSTRAP_TOKEN`.
3. Call `POST /api/register` with header `X-Admin-Bootstrap-Token: <token>`.
4. After bootstrap, disable public registration (`DISABLE_PUBLIC_REGISTRATION=true`).

## Verify backend auth

You can verify auth status with:

- `GET /api/checkLogin` using `Authorization: Bearer <token>`

## Recommended first admin tasks

1. Configure checker/scraper global settings.
2. Add or validate judge endpoints.
3. Configure scrape sources.
4. Set GeoLite API key if geo enrichment is needed.
5. Review website blacklist policy.
6. Review auth and observability hardening envs.

## JWT behavior

- Login/register returns JWT.
- JWT is expected in `Authorization: Bearer <token>`.
- `POST /api/refreshToken` rotates and revokes the old token.
- `POST /api/logout` revokes the current token.
- Role-based authorization protects admin endpoints.
