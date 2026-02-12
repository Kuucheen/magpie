# Documentation Maintenance

Docs live in `website/docs` and are versioned with code.

## Local docs development

```bash
cd website/docs
npm install
npm run start
```

## Build verification

```bash
npm run build
```

## Content guidelines

- Prefer behavior that is observable in code
- Call out constraints and defaults explicitly
- Keep API examples aligned with actual payload shapes
- Update docs in same PR as behavior changes
