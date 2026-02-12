# Navigation

The Angular app routes map to these core views:

- `/dashboard`: overview and KPIs
- `/proxies`: proxy list and filters
- `/proxies/:id`: proxy details and statistics
- `/addProxies`: proxy import workflow
- `/rotating`: rotating proxy management
- `/scraper`: scrape source list
- `/scraper/:id`: scrape source details and related proxies
- `/checker/settings`: per-user checker settings
- `/checker/judges`: per-user judge settings
- `/account`: account and password actions
- `/notifications`: release/build notifications

Admin-only routes:

- `/global/checker`
- `/global/scraper`
- `/global/blacklist`
- `/global/other`
