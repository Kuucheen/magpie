# Security

## Secrets

- Never use default/fallback secrets in production.
- Persist and protect `PROXY_ENCRYPTION_KEY`.
- Set explicit `JWT_SECRET`.

## Auth model

- JWT bearer auth on protected routes
- role-based authorization for admin endpoints

## Data handling

Magpie encrypts proxy secret fields using AES-GCM with key material derived from `PROXY_ENCRYPTION_KEY`.

## Network hardening

- Limit public exposure of backend admin/API ports
- Restrict Postgres/Redis to trusted network
- Terminate TLS at an ingress/reverse proxy layer

## Website blacklist controls

Website blacklist helps prevent usage of blocked judge/scrape/blacklist URLs and can remove existing blocked relations.
