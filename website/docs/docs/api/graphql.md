# GraphQL API

## Endpoint

```text
POST /api/graphql
```

GraphQL uses the same bearer token auth as REST (`Authorization: Bearer <token>`).

## Request envelope

```json
{
  "query": "query { viewer { id email } }",
  "variables": {}
}
```

## Root fields

Current schema exposes:

- `Query.viewer`
- `Mutation.updateUserSettings(input: UpdateUserSettingsInput!)`

## Viewer data

`viewer` includes:

- Identity: `id`, `email`, `role`
- Settings: protocol/checker settings, judges, scraping source URLs, table-column preferences
- Dashboard: counts and breakdowns
- Proxy metrics: `proxyCount`, `proxyLimit`, `proxyHistory`, `proxySnapshots`
- Paged resources: `proxies(page: Int!)`, `scrapeSources(page: Int!)`
- Scrape-source URL helper list: `scrapeSourceUrls`

## Example dashboard query

```graphql
query DashboardData($proxyPage: Int!) {
  viewer {
    dashboard {
      totalChecks
      totalScraped
      reputationBreakdown { good neutral poor unknown }
      countryBreakdown { country count }
    }
    proxyCount
    proxyLimit
    proxies(page: $proxyPage) {
      page
      pageSize
      totalCount
      items {
        id
        ip
        port
        estimatedType
        responseTime
        country
        anonymityLevel
        alive
        latestCheck
      }
    }
    proxyHistory(limit: 168) { count recordedAt }
    proxySnapshots(limit: 168) {
      alive { count recordedAt }
      scraped { count recordedAt }
    }
    scrapeSourceCount
  }
}
```

## Update settings mutation

```graphql
mutation Update($input: UpdateUserSettingsInput!) {
  updateUserSettings(input: $input) {
    httpProtocol
    httpsProtocol
    socks4Protocol
    socks5Protocol
    timeout
    retries
    useHttpsForSocks
    autoRemoveFailingProxies
    autoRemoveFailureThreshold
    judges { url regex }
    scrapingSources
    proxyListColumns
    scrapeSourceProxyColumns
    scrapeSourceListColumns
  }
}
```

Supported input fields are optional booleans/integers plus `judges`, `scrapingSources`, and column-list arrays.

Current behavior note:

- GraphQL settings mutation delegates persistence to `database.UpdateUserSettings`.
- In current backend behavior, scrape-source lifecycle is still managed by REST scrape-source endpoints.

## Query guardrails

GraphQL requests are validated before execution:

- Max query bytes: `GRAPHQL_MAX_QUERY_BYTES` (default `16384`)
- Max depth: `GRAPHQL_MAX_DEPTH` (default `12`)
- Max field count: `GRAPHQL_MAX_FIELDS` (default `250`)
- Introspection disabled by default unless `GRAPHQL_ALLOW_INTROSPECTION=true`

Violations return HTTP `400` or `413` with `{"error": ...}`.

## Error format

GraphQL resolver errors follow standard GraphQL response shape:

```json
{
  "errors": [
    {"message": "unauthenticated"}
  ]
}
```
