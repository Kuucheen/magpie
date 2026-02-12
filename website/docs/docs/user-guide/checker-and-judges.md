# Checker and Judges

Checker behavior combines global settings and per-user settings.

## Per-user settings

User settings include:

- enabled protocols (`http`, `https`, `socks4`, `socks5`)
- timeout and retries
- transport protocol
- `use_https_for_socks`
- auto-remove settings
- judge list (`url`, `regex`)

REST endpoints:

- `GET /api/user/settings`
- `POST /api/user/settings`

GraphQL equivalent:

- Query: `viewer.settings`
- Mutation: `updateUserSettings(input: ...)`

## Judge notes

- Judge URLs are validated against website blacklist.
- User judge relations are synchronized into in-memory runtime cache.
