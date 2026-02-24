# Deployment

## Default Docker deployment

From repo root:

```bash
docker compose up -d
```

Services:

- frontend: `:5050`
- backend: `:5656`
- postgres: `:5434`
- redis: `:8946`

## Backend image/runtime notes

- Backend uses Go binary in distroless runtime image.
- Frontend serves static build via nginx.

## Production recommendations

1. Set strong `PROXY_ENCRYPTION_KEY` and `JWT_SECRET`.
2. Optionally override `DB_USERNAME`, `DB_PASSWORD`, and `DB_NAME` (Compose provides defaults).
3. Set `DB_SSLMODE=require` (or `verify-ca`/`verify-full`) when using an external/production Postgres endpoint.
4. Use persistent volumes for Postgres and backend settings if needed.
5. Put reverse proxy/TLS in front of frontend and backend.
6. Restrict database and redis exposure to private network.
7. Back up Postgres regularly.

Default Compose includes CPU/memory limits and reservations per service. Tune them with `*_CPU_LIMIT`, `*_MEMORY_LIMIT`, `*_CPU_RESERVATION`, and `*_MEMORY_RESERVATION` env vars.

Before production rollout, run the load/soak gate documented in [Performance Validation](./performance-validation.md).

## Multi-instance considerations

Leader-based routines use Redis locks. If running multiple backend instances, ensure all instances share the same Redis and database.

For rotating proxy listeners, set per-instance identity vars so instances are distinguishable in the UI and API:

- `MAGPIE_INSTANCE_ID`: stable unique id for the instance.
- `MAGPIE_INSTANCE_NAME`: display name for the instance.
- `MAGPIE_INSTANCE_REGION`: region label for the instance.

Optionally tune listener reconciliation:

- `ROTATING_PROXY_SYNC_INTERVAL_SECONDS` (default `10`).
