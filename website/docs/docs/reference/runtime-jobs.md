# Runtime Jobs

Magpie starts several long-running routines on backend startup.

## Core routines

- judge refresh routine
- proxy statistics routine
- proxy statistics retention routine
- proxy history routine
- proxy snapshot routine
- proxy geo refresh routine
- orphan cleanup routine
- GeoLite update routine
- blacklist refresh routine
- checker thread dispatcher
- scraper page pool manager
- scraper thread dispatcher
- email delivery routine (durable outbox worker)
- password reset token cleanup routine

## Leadership and coordination

Some routines are executed with leader-election semantics using Redis locks to avoid duplicate execution across instances.

This includes the email delivery routine, which processes outbox rows and retries failed deliveries asynchronously.

## Timers

Most intervals are configured in global settings (`checker_timer`, `scraper_timer`, `judge_timer`, `blacklist_timer`, runtime timers, GeoLite update timer).

Retention routine interval/limits are configured via environment variables:

- `PROXY_STATISTICS_RETENTION_INTERVAL` / `PROXY_STATISTICS_RETENTION_INTERVAL_MINUTES`
- `PROXY_STATISTICS_RETENTION_DAYS`
- `PROXY_STATISTICS_RESPONSE_RETENTION_DAYS`
- `PROXY_STATISTICS_RETENTION_BATCH_SIZE`
- `PROXY_STATISTICS_RETENTION_MAX_BATCHES`

Password recovery maintenance is configured via:

- `PASSWORD_RESET_CLEANUP_INTERVAL` / `PASSWORD_RESET_CLEANUP_INTERVAL_MINUTES`
- `EMAIL_OUTBOX_POLL_INTERVAL` / `EMAIL_OUTBOX_POLL_INTERVAL_SECONDS`
- `EMAIL_OUTBOX_BATCH_SIZE`
- `EMAIL_PROCESSING_TIMEOUT` / `EMAIL_PROCESSING_TIMEOUT_SECONDS`
- `EMAIL_OUTBOX_RETENTION_HOURS`
- `EMAIL_RETRY_BASE_SECONDS`
- `EMAIL_MAX_ATTEMPTS`
