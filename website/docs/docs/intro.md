---
sidebar_position: 1
slug: /
---

# Magpie Documentation

Magpie is a self-hosted proxy manager for teams and individuals who need to ingest, validate, score, and rotate large proxy pools.

This docs site is intended to be the complete technical documentation for Magpie, based on the current codebase in this repository.

## What Magpie does

- Scrapes proxies from user-defined sources
- Continuously checks proxies for liveness and quality
- Computes reputation labels and scores
- Exposes rotating proxy endpoints over HTTP/HTTPS/SOCKS
- Provides a web dashboard for day-to-day operations
- Exposes both REST and GraphQL APIs

## System at a glance

- `backend`: Go API + worker routines + rotating proxy listeners
- `frontend`: Angular dashboard UI
- `postgres`: persistent relational storage
- `redis`: queueing, coordination, and distributed routines
- `website/homepage`: marketing website
- `website/docs`: this Docusaurus documentation site

## Where to start

1. Use [Quick Start](getting-started/quick-start.md) if you want a working instance quickly.
2. Use [Installation](getting-started/installation.md) for setup variants.
3. Use [First Login](getting-started/first-login.md) for initial admin setup.
4. Use [API](api/rest-overview.md) if you are integrating clients.
