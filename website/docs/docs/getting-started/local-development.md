# Local Development

## Prerequisites

- Go `1.26.x`
- Node.js `20.19+` or `22.12+`
- npm
- Docker (for Postgres + Redis during local dev)

Current local app stack in this repository:

- backend: Go `1.26`
- frontend: Angular `21.1`, PrimeNG `21`, Tailwind CSS `4`
- docs site: Docusaurus on Node.js `20+`

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
