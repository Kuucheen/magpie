# Error Handling

Magpie currently returns a mix of JSON-object errors, plain-text auth middleware errors, and JSON string messages on some endpoints.

## JSON object error format

Most route handlers return:

```json
{"error": "message"}
```

This is the default shape from `writeError(...)`.

## Plain-text auth errors

Middleware-based auth checks may return plain-text HTTP errors:

- `401 Unauthorized`
- `403 Forbidden`

## Extended JSON error formats

Some endpoints return additional metadata:

- Blocked global config URLs:
  - `error`, `blocked_websites`, `website_blacklist`
- Blocked scrape-source uploads:
  - `error`, `blocked_sources`, `websiteBlacklist`
- Blocked user judge URLs:
  - `error`, `blocked_websites`

## Endpoint-specific error statuses

Common patterns:

- `400`: invalid payload/params or validation failure
- `401`: missing/invalid token
- `403`: admin role required
- `404`: missing resource
- `409`: conflict (for example duplicate rotating proxy name or no alive proxy to rotate)
- `503`: rotating-proxy listen port pool exhausted
- `500`: internal failure

## String responses that are not error objects

Some success/failure informational responses are JSON strings, e.g.:

- `"Deleted 25 proxies."`
- `"No proxies matched the delete criteria."`

Handle these as text messages, not structured objects.
