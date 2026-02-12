# Scraping Sources

Scrape sources are websites Magpie crawls to discover proxies.

## Manage sources

- `GET /api/getScrapingSourcesCount`
- `GET /api/getScrapingSourcesPage/{page}`
- `POST /api/scrapingSources`
- `DELETE /api/scrapingSources`
- `GET /api/scrapingSources/{id}`
- `GET /api/scrapingSources/{id}/proxies`

## Add sources input

`POST /api/scrapingSources` accepts multipart content from:

- `file`
- `scrapeSourceTextarea`
- `clipboardScrapeSources`

## Robots check

Use these endpoints before enabling a source:

- `GET /api/scrapingSources/check?url=...`
- `GET /api/scrapingSources/respectRobots`

## Blacklist interaction

If a source URL is present in website blacklist, save/check requests return validation errors.
