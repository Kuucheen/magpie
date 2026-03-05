# REST: Admin, System, and Observability

## `POST /api/saveSettings` (admin)

Requires admin role. Persists global configuration.

Request body is the full `config.Config` object (see [Configuration](../reference/configuration.md)).

Possible blocked-website response (`400`):

```json
{
  "error": "One or more URLs are blocked by website blacklist",
  "blocked_websites": {
    "judges": ["https://..."],
    "scrape_sites": ["https://..."],
    "blacklist_sources": ["https://..."],
    "ip_lookup": ["https://..."]
  },
  "website_blacklist": ["example.com"]
}
```

Success (`200`):

```json
{"message": "Configuration updated successfully"}
```

Side effects when config changes:

- Website blacklist cleanup across user judges/sources
- Queue cleanup for removed scrape sources
- GeoLite update job trigger when API key is set
- Blacklist refresh trigger when new blacklist source URLs are added

## `GET /api/global/settings` (admin)

Requires admin role. Returns full current global config.

## `GET /api/getDashboardInfo`

Requires auth. Returns dashboard metrics used by the UI.

Includes totals, weekly totals, reputation/country breakdowns, judge success aggregates, and optional `top_reputation_proxy`.

## `GET /api/releases`

Public endpoint (no auth required).

Response:

```json
{
  "releases": [
    {
      "id": 1,
      "tagName": "v1.2.3",
      "title": "Release title",
      "body": "...",
      "htmlUrl": "https://github.com/...",
      "publishedAt": "2026-02-01T12:00:00Z",
      "prerelease": false
    }
  ],
  "build": {
    "buildVersion": "abcdef1",
    "builtAt": "2026-02-01T10:00:00Z"
  }
}
```

## Observability endpoints

- `GET /healthz`: process liveness and build metadata.
- `GET /readyz`: dependency readiness (`database`, `redis`, `startup_queue_bootstrap`) with `ready`, `degraded`, or `not_ready` status.
- `GET /metrics`: Prometheus metrics endpoint.

These routes are wrapped by observability protection:

- In non-production mode, public access is allowed by default.
- In production mode, public access is denied by default unless:
  - caller is loopback, or
  - `X-Observability-Token` matches `OBSERVABILITY_TOKEN`, or
  - `ALLOW_PUBLIC_OBSERVABILITY_ENDPOINTS=true`.

Example metrics families exposed by `/metrics`:

- `magpie_http_requests_total`
- `magpie_http_request_duration_seconds`
- `magpie_auth_failures_total`
- `magpie_rate_limit_blocks_total`
- `magpie_rotating_proxy_errors_total`
