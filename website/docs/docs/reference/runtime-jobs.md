# Runtime Jobs

Magpie starts several long-running routines on backend startup.

## Core routines

- judge refresh routine
- proxy statistics routine
- proxy history routine
- proxy snapshot routine
- proxy geo refresh routine
- orphan cleanup routine
- GeoLite update routine
- blacklist refresh routine
- checker thread dispatcher
- scraper page pool manager
- scraper thread dispatcher

## Leadership and coordination

Some routines are executed with leader-election semantics using Redis locks to avoid duplicate execution across instances.

## Timers

Most intervals are configured in global settings (`checker_timer`, `scraper_timer`, `judge_timer`, `blacklist_timer`, runtime timers, GeoLite update timer).
