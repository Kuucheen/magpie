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

1. Set a strong `PROXY_ENCRYPTION_KEY` and `JWT_SECRET`.
2. Use persistent volumes for Postgres and backend settings if needed.
3. Put reverse proxy/TLS in front of frontend and backend.
4. Restrict database and redis exposure to private network.
5. Back up Postgres regularly.

## Multi-instance considerations

Leader-based routines use Redis locks. If running multiple backend instances, ensure all instances share the same Redis and database.

For rotating proxy listeners, set per-instance identity vars so instances are distinguishable in the UI and API:

- `MAGPIE_INSTANCE_ID`: stable unique id for the instance.
- `MAGPIE_INSTANCE_NAME`: display name for the instance.
- `MAGPIE_INSTANCE_REGION`: region label for the instance.

Optionally tune listener reconciliation:

- `ROTATING_PROXY_SYNC_INTERVAL_SECONDS` (default `10`).
