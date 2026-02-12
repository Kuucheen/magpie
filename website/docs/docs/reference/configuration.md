# Configuration

Global config is modeled by `config.Config` and persisted in `data/settings.json`.

Default values come from `backend/internal/config/default_settings.json`.

## Top-level sections

- `protocols`
- `checker`
- `scraper`
- `proxy_limits`
- `runtime`
- `geolite`
- `blacklist_timer`
- `blacklist_sources`
- `website_blacklist`

## Example

```json
{
  "protocols": {
    "http": false,
    "https": true,
    "socks4": false,
    "socks5": false
  },
  "proxy_limits": {
    "enabled": false,
    "max_per_user": 1000,
    "exclude_admins": true
  }
}
```

## Runtime note

When running without a persistent backend filesystem mount, settings written to `data/settings.json` may not survive container replacement unless explicitly persisted.
