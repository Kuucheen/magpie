# REST: Scraping Sources

## `GET /api/getScrapingSourcesCount`

Requires auth. Returns source count for the current user.

## `GET /api/getScrapingSourcesPage/{page}`

Requires auth. Returns paged source summaries.

## `POST /api/scrapingSources`

Requires auth. Uploads sources from multipart form data.

Accepted form fields:

- `file`
- `scrapeSourceTextarea`
- `clipboardScrapeSources`

Success (`200`):

```json
{"sourceCount": 18}
```

If blocked by website blacklist, response is `400` with details:

```json
{
  "error": "One or more scrape sources are blocked",
  "blocked_sources": ["https://blocked.example/list.txt"],
  "websiteBlacklist": ["blocked.example"]
}
```

## `DELETE /api/scrapingSources`

Requires auth.

Request body is an array of scrape source IDs:

```json
[12, 13, 14]
```

Response is a JSON string, for example: `"Deleted 3 scraping sources."`.

## `GET /api/scrapingSources/{id}`

Requires auth. Returns detailed source stats.

## `GET /api/scrapingSources/{id}/proxies`

Requires auth. Returns paged proxies associated with a source.

Query params:

- `page`
- `pageSize`
- `search`
- same filter params as proxy list:
  - `status`, `protocol`, `country`, `type`, `anonymity`, `reputation`, `maxTimeout`, `maxRetries`

## `GET /api/scrapingSources/check?url=...`

Requires auth. Checks `robots.txt` allowance.

Response:

```json
{
  "allowed": true,
  "robots_found": true,
  "error": ""
}
```

## `GET /api/scrapingSources/respectRobots`

Requires auth.

Response:

```json
{
  "respect_robots_txt": true
}
```
