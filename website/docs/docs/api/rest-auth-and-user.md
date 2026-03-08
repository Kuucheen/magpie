# REST: Auth and User

## `POST /api/register`

Registers a new user.

Request:

```json
{
  "email": "admin@example.com",
  "password": "change-me-now"
}
```

Behavior:

- Validates email format.
- Requires password length `>= 8`.
- First user in DB becomes `admin`; later users become `user`.
- Registration policy can be restricted by env flags (`DISABLE_PUBLIC_REGISTRATION`, `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP`).
- Route is rate-limited (`429` + `Retry-After`).

Success (`201`):

```json
{
  "token": "<jwt>",
  "warning": "Default scrape sources could not be queued and were rolled back. Add sources again later."
}
```

`warning` is optional and appears only when default scrape-source bootstrap could not be queued.

## `POST /api/login`

Request:

```json
{
  "email": "admin@example.com",
  "password": "change-me-now"
}
```

Success (`200`):

```json
{
  "token": "<jwt>",
  "role": "admin"
}
```

Notes:

- Invalid credentials return `401` with `{"error":"Invalid email or password"}`.
- Login is protected by request and failure-based rate limiting (`429` + `Retry-After`).

## `POST /api/refreshToken`

Requires auth.

Rotates the current bearer token and revokes the previous token.

Success (`200`):

```json
{
  "token": "<new-jwt>",
  "role": "admin"
}
```

## `POST /api/logout`

Requires auth.

Revokes the current bearer token.

- Success: `204 No Content`

## `GET /api/checkLogin`

Requires auth. Returns `200` if token is valid.

## `POST /api/changePassword`

Requires auth.

Request:

```json
{
  "oldPassword": "old-value",
  "newPassword": "new-value"
}
```

Success returns a JSON string body: `"Password changed successfully"`.

## `POST /api/deleteAccount`

Requires auth.

Request:

```json
{
  "password": "current-password"
}
```

Success returns a JSON string body: `"Account deleted successfully"`.

## `GET /api/user/settings`

Requires auth. Returns user checker/scraper settings.

Response shape:

```json
{
  "http_protocol": true,
  "https_protocol": true,
  "socks4_protocol": false,
  "socks5_protocol": false,
  "timeout": 7500,
  "retries": 2,
  "UseHttpsForSocks": true,
  "transport_protocol": "tcp",
  "auto_remove_failing_proxies": false,
  "auto_remove_failure_threshold": 3,
  "judges": [{"url": "https://example/judge", "regex": "..."}],
  "scraping_sources": ["https://example/source"],
  "proxy_list_columns": ["ip", "country"],
  "scrape_source_proxy_columns": ["ip", "protocol"],
  "scrape_source_list_columns": ["url", "proxy_count"]
}
```

## `POST /api/user/settings`

Requires auth. Saves user protocol/checker settings and judges.

Request uses the same fields as `GET /api/user/settings`.

Success (`200`):

```json
{"message": "Settings saved successfully"}
```

Current implementation note:

- `scraping_sources` may be accepted in this payload but scrape-source persistence is managed by `POST /api/scrapingSources` and `DELETE /api/scrapingSources`.

## `GET /api/user/role`

Requires auth.

Response body is a JSON string: `"admin"` or `"user"`.

## `POST /api/user/export`

Requires auth. Exports proxies by selected IDs and/or filters.

Request:

```json
{
  "proxies": [101, 102],
  "filter": true,
  "http": true,
  "https": true,
  "socks4": false,
  "socks5": false,
  "maxRetries": 2,
  "maxTimeout": 5000,
  "proxyStatus": "alive",
  "reputationLabels": ["good", "neutral"],
  "outputFormat": "ip:port"
}
```

Response:

- `Content-Type: text/plain`
- Body contains the formatted export content.
