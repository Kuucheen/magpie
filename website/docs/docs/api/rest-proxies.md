# REST: Proxies

## `POST /api/addProxies`

Requires auth. Uploads proxies from multipart fields.

Accepted form fields:

- `file`: uploaded proxy text file
- `proxyTextarea`: raw text area input
- `clipboardProxies`: pasted input

Success (`200`):

```json
{
  "proxyCount": 42,
  "details": {
    "submittedCount": 100,
    "parsedCount": 70,
    "invalidFormatCount": 20,
    "invalidIpCount": 5,
    "invalidIpv4Count": 2,
    "invalidPortCount": 3,
    "blacklistedCount": 28,
    "processingMs": 17
  }
}
```

## `GET /api/getProxyCount`

Requires auth. Returns total proxy count for the user.

## `GET /api/getProxyPage/{page}`

Requires auth. Returns paged proxy rows.

Query params:

- `pageSize`
- `search`
- `status=alive|dead`
- repeated `protocol=http|https|socks4|socks5`
- repeated `country`
- repeated `type`
- repeated `anonymity`
- repeated `reputation=good|neutral|poor|unknown`
- `maxTimeout`
- `maxRetries`

Response:

```json
{
  "proxies": [
    {
      "id": 1,
      "ip": "1.2.3.4",
      "port": 8080,
      "estimated_type": "datacenter",
      "response_time": 280,
      "country": "US",
      "anonymity_level": "elite",
      "alive": true,
      "latest_check": "2026-02-12T10:00:00Z",
      "reputation": {
        "overall": {"kind": "overall", "score": 0.89, "label": "good"}
      }
    }
  ],
  "total": 1234
}
```

## `GET /api/proxyFilters`

Requires auth. Returns available filter values:

```json
{
  "countries": ["DE", "US"],
  "types": ["datacenter", "residential"],
  "anonymityLevels": ["elite", "anonymous", "transparent", "N/A"]
}
```

## `GET /api/proxies/{id}`

Requires auth. Returns proxy detail including latest statistic and reputation breakdown.

## `GET /api/proxies/{id}/statistics`

Requires auth. Query param: `limit` (max 500).

Response:

```json
{
  "statistics": [
    {
      "id": 987,
      "alive": true,
      "attempt": 1,
      "response_time": 190,
      "protocol": "http",
      "anonymity_level": "elite",
      "judge": "https://judge.example",
      "created_at": "2026-02-12T09:58:00Z"
    }
  ]
}
```

## `GET /api/proxies/{id}/statistics/{statisticId}`

Requires auth.

Response:

```json
{
  "response_body": "...",
  "regex": "..."
}
```

## `DELETE /api/proxies`

Requires auth. Supports two body modes.

Mode A: selected IDs array.

```json
[101, 102, 103]
```

Mode B: filter object.

```json
{
  "proxies": [101, 102],
  "filter": true,
  "http": true,
  "https": false,
  "socks4": false,
  "socks5": false,
  "maxRetries": 2,
  "maxTimeout": 5000,
  "proxyStatus": "alive",
  "reputationLabels": ["good"],
  "scope": "selected"
}
```

Responses are JSON strings, for example:

- `"Deleted 25 proxies."`
- `"No proxies matched the delete criteria."`
