# Testing

## Backend

```bash
cd backend
go test ./...
```

## Frontend

```bash
cd frontend
npm test
```

## Docs

```bash
cd website/docs
npm run build
```

## Suggested PR checks

- backend tests pass for touched packages
- frontend tests/build pass for touched components
- docs build succeeds if docs changed
