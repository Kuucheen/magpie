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
- Settings: protocol/checker settings, judges, scraping source URLs
- Dashboard: counts and breakdowns
- Proxy metrics: `proxyCount`, `proxyLimit`, `proxyHistory`, `proxySnapshots`
- Paged resources: `proxies(page: Int!)`, `scrapeSources(page: Int!)`

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
    proxySnapshots {
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
  }
}
```

Supported input fields are optional booleans/integers plus `judges` and `scrapingSources` arrays.

Current behavior note:

- GraphQL settings mutation delegates persistence to `database.UpdateUserSettings`.
- In the current backend code, `scrapingSources` is accepted in input but scrape-source persistence is handled by REST source endpoints.

## Error format

GraphQL errors follow standard GraphQL shape:

```json
{
  "errors": [
    {"message": "unauthenticated"}
  ]
}
```
