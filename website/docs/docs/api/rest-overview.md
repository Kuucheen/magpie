# REST API Overview

## Base URLs

Application REST endpoints are served under:

```text
/api
```

Examples:

- `POST /api/login`
- `GET /api/getProxyPage/1`
- `POST /api/rotatingProxies`

Observability endpoints are served at root (not under `/api`):

- `GET /healthz`
- `GET /readyz`
- `GET /metrics`

## Authentication headers

Protected API routes use JWT bearer auth:

```http
Authorization: Bearer <token>
```

Additional headers used by specific endpoints:

- `X-Admin-Bootstrap-Token`: first-admin bootstrap via `POST /api/register` when public bootstrap token mode is enabled.
- `X-Observability-Token`: optional token for `/healthz`, `/readyz`, `/metrics` when public observability is disabled.

## Data formats

- Most endpoints use JSON request/response bodies.
- Upload endpoints use `multipart/form-data`.
- Some endpoints return a JSON string payload instead of an object.

## Endpoint groups

- Auth and user: register/login/token lifecycle, password/account, user settings
- Proxies: upload, list/filter, detail/stats, delete, export
- Rotating proxies: create/list/delete/manual rotate + instance discovery
- Scraping sources: upload/list/detail/delete/robots checks
- Admin/system: global settings, dashboard info, releases
- GraphQL: dashboard + user settings API
- Observability: health/readiness/Prometheus metrics

## Pagination and filtering

Proxy list endpoints support:

- Path page index: `.../getProxyPage/{page}`
- Optional `pageSize`
- Optional `search`
- Optional filter query params:
  - `status=alive|dead`
  - repeated: `protocol`, `country`, `type`, `anonymity`, `reputation`
  - numeric: `maxTimeout`, `maxRetries`

Scrape-source proxy lists use the same filter model plus `page`.

## Request size limits

The backend enforces body-size limits from environment config:

- `API_UPLOAD_MAX_BODY_BYTES` (multipart uploads)
- `API_JSON_MAX_BODY_BYTES` (JSON routes)
- `GRAPHQL_MAX_QUERY_BYTES` (GraphQL query string)

Exceeded limits return `413 Request Entity Too Large`.

## Response status conventions

Common status codes:

- `200 OK`: success
- `201 Created`: resource created
- `204 No Content`: success with empty body
- `400 Bad Request`: invalid payload/params
- `401 Unauthorized`: missing/invalid token
- `403 Forbidden`: authenticated but missing required permission
- `404 Not Found`: resource not found
- `409 Conflict`: duplicate/conflicting state
- `413 Request Entity Too Large`: request/query too large
- `429 Too Many Requests`: auth rate limits exceeded (includes `Retry-After`)
- `500 Internal Server Error`: unexpected server error
- `503 Service Unavailable`: transient backend dependency/port capacity limits
