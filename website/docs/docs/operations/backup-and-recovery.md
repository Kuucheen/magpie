# Backup and Recovery

## What to back up

1. PostgreSQL data (`postgres_data` volume)
2. Deployment env/secrets (`PROXY_ENCRYPTION_KEY`, `JWT_SECRET`)
3. Optional backend settings file persistence if mounted separately

## Why secrets matter for recovery

Without original `PROXY_ENCRYPTION_KEY`, encrypted proxy secrets cannot be decrypted after restore.

## Recovery checklist

1. Restore Postgres backup.
2. Restore exact secret values.
3. Start Redis, Postgres, backend, frontend.
4. Verify login and proxy visibility.
