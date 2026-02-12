# Managing Proxies

## Import proxies

`POST /api/addProxies` accepts multipart input from any combination of:

- `file`
- `proxyTextarea`
- `clipboardProxies`

Accepted formats include:

- `ip:port`
- `ip:port:user:pass`
- `user:pass@ip:port`

Invalid entries are counted and returned in response statistics.

## Browse and filter

Use:

- `GET /api/getProxyPage/{page}`
- `GET /api/proxyFilters`

Filter/query parameters include:

- `search`
- `pageSize`
- `status=alive|dead`
- repeated: `protocol`, `country`, `type`, `anonymity`, `reputation`
- `maxTimeout`, `maxRetries`

## Proxy detail and stats

- `GET /api/proxies/{id}`
- `GET /api/proxies/{id}/statistics?limit=...`
- `GET /api/proxies/{id}/statistics/{statisticId}`

## Delete proxies

`DELETE /api/proxies` supports two body formats:

1. JSON array of IDs: `[1,2,3]`
2. Filter settings object (`scope`, protocol filters, timeout/retries, reputation labels)

## Export proxies

`POST /api/user/export` returns formatted text.

Output format is a template string; placeholders include:

- `protocol`
- `ip`
- `port`
- `username`
- `password`
- `country`
- `alive`
- `type`
- `time`
- `reputation`
- `reputation_label`
- `reputation_score`

Example format:

```text
protocol ip:port username password country alive type time reputation_score reputation_label
```
