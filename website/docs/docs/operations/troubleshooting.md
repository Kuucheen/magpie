# Troubleshooting

## Old data not decrypting after restart/update

Likely cause: `PROXY_ENCRYPTION_KEY` changed.

Fix: restart with previous key.

## `401 Unauthorized` on API calls

- Missing `Authorization: Bearer <token>`
- Expired or invalid JWT
- Token created with different `JWT_SECRET`

## Rotating proxy creation fails

Possible causes:

- protocol not enabled for user
- name conflict
- missing auth fields when auth required
- no free listener ports in configured range

## Scrape source rejected as blocked

URL appears in `website_blacklist`.

## Backend cannot connect to Redis/Postgres

Check effective env values:

- `redisUrl`
- `DB_HOST`, `DB_PORT`, `DB_NAME`, `DB_USERNAME`, `DB_PASSWORD`

## Build or install dependency issues

Delete `node_modules` and lockfile in the affected workspace and reinstall.
