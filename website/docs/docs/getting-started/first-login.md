# First Login

## Create the first user

1. Open `http://localhost:5050`.
2. Register a user account.
3. The first user becomes `admin` automatically.

## Verify backend auth

You can verify auth status with:

- `GET /api/checkLogin` using `Authorization: Bearer <token>`

## Recommended first admin tasks

1. Configure checker/scraper global settings.
2. Add or validate judge endpoints.
3. Configure scrape sources.
4. Set GeoLite API key if geo enrichment is needed.
5. Review website blacklist policy.

## JWT behavior

- Login/register returns JWT.
- JWT is expected in `Authorization: Bearer <token>`.
- Role-based authorization protects admin endpoints.
