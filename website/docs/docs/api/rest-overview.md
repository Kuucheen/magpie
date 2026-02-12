# REST API Overview

## Base URL

All REST endpoints are served under:

```text
/api
```

Examples:

- `POST /api/login`
- `GET /api/getProxyPage/1`
- `POST /api/rotatingProxies`

## Data formats

- Most endpoints use JSON request/response bodies.
- Upload endpoints use `multipart/form-data`.
- Some endpoints return a JSON string payload instead of an object.

## Endpoint groups

- Auth and user: register/login, password/account, user settings
- Proxies: upload, list/filter, detail/stats, delete, export
- Rotating proxies: create/list/delete/manual rotate
- Scraping sources: upload/list/detail/delete/robots checks
- Admin/system: global settings, dashboard info, releases
- GraphQL: dashboard + user settings API

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

## Response status conventions

Common status codes:

- `200 OK`: success
- `201 Created`: resource created
- `204 No Content`: successful delete with empty body
- `400 Bad Request`: invalid payload/params
- `401 Unauthorized`: missing/invalid token
- `403 Forbidden`: authenticated but missing admin role
- `404 Not Found`: resource not found
- `409 Conflict`: duplicate/conflicting state
- `500 Internal Server Error`: unexpected server error
