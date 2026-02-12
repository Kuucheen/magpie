# Quick Start

## Prerequisites

- Docker Desktop, or Docker Engine + Compose
- Internet access to pull container images

## Fast install (recommended)

### macOS/Linux

```bash
curl -fsSL https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/install.sh | bash
```

### Windows (PowerShell)

```powershell
iwr -useb https://raw.githubusercontent.com/Kuucheen/magpie/refs/heads/master/scripts/install.ps1 | iex
```

## Manual install

```bash
git clone https://github.com/Kuucheen/magpie.git
cd magpie
cp .env.example .env
# set PROXY_ENCRYPTION_KEY in .env
docker compose up -d
```

## Access URLs

- UI: `http://localhost:5050`
- API base: `http://localhost:5656/api`

## First account behavior

The first registered user is auto-assigned the `admin` role.
