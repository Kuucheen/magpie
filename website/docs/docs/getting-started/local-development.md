# Local Development

## Prerequisites

- Go `1.24.x`
- Node.js `20+`
- npm
- Docker (for Postgres + Redis during local dev)

## Start infrastructure

```bash
docker compose up -d postgres redis
```

## Run backend

```bash
cd backend
go run ./cmd/magpie
```

Backend defaults to port `5656`.

## Run frontend

```bash
cd frontend
npm install
npm run start
```

Frontend dev server defaults to `http://localhost:4200`.

## Common dev commands

```bash
cd backend
go test ./...
```

```bash
cd frontend
npm test
```
