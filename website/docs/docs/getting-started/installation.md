# Installation

## Docker Compose stack

The default stack includes:

- `backend` on port `5656`
- `frontend` on port `5050`
- `postgres` on host port `5434`
- `redis` on host port `8946`

Rotating proxy listener ports are also mapped by default:

- TCP: `20000-20100`
- UDP: `20000-20100`

## Required secret

Set `PROXY_ENCRYPTION_KEY` before starting Magpie.

Why it matters:

- It encrypts stored proxy secrets (auth, passwords, and encrypted IP fields).
- Changing it later prevents old encrypted values from being decrypted.

## Local clone workflow

```bash
git clone https://github.com/Kuucheen/magpie.git
cd magpie
cp .env.example .env
# edit .env and set PROXY_ENCRYPTION_KEY
docker compose up -d
```

## Installer script notes

Install scripts support environment overrides such as:

- `MAGPIE_INSTALL_DIR`
- `MAGPIE_IMAGE_TAG`
- `MAGPIE_REPO_OWNER`
- `MAGPIE_REPO_NAME`
- `MAGPIE_REPO_REF`

See [Deployment](../operations/deployment.md) and [Environment Variables](../reference/environment-variables.md).
