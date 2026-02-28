<div align="center">
  <img src="frontend/src/assets/logo/magpie-logo-thumbnail.png" alt="Magpie logo">
  <h3>A Multi-user AIO Proxy Manager</h3>
</div>

<div align="center">
  <img src="https://img.shields.io/github/license/Kuucheen/magpie.svg" alt="license">
  <img src="https://img.shields.io/github/issues/Kuucheen/magpie.svg" alt="issues">
  <a href="https://magpie.tools">
      <img src="https://img.shields.io/badge/Website-magpie.tools-0f766e?style=flat-square&logoColor=white" alt="website">
  </a>
  <a href="https://discord.gg/7FWAGXzhkC">
      <img src="https://img.shields.io/badge/Discord-%235865F2.svg?&logo=discord&logoColor=white" alt="discord">
  </a>
  <br>
  <img src="https://img.shields.io/docker/pulls/kuuchen/magpie-frontend?style=flat-square&logo=docker&label=frontend%20pulls" alt="docker frontend pulls">
  <img src="https://img.shields.io/docker/pulls/kuuchen/magpie-backend?style=flat-square&logo=docker&label=backend%20pulls" alt="docker backend pulls">


[//]: # (  <img src="https://img.shields.io/github/stars/Kuucheen/magpie.svg?style=social" alt="stars">)
</div>

---

Magpie is a self-hosted proxy manager that turns messy proxy lists into something you can actually use: 
- it scrapes proxies from public sources
- continuously checks which ones are alive
- filters out dead/bad entries
- assigns each proxy a reputation score (uptime/latency/anonymity)
- lets you create your own rotating proxy endpoints from the healthy pool

all via a web dashboard.

<details>
    <summary>Screenshots</summary>
    <img src="resources/screenshots/dashboard.png" alt="Dashboard">
    <img src="resources/screenshots/proxyList.png" alt="Proxy List">
    <img src="resources/screenshots/proxyDetail.png" alt="Proxy Details">
    <img src="resources/screenshots/rotatingProxies.png" alt="Rotating Proxies">
    <img src="resources/screenshots/accountSettings.png" alt="Account Settings">
</details>

## Features
- Multi-user
- Auto-scraping
- Proxy Checking / Health checks
- Reputation & filters
- Rotating proxy endpoints
- Dashboard + API
- Application protocols support (HTTP, HTTPS, SOCKS4, SOCKS5)
- Transport protocols support (TCP, QUIC/HTTP3)

## Quick Start

1. **Install Prerequisites:**
    - [Docker Desktop](https://www.docker.com/) (or Docker Engine + Compose)

2. **One-command install (recommended)**

   **macOS/Linux**:
   ```bash
   curl -fsSL https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/install.sh | bash
   ```
   If you see a Docker socket permission error on Linux, the installer will try to use `sudo` for Docker commands (you may be prompted).  
   Alternative fix: `sudo usermod -aG docker "$USER"` (then log out/in, or run `newgrp docker`).  
   Note: `sudo curl ... | bash` still runs `bash` as your user. Try `curl ... | sudo bash` instead.

   **Windows (PowerShell)**:
   ```bash
   iwr -useb https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/install.ps1 | iex
   ```

   This creates a `magpie/` folder with a `docker-compose.yml` and `.env`, then starts the stack.

3. **Required secrets** – Magpie requires:
   - `PROXY_ENCRYPTION_KEY` to encrypt stored proxy secrets (keep it stable between restarts/updates)
   - `JWT_SECRET` to sign authentication tokens
   - optional `STRICT_SECRET_VALIDATION` (defaults to `false` locally, `true` with backend `-production`)

> [!WARNING]
> `PROXY_ENCRYPTION_KEY` locks all stored secrets (proxy auth, passwords, and ip addresses).  
> If you start the backend (or update to a new version) with a *different* key than the one used before, decryption fails and previously added proxies will not display or validate.  
> **Fix:** start the backend again using the **previous key** and everything works like before.  
> **Only rotate on purpose:** if you need a new key, export your proxies first.

4. **If you don't want to use the installer** 

    Requires [Git](https://git-scm.com/downloads)

   ```bash
   git clone https://github.com/Kuucheen/magpie.git
   cd magpie
   cp .env.example .env
   # edit .env and set PROXY_ENCRYPTION_KEY and JWT_SECRET
   # optional: override DB_USERNAME/DB_PASSWORD/DB_NAME
   # optional: tune proxy statistics retention (PROXY_STATISTICS_RETENTION_*)
   docker compose up -d
   ```
5. **Dive in**
    - UI: http://localhost:5050
    - API: http://localhost:5656/api

      The first user who registers becomes admin automatically:

      ```bash
      curl -X POST http://localhost:5656/api/register \
        -H "Content-Type: application/json" \
        -d '{"email":"admin@example.com","password":"ChangeMe123!"}'
      ```

      Local default stays simple: first user can register and becomes admin.

      Production mode (`backend -production`) hardening defaults:
      - `DISABLE_PUBLIC_REGISTRATION=true`
      - `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=false` (public first-admin bootstrap disabled by default)
      - `DB_AUTO_MIGRATE=false`
      - `STRICT_SECRET_VALIDATION=true`

      To bootstrap first admin in production via `/api/register`, explicitly set:
      - `ENABLE_PUBLIC_FIRST_ADMIN_BOOTSTRAP=true` (temporary, controlled window)

      Optional reverse-proxy trust boundary for forwarded client IP headers:
      - `TRUSTED_PROXY_CIDRS=10.0.0.0/8,192.168.0.0/16`

      Health probes:
      - Liveness: `GET /healthz`
      - Readiness: `GET /readyz`

For geo lookups, create a [MaxMind GeoLite2 account](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) and generate a License Key. Enter it in the dashboard (Admin → Other) to enable automatic database downloads and updates.

### Updating
Use the helper scripts to pull the latest code and rebuild just the frontend/backend containers.

- **If you used the one-command installer**:
  - **macOS/Linux**: 
      ```bash
      curl -fsSL https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/update.sh | bash
      ```
      If you see a Docker socket permission error on Linux, the updater will try to use `sudo` for Docker commands (you may be prompted).
  - **Windows (PowerShell)**: 
      ```bash
      iwr -useb https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/update.ps1 | iex
      ```

- **If you cloned the project**:
  - **macOS/Linux**:
    ```bash
    ./scripts/update-frontend-backend.sh
    ```
  - **Windows (Command Prompt)**:
    ```bash
    scripts\update-frontend-backend.bat
    ```
    Double-click the file or run it from the repo root.

## Local Development
- Services: `docker compose up -d postgres redis`
- Backend: `cd backend && go run ./cmd/magpie`
- Frontend: `cd frontend && npm install && npm run start`

## Performance Validation
- Harness: `scripts/perf/`
- Quick gate run: `cd scripts/perf && ./run-gate.sh`
- Include long soak: `cd scripts/perf && PERF_SOAK_DURATION=24h ./run-gate.sh`

Magpie targets Go 1.24.x, Angular 20, PostgreSQL, and Redis. Keep those versions handy for parity.

## Attributions & External Sources
- [AbuseIPDB](https://www.abuseipdb.com/) — logo used with permission when linking to their site.

## Community
- Website: https://magpie.tools
- Discord: https://discord.gg/7FWAGXzhkC
- Issues & feature requests: open them on GitHub.

## License
Magpie ships under the **GNU Affero General Public License v3.0**. See `LICENSE` for the full text. Contributions are more than welcome.
